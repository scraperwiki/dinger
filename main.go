package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

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

	var (
		mu         sync.Mutex
		eventTimes []time.Time
	)

	go func() {
		for range events {
			log.Printf("Recieved event.")

			func() {
				mu.Lock()
				defer mu.Unlock()

				eventTimes = append([]time.Time{time.Now()}, eventTimes...)

				const MAX_EVENTS = 3
				if len(eventTimes) > MAX_EVENTS {
					eventTimes = eventTimes[:MAX_EVENTS]
				}
			}()
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		for _, t := range eventTimes {
			const RFC3339_UTC = "2006-01-02T15:04:05Z"
			fmt.Fprintf(w, "<entry><updated>%v</updated></entry>\n", t.UTC().Format(RFC3339_UTC))
		}
	})

	log.Fatal(http.ListenAndServe(port, nil))
}
