package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"gitea.risky.info/risky-info/gossiper/ent"
	"gitea.risky.info/risky-info/gossiper/ent/job"
	"gitea.risky.info/risky-info/gossiper/pkg/services"
	"github.com/gorilla/websocket"
)

type RawMessage struct {
	Content struct {
		Headers struct {
			To      []string `json:"To"`
			From    []string `json:"From"`
			Subject []string `json:"Subject"`
		} `json:"Headers"`
		Body string `json:"Body"`
	} `json:"Content"`
}

type Message struct {
	From    string   `json:"From"`
	To      []string `json:"To"`
	Subject string   `json:"Subject"`
	Body    string   `json:"Body"`
}

func firstOrEmpty(arr []string) string {
	if len(arr) > 0 {
		return arr[0]
	}
	return ""
}

func main() {
	c := services.NewContainer()
	defer func() {
		if err := c.Shutdown(); err != nil {
			log.Fatal(err)
		}
	}()
	header := http.Header{}
	client, _, err := websocket.DefaultDialer.Dial(c.Config.Mailhog.Ws, header)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer client.Close()

	done := make(chan struct{})

	func() {
		defer close(done)
		log.Println("worker started")
		for {
			var rawMsg RawMessage
			if err := client.ReadJSON(&rawMsg); err != nil {
				log.Println("receiving error: ", err)
			}
			msg := Message{
				To:      strings.Split(rawMsg.Content.Headers.To[0], ","),
				From:    rawMsg.Content.Headers.From[0],
				Subject: rawMsg.Content.Headers.Subject[0],
				Body:    rawMsg.Content.Body,
			}
			go processMsg(msg, c.ORM)
		}
	}()
}

func processMsg(msg Message, client *ent.Client) error {
	jobs, err := client.Job.Query().
		Where(job.EmailIn(msg.To...)).
		Where(job.IsActive(true)).
		All(context.Background())
	if err != nil {
		return fmt.Errorf("failed querying jobs: %w", err)
	}

	for _, job := range jobs {
		if matched, err := regexp.MatchString(job.FromRegex, msg.From); err != nil {
			log.Printf("invalid from_regex pattern for job %d: %v", job.ID, err)
			continue
		} else if !matched {
			continue
		}

		var payload string
		if job.PayloadTemplate != "" {
			tmpl, err := template.New("payload").Parse(job.PayloadTemplate)
			if err != nil {
				log.Printf("invalid payload template for job %d: %v", job.ID, err)
				continue
			}

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, msg)
			if err != nil {
				log.Printf("failed executing template for job %d: %v", job.ID, err)
				continue
			}
			payload = buf.String()
		} else {
			jsonBytes, err := json.Marshal(msg)
			if err != nil {
				log.Printf("failed marshaling message for job %d: %v", job.ID, err)
				continue
			}
			payload = string(jsonBytes)
		}

		req, err := http.NewRequestWithContext(context.Background(), job.Method.String(), job.URL, strings.NewReader(payload))
		if err != nil {
			log.Printf("failed creating request for job %d: %v", job.ID, err)
			continue
		}

		for key, value := range job.Headers {
			req.Header.Set(key, value)
		}

		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}

		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("failed sending request for job %d: %v", job.ID, err)
			continue
		}
		defer resp.Body.Close()

		log.Printf("webhook call for job %d completed with status: %d", job.ID, resp.StatusCode)
	}

	return nil
}
