package worker

import (
	"context"
	"net/http"
	"time"

	"gitea.v3m.net/idriss/gossiper/ent"
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
	From    string `json:"From"`
	To      string `json:"To"`
	Subject string `json:"Subject"`
	Body    string `json:"Body"`
}

type Config struct {
	WebSocketURL    string
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
	GetActiveJobs(ctx context.Context, email string) ([]*ent.Job, error)
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