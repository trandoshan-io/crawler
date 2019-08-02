package main

import (
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
	"time"
)

const (
	todoQueue    = "todo"
	doneQueue    = "done"
	contentQueue = "content"
)

var (
	urlRegex = regexp.MustCompile("https?://[a-zA-Z0-9-_./]+")
)

func main() {
	log.Println("Initializing crawler")

	// load .env
	if err := godotenv.Load(); err != nil {
		log.Fatal("Unable to load .env file: ", err.Error())
	}
	log.Println("Loaded .env file")

	// allow some kind of boot delay to fix usage in docker-compose
	// TODO: find a better way
	if startupDelay := os.Getenv("STARTUP_DELAY"); startupDelay != "" {
		val, _ := strconv.Atoi(startupDelay)
		time.Sleep(time.Duration(val) * time.Second)
	}

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
	log.Println("Consumer initialized successfully")

	if err := consumer.Consume(todoQueue, true, handleMessages(publisher)); err != nil {
		log.Fatal("Unable to consume message: ", err.Error())
	}

	//TODO: better way
	select {}

	_ = consumer.Shutdown()
}

func handleMessages(publisher tamqp.Publisher) func(deliveries <-chan amqp.Delivery, done chan error) {
	return func(deliveries <-chan amqp.Delivery, done chan error) {
		for delivery := range deliveries {
			var url string
			if err := json.Unmarshal(delivery.Body, url); err == nil {
				data, urls, err := crawlPage(url)
				if err != nil {
					log.Println("Error while processing message: ", err.Error())
				}

				if err := publisher.PublishJson("", doneQueue, urls); err != nil {
					log.Println("Error while trying to publish to done queue: ", err.Error())
				}
				if err := publisher.PublishJson("", contentQueue, data); err != nil {
					log.Println("Error while trying to publish to content queue: ", err.Error())
				}
			}
		}
	}
}

func crawlPage(url string) ([]byte, []string, error) {
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)

	resp := fasthttp.AcquireResponse()
	client := &fasthttp.Client{}

	if err := client.Do(req, resp); err != nil {
		return nil, nil, err
	}

	switch statusCode := resp.StatusCode(); {
	case statusCode > 301:
		return nil, nil, fmt.Errorf("Invalid status code: " + string(statusCode))
	case statusCode == 301:
		return crawlPage(string(resp.Header.Peek("Location")))
	default:
		return resp.Body(), extractUrls(resp.Body()), nil
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
