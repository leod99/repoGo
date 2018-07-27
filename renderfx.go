// Package render implements HTML page creation.
package render

import (
	"appengine"
	"encoding/json"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type Article struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Date  string   `json:"date"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
	// hidden field of tags set
	TagsSet map[string]bool `json:"-"`
}

// response json format for article post
type ResponseState struct {
	State   bool   `json:"state"`
	Message string `json:"message"`
}

type ArticleByTag struct {
	Tag         string   `json:"tag"`
	Count       int      `json:"count"`
	Articles    []string `json:"articles"`
	RelatedTags []string `json:"related_tags"`
}

var (
	//
	articleMap = make(map[string]*Article)
	// date maps to a list of articles
	articleDateMap = make(map[string][]*Article)
)

// init sets handler functions for URLs.
func init() {
	rtr := mux.NewRouter()
	rtr.HandleFunc("/articles", articleHandler)
	rtr.HandleFunc("/articles/{id:[0-9a-z]+}", articleHandler)
	rtr.HandleFunc("/tags/{tag:[a-z]+}/{date:[0-9/-]+}", tagHandler)
	http.Handle("/", rtr)
}

// articleHandler handles post and get article request.
func articleHandler(w http.ResponseWriter, req *http.Request) {
	c := appengine.NewContext(req)
	u, _ := url.Parse(req.URL.Path)
	values := strings.Split(u.Path, "/")
	c.Infof("Article Handler URL path: %v", values)
	// get article
	if len(values) > 2 {
		articleId := values[2]
		articleData, ok := articleMap[articleId]
		if !ok {
			c.Infof("No article found for ID: %v", articleId)
		}
		js, err := json.Marshal(articleData)
		if err != nil {
			c.Infof("Error marshal json response: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
		return
	}

	// post article
	defer req.Body.Close()
	var err error
	jsonBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		c.Infof("Error reading json request: %v", err)
	}
	c.Infof("Posting article: %v", string(jsonBody))
	var articleRecord *Article
	var responseState *ResponseState
	err = json.Unmarshal([]byte(string(jsonBody)), &articleRecord)
	if err != nil {
		c.Infof("Error parsing article json: %v", err)
		responseState = &ResponseState{
			State:   false,
			Message: "",
		}
	} else {
		// set response
		c.Infof("Appending article %v for date: %v", articleRecord.ID, articleRecord.Date)
		responseState = &ResponseState{
			State:   true,
			Message: articleRecord.ID,
		}

		articleRecord.TagsSet = make(map[string]bool)
		for _, v := range articleRecord.Tags {
			articleRecord.TagsSet[v] = true
		}
		// set article data in map
		articleMap[articleRecord.ID] = articleRecord
		articleDateMap[articleRecord.Date] = append(articleDateMap[articleRecord.Date], articleRecord)
	}

	js, err := json.Marshal(responseState)
	if err != nil {
		c.Infof("Error marshal json response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// tagHandler handles get tag request
func tagHandler(w http.ResponseWriter, req *http.Request) {
	c := appengine.NewContext(req)
	u, _ := url.Parse(req.URL.Path)
	values := strings.Split(u.Path, "/")
	tag := values[2]
	date := values[3]
	c.Infof("Tag Handler URL path: %v", values)

	var tagResponse *ArticleByTag
	var count int
	var ids, relatedTags []string
	relatedTagsSet := make(map[string]bool)
	c.Infof("Querying articles for %v on %v", tag, date)
	articles, ok := articleDateMap[date]
	if !ok {
		c.Infof("No articles found for date: %v", date)
	}
	c.Infof("%v articles found for date: %v", len(articles), date)
	for idx, v := range articles {
		_, ok := v.TagsSet[tag]
		if ok {
			count++
			// last 10 articles for the tag
			if len(articles)-1-idx < 10 {
				ids = append(ids, v.ID)
			}
			// set related tags
			for k := range v.TagsSet {
				relatedTagsSet[k] = true
			}
		}
	}
	// remove queried tag from set
	delete(relatedTagsSet, tag)
	for k := range relatedTagsSet {
		relatedTags = append(relatedTags, k)
	}
	tagResponse = &ArticleByTag{
		Tag:         tag,
		Count:       count,
		Articles:    ids,
		RelatedTags: relatedTags,
	}
	js, err := json.Marshal(tagResponse)
	if err != nil {
		c.Infof("Error marshal json response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
