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

	"github.com/sensiblecodeio/hookbot/pkg/listen"
)

var slackURL string

type slackMessage struct {
	Text      string `json:"text"`
	Username  string `json:"username"`
	IconEmoji string `json:"icon_emoji"`
	Channel   string `json:"channel"`
}

func createslackMessage(eventData []byte) slackMessage {
	// Assumes eventData is of the form domain/channel/name/icon␀msg
	// but domain is ignored (should probably be slack.scraperwiki.com)
	i := bytes.IndexByte(eventData, byte('\x00'))
	if i == -1 {
		return slackMessage{
			string(eventData),
			"dinger",
			":broken_heart:",
			"#pdftables-bots",
		}
	}

	splitEventData := bytes.SplitN(eventData, []byte("\x00"), 2)
	route := splitEventData[0]
	text := splitEventData[1]

	splitRoute := bytes.SplitN(route, []byte("/"), 3)
	domain := splitRoute[0]
	_ = domain // Unused.
	channel := splitRoute[1]
	name := splitRoute[2]
	icon := splitRoute[3]

	return slackMessage{
		string(text),
		string(name),
		string(icon),
		"#" + string(channel),
	}

}

func sendToSlack(eventData []byte) {
	if slackURL == "" {
		return
	}

	msg := createslackMessage(eventData)
	jsonMsg, _ := json.Marshal(msg)
	msgReader := bytes.NewReader(jsonMsg)

	resp, err := http.Post(slackURL, "", msgReader)
	if err != nil {
		log.Printf("Error sending message to slack: %v", err)
		return
	}
	if resp.StatusCode != 200 {
		log.Printf("Slack not OK: %v", resp)
	}
	defer resp.Body.Close()
}

func main() {
	ringSubscribeURL := os.Getenv("DINGER_RING_SUB_URL")
	if ringSubscribeURL == "" {
		log.Fatal("DINGER_RING_SUB_URL not set")
	}

	logSubscribeURL := os.Getenv("DINGER_LOG_SUB_URL")
	if logSubscribeURL == "" {
		log.Print("DINGER_LOG_SUB_URL not set: will not notify in chat")
	}

	slackURL = os.Getenv("SLACK_WEBHOOK_URL")
	if slackURL == "" {
		log.Print("SLACK_WEBHOOK_URL not set: will not notify in chat")
	}

	port := os.Getenv("PORT")
	host := os.Getenv("HOST")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprint(host, ":", port)

	header := http.Header{}
	ringEvents, ringErrs := listen.RetryingWatch(ringSubscribeURL, header, nil)
	logEvents, logErrs := listen.RetryingWatch(logSubscribeURL, header, nil)

	go func() {
		for err := range ringErrs {
			log.Printf("Error with ring event stream: %v", err)
		}
	}()

	go func() {
		for err := range logErrs {
			log.Printf("Error with log hookbot event stream: %v", err)
		}
	}()

	var (
		mu         sync.Mutex
		eventTimes []time.Time
	)

	go func() {
		for eventData := range logEvents {
			log.Printf("Received log event: %q", eventData)
			sendToSlack(eventData)
		}
	}()

	// keep track of ring calls
	go func() {
		for eventData := range ringEvents {
			log.Printf("Received ring event: %q", eventData)

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
