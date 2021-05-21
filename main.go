package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gomarkdown/markdown"
	"github.com/gorilla/mux"
	"github.com/gosimple/slug"
	"github.com/microcosm-cc/bluemonday"
)

const topLevelBucket = "ANANSI"
const contentBucket = "CONTENT"
const tagBucket = "TAGS"

// SiteMetaData is general information about the Site
type SiteMetaData struct {
	Title       string
	Description string
}

// Content is the data required to represent a Blog Content Object.
type Content struct {
	Author    string    `json:"author,omitempty"`
	Body      string    `json:"body,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	Title     string    `json:"title,omitempty"`
	Slug      string    `json:"slug,omitempty"`
}

// ContentMap is a map of contents with the slug as the key.
type ContentMap map[string]Content

// Content is the data required to represent a Blog Content Object.
type Tag struct {
	Author    string    `json:"author,omitempty"`
	Body      string    `json:"body,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	Title     string    `json:"title,omitempty"`
	Slug      string    `json:"slug,omitempty"`
}

type TagMap map[string]Tag

// Test Data

var siteMetaData = SiteMetaData{
	Title:       "Anansi",
	Description: "A content tagging service for discovery and organization.",
}

// HomePageData is the data required to render the HTML template for the home page.
// It is made up of the site meta data, and a map of all of the contents.
type HomePageData struct {
	SiteMetaData SiteMetaData
	Content      ContentMap
}

type ContentListData struct {
	SiteMetaData SiteMetaData
	Content      ContentMap
}

type TagListData struct {
	SiteMetaData SiteMetaData
	Tags         TagMap
}

// ContentPageData is the data required to render the HTML template for the content page.
// It is made up of the site meta data, and a Content struct.
type ContentPageData struct {
	SiteMetaData SiteMetaData
	Content      Content
	HTML         template.HTML
}

type TagPageData struct {
	SiteMetaData SiteMetaData
	Tag          Tag
	HTML         template.HTML
}

func main() {

	db, err := setupDB()
	defer db.Close()

	if err != nil {
		log.Println(err)
	}

	r := newRouter(db)
	// Create http server and run inside go routine for graceful shutdown.
	srv := &http.Server{
		Handler:      r,
		Addr:         "0.0.0.0:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Println("Starting up..")

	// This code is all about gracefully shutting down the web server.
	// This allows the server to resolve any pending requests before shutting down.
	// This works by running the web server in a go routine.
	// The main function then continues and blocks waiting for a kill signal from the os.
	// It intercepts the kill signal, shuts down the server by calling the Shutdown method.
	// Then exits when that is done.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	srv.Shutdown(ctx)
	log.Println("Shutting down..")
	os.Exit(0)
}

// homeHandler returns the list of blog contents rendered in an HTML template.
func homeHandler(db *bolt.DB, t *template.Template) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		log.Println("Requested the home page.")
		res.Header().Set("Content-Type", "text/html; charset=UTF-8")
		res.WriteHeader(http.StatusOK)
		t.Execute(res, HomePageData{SiteMetaData: siteMetaData})
	}

	return fn
}

// homeHandler returns the list of blog contents rendered in an HTML template.
func contentListHandler(db *bolt.DB, t *template.Template) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		contentData, err := listContent(db)
		if err != nil {
			res.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte("Could not list contents."))
			return
		}
		log.Println("Requested the content list page.")
		res.Header().Set("Content-Type", "text/html; charset=UTF-8")
		res.WriteHeader(http.StatusOK)
		t.Execute(res, HomePageData{SiteMetaData: siteMetaData, Content: contentData})
	}

	return fn
}

// createContentPageHandler serves the UI for creating a content. It is a form that submits to the create content REST endpoint.
func createContentPageHandler(db *bolt.DB, t *template.Template) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		log.Println("Requested the create content page.")
		res.Header().Set("Content-Type", "text/html; charset=UTF-8")
		res.WriteHeader(http.StatusOK)
		t.Execute(res, HomePageData{SiteMetaData: siteMetaData})
	}

	return fn
}

