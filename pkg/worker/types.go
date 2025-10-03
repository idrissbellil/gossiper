package worker

import (
	"context"
	"net/http"
	"time"

	"gitea.v3m.net/idriss/gossiper/pkg/models"
	"github.com/gorilla/websocket"
)

type RawMessage struct {
	ID                 string `json:"id"`
	Time               int64  `json:"time"`
	From               EmailAddress `json:"from"`
	To                 []EmailAddress `json:"to"`
	Subject            string `json:"subject"`
	Date               string `json:"date"`
	Size               string `json:"size"`
	Opened             bool   `json:"opened"`
	HasHTML            bool   `json:"has_html"`
	HasPlain           bool   `json:"has_plain"`
	Attachments        []interface{} `json:"attachments"`
	EnvelopeFrom       string `json:"envelope_from"`
	EnvelopeRecipients []string `json:"envelope_recipients"`
}

type EmailAddress struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type MailcrabMessage struct {
	ID          string                 `json:"id"`
	Time        int64                  `json:"time"`
	From        EmailAddress           `json:"from"`
	To          []EmailAddress         `json:"to"`
	Subject     string                 `json:"subject"`
	Date        string                 `json:"date"`
	Size        string                 `json:"size"`
	Opened      bool                   `json:"opened"`
	Headers     map[string]string      `json:"headers"`
	Text        string                 `json:"text"`
	HTML        string                 `json:"html"`
	Attachments []interface{}          `json:"attachments"`
	Raw         string                 `json:"raw"`
	EnvelopeFrom       string         `json:"envelope_from"`
	EnvelopeRecipients []string       `json:"envelope_recipients"`
}

type Message struct {
	From    string `json:"From"`
	To      string `json:"To"`
	Subject string `json:"Subject"`
	Body    string `json:"Body"`
}

type Config struct {
	WebSocketURL    string
	APIURL          string
	HTTPTimeout     time.Duration
	MaxRetries      int
	BufferSize      int
	ShutdownTimeout time.Duration
}

type DefaultConfig struct{}

func (d DefaultConfig) GetConfig() Config {
	return Config{
		HTTPTimeout:     30 * time.Second,
		MaxRetries:      3,
		BufferSize:      100,
		ShutdownTimeout: 10 * time.Second,
	}
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type WSClient interface {
	ReadJSON(v interface{}) error
	Close() error
}

type JobRepository interface {
	GetActiveJobs(ctx context.Context, email string) ([]*models.Job, error)
}

type MessageFetcherInterface interface {
	FetchMessage(messageID string) (*MailcrabMessage, error)
	GetMessageBody(msg *MailcrabMessage) string
}

type Logger interface {
	Printf(format string, args ...interface{})
	Println(args ...interface{})
}

type WebSocketDialer interface {
	Dial(urlStr string, requestHeader http.Header) (WSClient, *http.Response, error)
}

type DefaultWebSocketDialer struct{}

func (d DefaultWebSocketDialer) Dial(urlStr string, requestHeader http.Header) (WSClient, *http.Response, error) {
	conn, resp, err := websocket.DefaultDialer.Dial(urlStr, requestHeader)
	return conn, resp, err
}