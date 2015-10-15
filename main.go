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

var slackUrl string

type SlackMessage struct {
	Text      string `json:"text"`
	Username  string `json:"username"`
	IconEmoji string `json:"icon_emoji"`
	Channel   string `json:"channel"`
}

func SendToSlack(eventData []byte) {
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

}

func main() {
	dingerSubscribeUrl := os.Getenv("DINGER_SUBSCRIBE_URL")
	if dingerSubscribeUrl == "" {
		log.Fatal("DINGER_SUBSCRIBE_URL not set")
	}

	ircSubscribeUrl := os.Getenv("IRC_SUBSCRIBE_URL")
	if ircSubscribeUrl == "" {
		log.Print("IRC_SUBSCRIBE_URL not set: will not notify in chat")
	}

	slackUrl = os.Getenv("SLACK_WEBHOOK_URL")
	if slackUrl == "" {
		log.Print("SLACK_WEBHOOT_URL not set: will not notify in chat")
	}

	port := os.Getenv("PORT")
	host := os.Getenv("HOST")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprint(host, ":", port)

	header := http.Header{}
	dingerEvents, dingerErrs := listen.RetryingWatch(dingerSubscribeUrl, header, nil)
	ircEvents, ircErrs := listen.RetryingWatch(ircSubscribeUrl, header, nil)

	go func() {
		for err := range dingerErrs {
			log.Printf("Error in dinger hookbot event stream: %v", err)
		}
	}()

	go func() {
		for err := range ircErrs {
			log.Printf("Error in irc hookbot event stream: %v", err)
		}
	}()

	var (
		mu         sync.Mutex
		eventTimes []time.Time
	)

	go func() {
		for eventData := range ircEvents {
			log.Printf("Received IRC event: %q", eventData)
			SendToSlack(eventData)
		}
	}()

	// keep track of 'dinger' calls
	go func() {
		for eventData := range dingerEvents {
			log.Printf("Received dinger event: %q", eventData)

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

	// endpoint that Acker's bell listens to
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
