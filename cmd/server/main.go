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

// Tag is a filterable tag
type Tag struct {
	Key string
	Time time.Time
	Description string
	Order int
}


// Content is an interface for content data.
type Content interface {
	url() string
}

// Image is a struct containing the fields for graphical content..
type Image struct{
	Location string
	Time time.Time
	Tags []Tag
	Previous *Content
	Next *Content
}

func (*Image) url() string {
	return "http://codeworkshop.dev"
}

// Results are a list of matching content items.
type Results []Content

// ListContentHandler accepts a list of tags and a list of content items to apply them to.
func ListContentHandler(db *bolt.DB) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
	   data := []Tag{{Key: "coding"},{Key: "listcontenthandler"}}
	   w.Header().Set("Content-Type", "application/json")
	   w.WriteHeader(http.StatusCreated)
	   json.NewEncoder(w).Encode(data)
   }
   return fn
   }

// AddContentHandler accepts a list of tags and a list of content items to apply them to.
func AddContentHandler(db *bolt.DB) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
	   data := []Tag{{Key: "coding"},{Key: "addcontenthandler"}}
	   w.Header().Set("Content-Type", "application/json")
	   w.WriteHeader(http.StatusCreated)
	   json.NewEncoder(w).Encode(data)
   }
   return fn
   }

// ListTagsHandler Returns a list of all tags in the system.
func ListTagsHandler(w http.ResponseWriter, r *http.Request) {
	data := []Tag{{Key: "coding"},{Key: "stephen_castle"}}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}

// AddTagsHandler accepts a list of tags and a list of content items to apply them to.
func AddTagsHandler(w http.ResponseWriter, r *http.Request) {
	data := []Tag{{Key: "coding"},{Key: "addtaghandler"}}
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