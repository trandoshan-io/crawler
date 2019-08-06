package main

import (
   "crypto/tls"
   "encoding/json"
   "fmt"
   "github.com/joho/godotenv"
   "github.com/streadway/amqp"
   tamqp "github.com/trandoshan-io/amqp"
   "github.com/valyala/fasthttp"
   "log"
   "os"
   "regexp"
   "strconv"
)

const (
   todoQueue    = "todo"
   doneQueue    = "done"
   contentQueue = "content"
)

var (
   urlRegex = regexp.MustCompile("https?://[a-zA-Z0-9-_./]+")
)

type WebsiteData struct {
   Url  string `json:"url"`
   Data string `json:"data"`
}

func main() {
   log.Println("Initializing crawler")

   // load .env
   if err := godotenv.Load(); err != nil {
      log.Fatal("Unable to load .env file: ", err.Error())
   }
   log.Println("Loaded .env file")

   prefetch, err := strconv.Atoi(os.Getenv("AMQP_PREFETCH"))
   if err != nil {
      log.Fatal(err)
   }

   // initialize publishers
   publisher, err := tamqp.NewStateFullPublisher(os.Getenv("AMQP_URI"))
   if err != nil {
      log.Fatal("Unable  to create publisher: ", err.Error())
   }
   log.Println("Publisher initialized successfully")

   // initialize consumer & start him
   consumer, err := tamqp.NewConsumer(os.Getenv("AMQP_URI"), prefetch)
   if err != nil {
      log.Fatal("Unable to create consumer: ", err.Error())
   }
   if err := consumer.Consume(todoQueue, false, handleMessages(publisher)); err != nil {
      log.Fatal("Unable to consume message: ", err.Error())
   }
   log.Println("Consumer initialized successfully")

   //TODO: better way
   select {}

   _ = consumer.Shutdown()
}

func handleMessages(publisher tamqp.Publisher) func(deliveries <-chan amqp.Delivery, done chan error) {
   return func(deliveries <-chan amqp.Delivery, done chan error) {
      for delivery := range deliveries {
         var url string

         // Unmarshal message
         if err := json.Unmarshal(delivery.Body, &url); err != nil {
            log.Println("Error while de-serializing payload: ", err.Error())
            _ = delivery.Reject(false)
            continue
         }

         data, urls, err := crawlPage(url)
         if err != nil {
            log.Println("Error while processing message: ", err.Error())
            _ = delivery.Reject(false)
            continue
         }
         // Put website body in content queue
         if err := publisher.PublishJson("", contentQueue, WebsiteData{Url: url, Data: data,}); err != nil {
            log.Println("Error while trying to publish to content queue: ", err.Error())
            _ = delivery.Reject(false)
            continue
         }
         // Put all found URLs into done queue
         for _, url := range urls {
            if err := publisher.PublishJson("", doneQueue, url); err != nil {
               log.Println("Error while trying to publish to done queue: ", err.Error())
            }
         }

         _ = delivery.Ack(false)
      }
   }
}

func crawlPage(url string) (string, []string, error) {
   log.Println("Crawling page: ", url)
   req := fasthttp.AcquireRequest()
   req.SetRequestURI(url)

   resp := fasthttp.AcquireResponse()
   // disable SSL check because certificate may not be available inside container
   //TODO: create at startup ?
   client := &fasthttp.Client{Name: "Trandoshan/Crawler", TLSConfig: &tls.Config{InsecureSkipVerify: true}}

   if err := client.Do(req, resp); err != nil {
      return "", nil, err
   }

   // todo do not load if content type is octet stream or something

   switch statusCode := resp.StatusCode(); {
   case statusCode > 301:
      return "", nil, fmt.Errorf("Invalid status code: " + string(statusCode))
   case statusCode == 301:
      return crawlPage(string(resp.Header.Peek("Location")))
   default:
      return string(resp.Body()), extractUrls(resp.Body()), nil
   }
}

func extractUrls(content []byte) []string {
   // Compile regex to extract all urls in the page body
   urls := urlRegex.FindAll(content, -1)

   // Convert each bytes element into their string representation
   var urlStrings []string
   for _, element := range urls {
      urlStrings = append(urlStrings, string(element))
   }
   return urlStrings
}
