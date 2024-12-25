package main

import (
	"log"
	"net/http"
	"strings"

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
	// Start a new container
	c := services.NewContainer()
	defer func() {
		if err := c.Shutdown(); err != nil {
			log.Fatal(err)
		}
	}()
	// listen to websock and trigger a go routine to print From, Subject, Body
	// from regex
	header := http.Header{}
	client, _, err := websocket.DefaultDialer.Dial(c.Config.Mailhog.Ws, header)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer client.Close()

	done := make(chan struct{})

	func() {
		defer close(done)
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
			go processMsg(msg)
		}
	}()
}

func processMsg(msg Message) {
	log.Println(msg)
}