// contentHandler looks up a specific blog content and returns it as an HTML template.
func getContentHandler(db *bolt.DB, t *template.Template) http.HandlerFunc {

	fn := func(res http.ResponseWriter, r *http.Request) {
		// Get the URL param named slug from the response.
		slug := mux.Vars(r)["slug"]
		content, err := getContent(db, slug)
		if err != nil {
			res.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			res.WriteHeader(http.StatusNotFound)
			res.Write([]byte("404 Page Not Found"))
			return
		}
		log.Printf("Requested: %s by %s \n", content.Title, content.Author)
		unsafeContentHTML := markdown.ToHTML([]byte(content.Body), nil, nil)
		contentHTML := bluemonday.UGCPolicy().SanitizeBytes(unsafeContentHTML)
		res.Header().Set("Content-Type", "text/html; charset=UTF-8")
		res.WriteHeader(http.StatusOK)
		t.Execute(res, ContentPageData{SiteMetaData: siteMetaData, Content: *content, HTML: template.HTML(contentHTML)})
	}
	return fn
}

// editContentPageHandler serves the UI for creating a content. It is a form that submits to the create content REST endpoint.
func editContentPageHandler(db *bolt.DB, t *template.Template) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		// Get the URL param named slug from the response.
		slug := mux.Vars(r)["slug"]
		content, err := getContent(db, slug)
		if err != nil {
			res.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			res.WriteHeader(http.StatusNotFound)
			res.Write([]byte("404 Page Not Found"))
			return
		}
		log.Printf("Requested edit page for: %s by %s \n", content.Title, content.Author)
		res.Header().Set("Content-Type", "text/html; charset=UTF-8")
		res.WriteHeader(http.StatusOK)
		t.Execute(res, ContentPageData{SiteMetaData: siteMetaData, Content: *content})
	}

	return fn
}

// createContentHandler handles contented JSON data representing a new content, and stores it in the database.
// It creates a slug to use as a key using the title of the content.
// This implies in the current state of affairs that titles must be unique or the keys will overwrite each other.
func createContentHandler(db *bolt.DB) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		var content Content
		res.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		// Reads in the body content from the content request safely limiting to max size.
		body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		if err != nil {
			panic(err)
		}
		// Close the Reader.
		if err := r.Body.Close(); err != nil {
			panic(err)
		}

		// Convert the JSON to a Content struct and write it to the content variable created at the top
		// of the handler.
		if err := json.Unmarshal(body, &content); err != nil {
			res.Header().Set("Content-Type", "application/json; charset=UTF-8")
			res.WriteHeader(422) // unprocessable entity
			if err := json.NewEncoder(res).Encode(err); err != nil {
				panic(err)
			}
		}

		// Set the creation time stamp to the current server time.
		content.CreatedAt = time.Now()

		// Create a URL safe slug from the timestamp and the title.
		autoSlug := fmt.Sprintf("%s-%s", slug.Make(content.CreatedAt.Format(time.RFC3339)), slug.Make(content.Title))
		content.Slug = autoSlug

		if err = upsertContent(db, content, autoSlug); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte("Error writing to DB."))
			return
		}

		res.Header().Set("Content-Type", "application/json; charset=UTF-8")
		res.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(res).Encode(content); err != nil {
			panic(err)
		}
	}
	return fn
}

// modifyContentHandler is responsible for modifing the contents of a specific content.
// It accepts a new content object as JSON content in the request body.
// It writes the new content object to the URL slug value unlike the createContentHandler
// which generates a new slug using the content date and time. Notice this means you can not change the URI.
// This is left as homework for the reader.
func modifyContentHandler(db *bolt.DB) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		var content Content
		slug := mux.Vars(r)["slug"]
		res.Header().Set("Content-Type", "application/json; charset=UTF-8")
		body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		if err != nil {
			panic(err)
		}
		if err := r.Body.Close(); err != nil {
			panic(err)
		}
		if err := json.Unmarshal(body, &content); err != nil {
			res.WriteHeader(422) // unprocessable entity
			if err := json.NewEncoder(res).Encode(err); err != nil {
				panic(err)
			}
		}
		content.Slug = slug
		content.CreatedAt = time.Now()
		// Call the upsertContent function passing in the database, a content struct, and the slug.
		// If there is an error writing to the database write an error to the response and return.
		if err = upsertContent(db, content, slug); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte("Error writing to DB."))
			return
		}
		res.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(res).Encode(content); err != nil {
			panic(err)
		}
	}
	return fn
}

