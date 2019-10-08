package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	crawlingQueue    = "crawlingQueue"
	todoSubject      = "todoSubject"
	doneSubject      = "doneSubject"
	contentSubject   = "contentSubject"
	defaultUserAgent = "trandoshan-io/crawler"
)

var (
	absoluteUrlRegex = regexp.MustCompile("https?://[a-zA-Z0-9-_./]+.onion?[a-zA-Z0-9-_./]+")
	relativeUrlRegex = regexp.MustCompile("href=\"?[a-zA-Z0-9-_./]+\"")
)

// Data sent to NATS with subject content
type pageData struct {
	Url     string `json:"url"`
	Content string `json:"content"`
}

func main() {
	log.Print("Initializing crawler")

	// build list of forbidden content-type from environment variable
	var forbiddenContentTypes = strings.Split(os.Getenv("FORBIDDEN_CONTENT_TYPES"), ";")
	log.Printf("Loaded %d forbidden content types", len(forbiddenContentTypes))

	// Determinate user agent based on environment variable if any
	var userAgent string
	if userAgentEnvVariable := os.Getenv("USER_AGENT"); userAgentEnvVariable != "" {
		userAgent = userAgentEnvVariable
	} else {
		userAgent = defaultUserAgent
	}

	// create HTTP client with optimized configuration
	// disable SSL check because certificate may not be available inside container
	httpClient := &fasthttp.Client{
		Name:         userAgent,
		Dial:         fasthttpproxy.FasthttpSocksDialer(os.Getenv("TOR_PROXY")),
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 5,
		TLSConfig:    &tls.Config{InsecureSkipVerify: true},
	}

	// connect to NATS server
	nc, err := nats.Connect(os.Getenv("NATS_URI"))
	if err != nil {
		log.Fatalf("Error while connecting to nats server: %s", err)
	}
	defer nc.Close()

	// initialize queue subscriber
	if _, err := nc.QueueSubscribe(todoSubject, crawlingQueue, handleMessages(nc, httpClient, forbiddenContentTypes)); err != nil {
		log.Fatalf("Error while trying to subscribe to server: %s", err)
	}

	log.Print("Consumer initialized successfully")

	//TODO: better way
	select {}
}

func handleMessages(natsClient *nats.Conn, httpClient *fasthttp.Client, forbiddenContentTypes []string) func(*nats.Msg) {
	return func(m *nats.Msg) {
		var url string

		// Unmarshal message
		if err := json.Unmarshal(m.Data, &url); err != nil {
			log.Printf("Error while de-serializing payload: %s", err)
			// todo: store in sort of DLQ?
			return
		}

		// Crawl the page
		data, urls, err := crawlPage(url, httpClient, forbiddenContentTypes)
		if err != nil {
			log.Printf("Error while processing message: %s", err)
			// todo: store in sort of DLQ?
			return
		}

		// Put page body in content queue
		bytes, err := json.Marshal(pageData{Url: url, Content: data})
		if err != nil {
			log.Printf("Error while serializing message into json: %s", err)
			// todo: store in sort of DLQ?
			return
		}
		if err = natsClient.Publish(contentSubject, bytes); err != nil {
			log.Printf("Error while trying to publish to content queue: %s", err)
			// todo: store in sort of DLQ?
			return
		}

		// Put all found URLs into done queue
		for _, url := range urls {
			bytes, err := json.Marshal(url)
			if err != nil {
				log.Printf("Error while serializing message into json: %s", err)
				continue
			}
			if err = natsClient.Publish(doneSubject, bytes); err != nil {
				log.Printf("Error while trying to publish to done queue: %s", err)
			}
		}
	}
}

// Crawl given page with given http client and return page content
//
// Function will return error if http content-type returned by remote server is contained in forbidden content type slice
func crawlPage(url string, httpClient *fasthttp.Client, forbiddenContentTypes []string) (string, []string, error) {
	log.Printf("Crawling page %s", url)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)

	if err := httpClient.Do(req, resp); err != nil {
		return "", nil, err
	}

	// make sure response has no forbidden content type
	contentType := string(resp.Header.ContentType())
	for _, forbiddenContentType := range forbiddenContentTypes {
		if contentType == forbiddenContentType {
			return "", nil, fmt.Errorf("forbidden content-type: %v", contentType)
		}
	}

	switch statusCode := resp.StatusCode(); {
	case statusCode > 302:
		return "", nil, fmt.Errorf("Non managed status code: " + string(statusCode))
		// in case of redirect return found url in header and do not automatically crawl
		// since the url may have been crawled already
	case statusCode == 301 || statusCode == 302:
		log.Printf("Found HTTP redirect (code: %d)", statusCode)
		// extract url that may be present in the page
		urls := extractUrls(strings.TrimSuffix(url, "/"), resp.Body())
		// add url present in the location header (if any)
		if locationUrl := string(resp.Header.Peek("Location")); locationUrl != "" {
			urls = append(urls, locationUrl)
		}
		return string(resp.Body()), urls, nil
	default:
		return string(resp.Body()), extractUrls(strings.TrimSuffix(url, "/"), resp.Body()), nil
	}
}

// Extract URLs from given content and return them
//
// Function will extract both relative and absolute URLs
// For relative url the found url will be prepend by websiteUrl parameter in order to translate them
func extractUrls(websiteUrl string, content []byte) []string {
	// Compile regex to extract all absolute urls in the page body
	absoluteUrls := absoluteUrlRegex.FindAll(content, -1)
	// Compile regex to extract all relative urls in the page body
	relativeUrls := relativeUrlRegex.FindAll(content, -1)

	// Convert each bytes element into their string representation
	var urlStrings []string
	for _, element := range absoluteUrls {
		urlStrings = append(urlStrings, string(element))
	}
	for _, element := range relativeUrls {
		// Little magic here !
		// First of all since the regex is taking the href="..." we need to remote both href=" and the last "
		url := strings.TrimSuffix(strings.ReplaceAll(string(element), "href=\"", ""), "\"")
		// Then remove any leading '/'
		url = strings.TrimPrefix(url, "/")
		// Then preprend website url to the found relative url to have the absolute one
		url = websiteUrl + "/" + url
		urlStrings = append(urlStrings, url)
	}
	return urlStrings
}
