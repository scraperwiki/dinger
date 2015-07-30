package main

import (
	"fmt"
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

	port := ":8081"

	finish := make(chan struct{})

	header := http.Header{}
	events, errs := listen.RetryingWatch(url, header, finish)

	go func() {
		defer close(finish)

		for err := range errs {
			log.Printf("Error in hookbot event stream: %v", err)
		}
	}()

	go func() {
		for payload := range events {
			log.Printf("Signalled via hookbot, content of payload:")
			log.Printf("%s", payload)
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, World!")
	})

	log.Fatal(http.ListenAndServe(port, nil))
}
