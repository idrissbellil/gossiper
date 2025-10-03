package worker

import (
	"context"
	"fmt"
	"net/http"
)

type WebSocketClient struct {
	dialer WebSocketDialer
	logger Logger
	config Config
}

func NewWebSocketClient(dialer WebSocketDialer, logger Logger, config Config) *WebSocketClient {
	return &WebSocketClient{
		dialer: dialer,
		logger: logger,
		config: config,
	}
}

func (w *WebSocketClient) Connect() (WSClient, error) {
	header := http.Header{}
	conn, _, err := w.dialer.Dial(w.config.WebSocketURL, header)
	if err != nil {
		return nil, fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	w.logger.Printf("connected to WebSocket: %s", w.config.WebSocketURL)
	return conn, nil
}

func (w *WebSocketClient) ReadMessages(ctx context.Context, conn WSClient, msgChan chan<- RawMessage, errorChan chan<- error) {
	defer close(msgChan)
	defer close(errorChan)

	for {
		select {
		case <-ctx.Done():
			w.logger.Println("WebSocket client stopping due to context cancellation")
			return
		default:
			var rawMsg RawMessage
			err := conn.ReadJSON(&rawMsg)
			if err != nil {
				select {
				case errorChan <- fmt.Errorf("failed to read WebSocket message: %w", err):
				case <-ctx.Done():
				}
				return
			}

			select {
			case msgChan <- rawMsg:
			case <-ctx.Done():
				return
			}
		}
	}
}