// DeleteContentHandler deletes the content with the key matching the slug in the URL.
func deleteContentHandler(db *bolt.DB) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		res.Header().Set("Content-Type", "application/json; charset=UTF-8")
		slug := mux.Vars(r)["slug"]
		if err := deleteContent(db, slug); err != nil {
			panic(err)
		}
		res.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(res).Encode(struct {
			Deleted bool
		}{
			true,
		}); err != nil {
			panic(err)
		}
	}
	return fn
}

// DATA STORE FUNCTIONS

// upsertContent writes a content to the boltDB KV store using the slug as a key, and a serialized content struct as the value.
// If the slug already exists the existing content will be overwritten.
func upsertContent(db *bolt.DB, content Content, slug string) error {
	// Marshl content struct into bytes which can be written to Bolt.
	buf, err := json.Marshal(content)
	if err != nil {
		return err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket([]byte(topLevelBucket)).Bucket([]byte(contentBucket)).Put([]byte(slug), []byte(buf))
		if err != nil {
			return fmt.Errorf("could not insert content: %v", err)
		}
		return nil
	})
	return err
}

// listContent returns a map of contents indexed by the slug.
// TODO: We could we add pagination to this!
func listContent(db *bolt.DB) (ContentMap, error) {
	results := ContentMap{}
	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(topLevelBucket)).Bucket([]byte(contentBucket))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			content := Content{}
			if err := json.Unmarshal(v, &content); err != nil {
				panic(err)
			}
			results[string(k)] = content
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// getContent gets a specific content from the database by the slug.
func getContent(db *bolt.DB, slug string) (*Content, error) {
	result := Content{}
	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(topLevelBucket)).Bucket([]byte(contentBucket))
		v := b.Get([]byte(slug))
		if err := json.Unmarshal(v, &result); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// deleteContent deletes a specific content by slug.
func deleteContent(db *bolt.DB, slug string) error {
	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket([]byte(topLevelBucket)).Bucket([]byte(contentBucket)).Delete([]byte(slug))
		if err != nil {
			return fmt.Errorf("could not delete content: %v", err)
		}
		return nil
	})
	return err
}

// TAG HANDLERS
// tagListHandler returns the list of blog tags rendered in an HTML template.
func tagListHandler(db *bolt.DB, t *template.Template) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		tagData, err := listTag(db)
		if err != nil {
			res.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte("Could not list tags."))
			return
		}
		log.Println("Requested the tag list page.")
		res.Header().Set("Content-Type", "text/html; charset=UTF-8")
		res.WriteHeader(http.StatusOK)
		t.Execute(res, TagListData{SiteMetaData: siteMetaData, Tags: tagData})
	}

	return fn
}

// createTagPageHandler serves the UI for creating a tag. It is a form that submits to the create tag REST endpoint.
func createTagPageHandler(db *bolt.DB, t *template.Template) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		log.Println("Requested the create tag page.")
		res.Header().Set("Content-Type", "text/html; charset=UTF-8")
		res.WriteHeader(http.StatusOK)
		t.Execute(res, HomePageData{SiteMetaData: siteMetaData})
	}

	return fn
}

// tagHandler looks up a specific blog tag and returns it as an HTML template.
func getTagHandler(db *bolt.DB, t *template.Template) http.HandlerFunc {

	fn := func(res http.ResponseWriter, r *http.Request) {
		// Get the URL param named slug from the response.
		slug := mux.Vars(r)["slug"]
		tag, err := getTag(db, slug)
		if err != nil {
			res.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			res.WriteHeader(http.StatusNotFound)
			res.Write([]byte("404 Page Not Found"))
			return
		}
		log.Printf("Requested: %s by %s \n", tag.Title, tag.Author)
		unsafeContentHTML := markdown.ToHTML([]byte(tag.Body), nil, nil)
		tagHTML := bluemonday.UGCPolicy().SanitizeBytes(unsafeContentHTML)
		res.Header().Set("Content-Type", "text/html; charset=UTF-8")
		res.WriteHeader(http.StatusOK)
		t.Execute(res, TagPageData{SiteMetaData: siteMetaData, Tag: *tag, HTML: template.HTML(tagHTML)})
	}
	return fn
}

