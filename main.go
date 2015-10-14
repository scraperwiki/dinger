package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/scraperwiki/hookbot/pkg/listen"
)

type SlackMessage struct {
	Text      string `json:"text"`
	Username  string `json:"username"`
	IconEmoji string `json:"icon_emoji"`
	Channel   string `json:"channel"`
}

func main() {
	hookbotUrl := os.Getenv("HOOKBOT_LISTEN_URL")
	if hookbotUrl == "" {
		log.Fatal("HOOKBOT_LISTEN_URL not set")
	}
	slackUrl := os.Getenv("SLACK_WEBHOOK_URL")
	if slackUrl == "" {
		log.Print("SLACK_WEBHOOT_URL not set: will not notify in chat")
	}
	port := os.Getenv("PORT")
	host := os.Getenv("HOST")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprint(host, ":", port)

	finish := make(chan struct{})

	header := http.Header{}
	events, errs := listen.RetryingWatch(hookbotUrl, header, finish)

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
		for eventData := range events {
			log.Printf("Received event: %q", eventData)
			go func() {
				if slackUrl == "" {
					return
				}
				jsonMsg, _ := json.Marshal(SlackMessage{string(eventData), "dinger", ":broken_heart:", "#log"})
				msgReader := bytes.NewReader(jsonMsg)

				resp, err := http.Post(slackUrl, "", msgReader)
				if err != nil {
					log.Printf("Error sending message to slack: %v", err)
				}
				if resp.StatusCode != 200 {
					log.Printf("Slack not OK: %v", resp)
				}
			}()

			func() {
				mu.Lock()
				defer mu.Unlock()

				dingCount := 1 // default
				_, err := fmt.Sscan(string(eventData), &dingCount)

				if err != nil && len(eventData) != 0 {
					log.Printf("Event data not a number: %q", eventData)
				}

				for i := 0; i < dingCount; i++ {
					eventTimes = append([]time.Time{time.Now()}, eventTimes...)
					log.Printf("Ding!")
				}

				const maxEvents = 10
				if len(eventTimes) > maxEvents {
					eventTimes = eventTimes[:maxEvents]
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

	log.Fatal(http.ListenAndServe(addr, nil))
}
