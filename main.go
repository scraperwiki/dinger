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

func CreateSlackMessage(eventData []byte) SlackMessage {
	// Assumes topic is of the form domain/name/icon␀msg
	// but domain is ignored (should probably be slack.scraperwiki.com)
	i := bytes.IndexByte(eventData, byte('\x00'))
	if i == -1 {
		return SlackMessage{
			string(eventData),
			"dinger",
			":broken_heart:",
			"#log",
		}
	}
	splitEventData := bytes.SplitN(eventData, []byte("\x00"), 2)
	route := splitEventData[0]
	text := splitEventData[1]
	splitRoute := bytes.SplitN(route, []byte("/"), 3)
	name := splitRoute[1]
	icon := splitRoute[2]
	return SlackMessage{
		string(text),
		string(name),
		string(icon),
		"#log",
	}

}

func SendToSlack(eventData []byte) {
	if slackUrl == "" {
		return
	}

	msg := CreateSlackMessage(eventData)
	jsonMsg, _ := json.Marshal(msg)
	msgReader := bytes.NewReader(jsonMsg)

	resp, err := http.Post(slackUrl, "", msgReader)
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
	ringSubscribeUrl := os.Getenv("DINGER_RING_SUB_URL")
	if ringSubscribeUrl == "" {
		log.Fatal("DINGER_RING_SUB_URL not set")
	}

	logSubscribeUrl := os.Getenv("DINGER_LOG_SUB_URL")
	if logSubscribeUrl == "" {
		log.Print("DINGER_LOG_SUB_URL not set: will not notify in chat")
	}

	slackUrl = os.Getenv("SLACK_WEBHOOK_URL")
	if slackUrl == "" {
		log.Print("SLACK_WEBHOOK_URL not set: will not notify in chat")
	}

	port := os.Getenv("PORT")
	host := os.Getenv("HOST")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprint(host, ":", port)

	header := http.Header{}
	ringEvents, ringErrs := listen.RetryingWatch(ringSubscribeUrl, header, nil)
	logEvents, logErrs := listen.RetryingWatch(logSubscribeUrl, header, nil)

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
			SendToSlack(eventData)
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