// ediTagPageHandler serves the UI for creating a tag. It is a form that submits to the create tag REST endpoint.
func editTagPageHandler(db *bolt.DB, t *template.Template) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		// Get the URL param named slug from the response.
		slug := mux.Vars(r)["slug"]
		tag, err := getTag(db, slug)
		if err != nil {
			res.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			res.WriteHeader(http.StatusNotFound)
			res.Write([]byte("404 Page Not Found"))
			return
		}
		log.Printf("Requested edit page for: %s by %s \n", tag.Title, tag.Author)
		res.Header().Set("Content-Type", "text/html; charset=UTF-8")
		res.WriteHeader(http.StatusOK)
		t.Execute(res, TagPageData{SiteMetaData: siteMetaData, Tag: *tag})
	}

	return fn
}

// createTagHandler handles taged JSON data representing a new tag, and stores it in the database.
// It creates a slug to use as a key using the title of the tag.
// This implies in the current state of affairs that titles must be unique or the keys will overwrite each other.
func createTagHandler(db *bolt.DB) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		var tag Tag
		res.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		// Reads in the body tag from the tag request safely limiting to max size.
		body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		if err != nil {
			panic(err)
		}
		// Close the Reader.
		if err := r.Body.Close(); err != nil {
			panic(err)
		}

		// Convert the JSON to a Tag struct and write it to the tag variable created at the top
		// of the handler.
		if err := json.Unmarshal(body, &tag); err != nil {
			res.Header().Set("Content-Type", "application/json; charset=UTF-8")
			res.WriteHeader(422) // unprocessable entity
			if err := json.NewEncoder(res).Encode(err); err != nil {
				panic(err)
			}
		}

		// Set the creation time stamp to the current server time.
		tag.CreatedAt = time.Now()

		// Create a URL safe slug from the timestamp and the title.
		autoSlug := fmt.Sprintf("%s-%s", slug.Make(tag.CreatedAt.Format(time.RFC3339)), slug.Make(tag.Title))
		tag.Slug = autoSlug

		if err = upsertTag(db, tag, autoSlug); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte("Error writing to DB."))
			return
		}

		res.Header().Set("Content-Type", "application/json; charset=UTF-8")
		res.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(res).Encode(tag); err != nil {
			panic(err)
		}
	}
	return fn
}

// modifyTagHandler is responsible for modifing the tags of a specific tag.
// It accepts a new tag object as JSON tag in the request body.
// It writes the new tag object to the URL slug value unlike the createTagHandler
// which generates a new slug using the tag date and time. Notice this means you can not change the URI.
// This is left as homework for the reader.
func modifyTagHandler(db *bolt.DB) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		var tag Tag
		slug := mux.Vars(r)["slug"]
		res.Header().Set("Content-Type", "application/json; charset=UTF-8")
		body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		if err != nil {
			panic(err)
		}
		if err := r.Body.Close(); err != nil {
			panic(err)
		}
		if err := json.Unmarshal(body, &tag); err != nil {
			res.WriteHeader(422) // unprocessable entity
			if err := json.NewEncoder(res).Encode(err); err != nil {
				panic(err)
			}
		}
		tag.Slug = slug
		tag.CreatedAt = time.Now()
		// Call the upserTag function passing in the database, a tag struct, and the slug.
		// If there is an error writing to the database write an error to the response and return.
		if err = upsertTag(db, tag, slug); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte("Error writing to DB."))
			return
		}
		res.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(res).Encode(tag); err != nil {
			panic(err)
		}
	}
	return fn
}

// DeleteTagHandler deletes the tag with the key matching the slug in the URL.
func deleteTagHandler(db *bolt.DB) http.HandlerFunc {
	fn := func(res http.ResponseWriter, r *http.Request) {
		res.Header().Set("Content-Type", "application/json; charset=UTF-8")
		slug := mux.Vars(r)["slug"]
		if err := deleteTag(db, slug); err != nil {
			panic(err)
		}
		res.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(res).Encode(struct {
			Deleted bool
		}{
			true,
		}); err != nil {
			panic(err)
		}
	}
	return fn
}

// DATA STORE FUNCTIONS

// upserTag writes a tag to the boltDB KV store using the slug as a key, and a serialized tag struct as the value.
// If the slug already exists the existing tag will be overwritten.
func upsertTag(db *bolt.DB, tag Tag, slug string) error {

	// Marshal tag struct into bytes which can be written to Bolt.
	buf, err := json.Marshal(tag)
	if err != nil {
		return err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket([]byte(topLevelBucket)).Bucket([]byte(tagBucket)).Put([]byte(slug), []byte(buf))
		if err != nil {
			return fmt.Errorf("could not insert tag: %v", err)
		}
		return nil
	})
	return err
}

