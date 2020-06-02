package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)
// Relation is the relationship between a Tag and Content.
type Relation int

const (
		// CREATOR is a person who created this work.
		CREATOR Relation = iota
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
	ContentType ContentType
	Tags []Tag
	Meta MetaParser
}

// Results are a list of matching content items.
type Results []Content

//ListContentHandler Returns a list of all content in the system.
func ListContentHandler(w http.ResponseWriter, r *http.Request) {
	data := Results{Content{Location:"https://codeworkshop.dev", Tags: []Tag{{Key: "coding"}}}}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}

//ListTags Returns a list of all tags in the system.
func ListTags(w http.ResponseWriter, r *http.Request) {
	data := []Tag{{Key: "coding"}}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}

func main() {
    r := mux.NewRouter()
	r.HandleFunc("/content", ListContentHandler)
	r.HandleFunc("/tags", ListTags)

    http.Handle("/", r)
	log.Println("Serving on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	  }
}