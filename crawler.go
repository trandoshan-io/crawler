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
   "strconv"
   "strings"
   "time"
)

const (
   crawlingQueue  = "crawlingQueue"
   todoSubject    = "todoSubject"
   doneSubject    = "doneSubject"
   contentSubject = "contentSubject"
)

var (
   absoluteUrlRegex = regexp.MustCompile("https?://[a-zA-Z0-9-_./]+.onion?[a-zA-Z0-9-_./]+")
   relativeUrlRegex = regexp.MustCompile("href=\"?[a-zA-Z0-9-_./]+\"")
)

type PageData struct {
   Url     string `json:"url"`
   Content string `json:"content"`
}

func main() {
   log.Println("Initializing crawler")

   // build list of forbidden content-type
   //TODO: plug this to database
   var forbiddenContentTypes = []string{
      "application/octet-stream",
   }

   // create HTTP client with optimized configuration
   // disable SSL check because certificate may not be available inside container
   httpClient := &fasthttp.Client{
      Name:         "trandoshan-io/crawler",
      Dial:         fasthttpproxy.FasthttpSocksDialer(os.Getenv("TOR_PROXY")),
      ReadTimeout:  time.Second * 5,
      WriteTimeout: time.Second * 5,
      TLSConfig:    &tls.Config{InsecureSkipVerify: true},
   }

   // connect to NATS server
   nc, err := nats.Connect(os.Getenv("NATS_URI"))
   if err != nil {
      log.Fatal("Error while connecting to nats server: ", err)
   }
   defer nc.Close()

   // initialize queue subscriber
   if _, err := nc.QueueSubscribe(todoSubject, crawlingQueue, handleMessages(nc, httpClient, forbiddenContentTypes)); err != nil {
      log.Fatal("Error while trying to subscribe to server: ", err)
   }

   log.Println("Consumer initialized successfully")

   //TODO: better way
   select {}
}

func handleMessages(nc *nats.Conn, hc *fasthttp.Client, forbiddenContentTypes []string) func(*nats.Msg) {
   return func(m *nats.Msg) {
      var url string

      // Unmarshal message
      if err := json.Unmarshal(m.Data, &url); err != nil {
         log.Println("Error while de-serializing payload: ", err)
         // todo: store in sort of DLQ?
         return
      }

      // Crawl the page
      data, urls, err := crawlPage(url, hc, forbiddenContentTypes)
      if err != nil {
         log.Println("Error while processing message: ", err)
         // todo: store in sort of DLQ?
         return
      }

      // Put page body in content queue
      bytes, err := json.Marshal(PageData{Url: url, Content: data,})
      if err != nil {
         log.Println("Error while serializing message into json: ", err)
         // todo: store in sort of DLQ?
         return
      }
      if err = nc.Publish(contentSubject, bytes); err != nil {
         log.Println("Error while trying to publish to content queue: ", err)
         // todo: store in sort of DLQ?
         return
      }

      // Put all found URLs into done queue
      for _, url := range urls {
         bytes, err := json.Marshal(url)
         if err != nil {
            log.Println("Error while serializing message into json: ", err)
            continue
         }
         if err = nc.Publish(doneSubject, bytes); err != nil {
            log.Println("Error while trying to publish to done queue: ", err.Error())
         }
      }
   }
}

func crawlPage(url string, hc *fasthttp.Client, forbiddenContentTypes []string) (string, []string, error) {
   log.Println("Crawling page: ", url)

   req := fasthttp.AcquireRequest()
   resp := fasthttp.AcquireResponse()
   defer fasthttp.ReleaseRequest(req)
   defer fasthttp.ReleaseResponse(resp)

   req.SetRequestURI(url)

   if err := hc.Do(req, resp); err != nil {
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
      return "", nil, fmt.Errorf("Invalid status code: " + string(statusCode))
      // in case of redirect return found url in header and do not automatically crawl
      // since the url may have been crawled already
   case statusCode == 301 || statusCode == 302:
      log.Println("Found redirect (HTTP " + strconv.Itoa(statusCode) + ")")
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
