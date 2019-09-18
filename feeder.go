package main

import (
	"encoding/json"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"log"
	"os"
)

func main() {
	log.Println("Initializing feeder")
	if len(os.Args) < 2 {
		log.Fatal("Correct usage: ./feeder <url>")
	}

	// load .env
	if err := godotenv.Load(); err != nil {
		log.Fatal("Unable to load .env file: ", err.Error())
	}
	log.Println("Loaded .env file")

	// connect to NATS server
	nc, err := nats.Connect(os.Getenv("NATS_URI"))
	if err != nil {
		log.Fatal("Error while connecting to nats server: ", err)
	}
	defer nc.Close()

	log.Println("Feeding url " + os.Args[1] + " to web-crawler")

	bytes, err := json.Marshal(os.Args[1])
	if err != nil {
		log.Fatal("Error while serializing message into json: ", err)
	}

	if err := nc.Publish("todoSubject", bytes); err != nil {
		log.Fatal("Error while publishing message: ", err)
	}
}
