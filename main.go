package main

import (
	"log"
	"net/http"
	"os"

	"github.com/scraperwiki/hookbot/pkg/listen"
)

func main() {
	url := os.Getenv("HOOKBOT_LISTEN_URL")
	if url == "" {
		log.Fatal("HOOKBOT_LISTEN_URL not set")
	}

	finish := make(chan struct{})
	header := http.Header{}
	events, errs := listen.RetryingWatch(url, header, finish)

	go func() {
		defer close(finish)

		for err := range errs {
			log.Printf("Error in hookbot event stream: %v", err)
		}
	}()

	for payload := range events {
		log.Printf("Signalled via hookbot, content of payload:")
		log.Printf("%s", payload)
	}
}
