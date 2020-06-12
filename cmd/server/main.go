package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
)
// Relation is the relationship between a Tag and Content.
type Relation int

const (
		// DEFAULT represents no known special relation.
		DEFAULT Relation = iota
		// CREATOR is the name of a series of content..
		CREATOR
		// SERIES is the name of a series of content..
		SERIES
		// REFERENCE is a reference to another work.
		REFERENCE
)

// ContentType is the ContentType of a Content struct.
type ContentType int

const (
		// IMAGE is any photo content, remote or local.
		IMAGE ContentType = iota
		// DOCUMENT is any rich text content, remote or local.
		DOCUMENT
		// VIDEO is any video content, remote or local.
		VIDEO
		// WEB is any html link content, remote or local.
		WEB
)

// Tag is a filterable tag
type Tag struct {
	Key string
	Time time.Time
	Relation Relation
	Desctiption string
	Order int
}

// MetaParser is an interface for getting type dependent metadata.
type MetaParser interface {
	GetMeta()
}

// Content is a specific piece of content.
type Content struct{
	Location string
	Time time.Time
	ContentType ContentType
	Tags []Tag
	Meta MetaParser
	Previous *Content
	Next *Content
}

// Results are a list of matching content items.
type Results []Content

// ListContentHandler accepts a list of tags and a list of content items to apply them to.
func ListContentHandler(db *bolt.DB) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
	   data := []Tag{{Key: "coding"},{Key: "listcontenthandler", Relation: CREATOR}}
	   w.Header().Set("Content-Type", "application/json")
	   w.WriteHeader(http.StatusCreated)
	   json.NewEncoder(w).Encode(data)
   }
   return fn
   }

// AddContentHandler accepts a list of tags and a list of content items to apply them to.
func AddContentHandler(db *bolt.DB) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
	   data := []Tag{{Key: "coding"},{Key: "addcontenthandler", Relation: CREATOR}}
	   w.Header().Set("Content-Type", "application/json")
	   w.WriteHeader(http.StatusCreated)
	   json.NewEncoder(w).Encode(data)
   }
   return fn
   }

// ListTagsHandler Returns a list of all tags in the system.
func ListTagsHandler(w http.ResponseWriter, r *http.Request) {
	data := []Tag{{Key: "coding"},{Key: "stephen_castle", Relation: CREATOR}}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}

// AddTagsHandler accepts a list of tags and a list of content items to apply them to.
func AddTagsHandler(w http.ResponseWriter, r *http.Request) {
	data := []Tag{{Key: "coding"},{Key: "addtaghandler", Relation: CREATOR}}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}


func main() {
	db, err := setupDB()
	if err != nil {
		log.Println("Error connecting to database.")
	}
    r := mux.NewRouter()
	r.HandleFunc("/content", ListContentHandler(db)).Methods("GET")
	r.HandleFunc("/content", AddContentHandler(db)).Methods("POST")
	r.HandleFunc("/content/{id}/tags", AddTagsHandler).Methods("POST")
	r.HandleFunc("/tags", ListTagsHandler).Methods("GET")
    http.Handle("/", r)
	log.Println("Serving on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	  }
}

func setupDB() (*bolt.DB, error) {
    db, err := bolt.Open("anansi.db", 0600, nil)
    if err != nil {
        return nil, fmt.Errorf("could not open db, %v", err)
	}
    err = db.Update(func(tx *bolt.Tx) error {
        root, err := tx.CreateBucketIfNotExists([]byte("DB"))
        if err != nil {
        return fmt.Errorf("could not create root bucket: %v", err)
        }
        _, err = root.CreateBucketIfNotExists([]byte("TAGS"))
        if err != nil {
        return fmt.Errorf("could not create tag bucket: %v", err)
        }
        _, err = root.CreateBucketIfNotExists([]byte("CONTENT"))
        if err != nil {
        return fmt.Errorf("could not create content bucket: %v", err)
        }
        return nil
    })
    if err != nil {
        return nil, fmt.Errorf("could not set up buckets, %v", err)
    }
    fmt.Println("DB Setup Done")
    return db, nil
}

func addContent(db *bolt.DB, weight string, date time.Time) error {
    err := db.Update(func(tx *bolt.Tx) error {
        err := tx.Bucket([]byte("DB")).Bucket([]byte("Content")).Put([]byte(date.Format(time.RFC3339)), []byte(weight))
        if err != nil {
            return fmt.Errorf("could not insert content: %v", err)
        }
        return nil
    })
    fmt.Println("Added Content")
    return err
}