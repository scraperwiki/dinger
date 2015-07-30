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
		for payload := range events {
			log.Printf("Signalled via hookbot, content of payload:")
			log.Printf("%s", payload)
			func() {
				mu.Lock()
				defer mu.Unlock()
				eventTimes = append(eventTimes, time.Now())
			}()
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(w, "<foo><bar>%v</foo></bar>\n", eventTimes)
	})

	log.Fatal(http.ListenAndServe(port, nil))
}