// lisTag returns a map of tags indexed by the slug.
// TODO: We could we add pagination to this!
func listTag(db *bolt.DB) (TagMap, error) {
	results := TagMap{}
	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(topLevelBucket)).Bucket([]byte(tagBucket))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			tag := Tag{}
			if err := json.Unmarshal(v, &tag); err != nil {
				panic(err)
			}
			results[string(k)] = tag
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// geTag gets a specific tag from the database by the slug.
func getTag(db *bolt.DB, slug string) (*Tag, error) {
	result := Tag{}
	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(topLevelBucket)).Bucket([]byte(tagBucket))
		v := b.Get([]byte(slug))
		if err := json.Unmarshal(v, &result); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// deleteContent deletes a specific tag by slug.
func deleteTag(db *bolt.DB, slug string) error {
	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket([]byte(topLevelBucket)).Bucket([]byte(tagBucket)).Delete([]byte(slug))
		if err != nil {
			return fmt.Errorf("could not delete tag: %v", err)
		}
		return nil
	})
	return err
}

// INITIALIZATION FUNCTIONS
// setupDB sets up the database when the program start.
//  First it connects to the database, then it creates the buckets required to run the app if they do not exist.
func setupDB() (*bolt.DB, error) {
	db, err := bolt.Open("anansi.db", 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists([]byte(topLevelBucket))
		if err != nil {
			return fmt.Errorf("could not create root bucket: %v", err)
		}
		_, err = root.CreateBucketIfNotExists([]byte(contentBucket))
		if err != nil {
			return fmt.Errorf("could not create content bucket: %v", err)
		}
		_, err = root.CreateBucketIfNotExists([]byte(tagBucket))
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

// newRouter configures and sets up the gorilla mux router paths and connects the route to the handler function.
func newRouter(db *bolt.DB) *mux.Router {

	// Load and parse the html templates to be used.
	homePageTemplate := template.Must(template.ParseFiles("templates/home.html"))
	contentListTemplate := template.Must(template.ParseFiles("templates/content/list.html"))
	contentDetailTemplate := template.Must(template.ParseFiles("templates/content/detail.html"))
	contentEditTemplate := template.Must(template.ParseFiles("templates/content/edit.html"))
	contentCreateTemplate := template.Must(template.ParseFiles("templates/content/create.html"))

	tagListTemplate := template.Must(template.ParseFiles("templates/tags/list.html"))
	tagDetailTemplate := template.Must(template.ParseFiles("templates/tags/detail.html"))
	tagEditTemplate := template.Must(template.ParseFiles("templates/tags/edit.html"))
	tagCreateTemplate := template.Must(template.ParseFiles("templates/tags/create.html"))
	r := mux.NewRouter()
	r.StrictSlash(true)
	r.HandleFunc("/", homeHandler(db, homePageTemplate)).Methods("GET")
	r.HandleFunc("/content", contentListHandler(db, contentListTemplate)).Methods("GET")
	r.HandleFunc("/content", createContentHandler(db)).Methods("POST")
	r.HandleFunc("/content/create", createContentPageHandler(db, contentCreateTemplate)).Methods("GET")
	r.HandleFunc("/content/{slug}", getContentHandler(db, contentDetailTemplate)).Methods("GET")
	r.HandleFunc("/content/{slug}", modifyContentHandler(db)).Methods("POST")
	r.HandleFunc("/content/{slug}", deleteContentHandler(db)).Methods("DELETE")
	r.HandleFunc("/content/{slug}/edit", editContentPageHandler(db, contentEditTemplate)).Methods("GET")

	r.HandleFunc("/tags", tagListHandler(db, tagListTemplate)).Methods("GET")
	r.HandleFunc("/tags", createTagHandler(db)).Methods("POST")
	r.HandleFunc("/tags/create", createTagPageHandler(db, tagCreateTemplate)).Methods("GET")
	r.HandleFunc("/tags/{slug}", getTagHandler(db, tagDetailTemplate)).Methods("GET")
	r.HandleFunc("/tags/{slug}", modifyTagHandler(db)).Methods("POST")
	r.HandleFunc("/tags/{slug}", deleteTagHandler(db)).Methods("DELETE")
	r.HandleFunc("/tags/{slug}/edit", editTagPageHandler(db, tagEditTemplate)).Methods("GET")
	return r
}
