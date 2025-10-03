package worker

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

type mockWSConn struct {
	messages []interface{}
	readErr  error
	closed   bool
}

func (m *mockWSConn) ReadJSON(v interface{}) error {
	if m.readErr != nil {
		return m.readErr
	}

	if len(m.messages) == 0 {
		time.Sleep(100 * time.Millisecond)
		return errors.New("no more messages")
	}

	msg := m.messages[0]
	m.messages = m.messages[1:]

	switch v := v.(type) {
	case *RawMessage:
		if rawMsg, ok := msg.(RawMessage); ok {
			*v = rawMsg
		}
	}

	return nil
}

func (m *mockWSConn) Close() error {
	m.closed = true
	return nil
}

type mockWebSocketDialer struct {
	conn *mockWSConn
	err  error
}

func (m *mockWebSocketDialer) Dial(urlStr string, requestHeader http.Header) (WSClient, *http.Response, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return m.conn, &http.Response{StatusCode: 101}, nil
}

func TestWebSocketClient_Connect(t *testing.T) {
	tests := []struct {
		name        string
		dialError   error
		expectError bool
	}{
		{
			name:        "successful connection",
			expectError: false,
		},
		{
			name:        "connection failure",
			dialError:   errors.New("connection failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			config := Config{WebSocketURL: "ws://localhost:8080/ws"}

			mockDialer := &mockWebSocketDialer{
				conn: &mockWSConn{},
				err:  tt.dialError,
			}

			client := NewWebSocketClient(mockDialer, logger, config)

			conn, err := client.Connect()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if conn != nil {
					t.Error("expected nil connection on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if conn == nil {
					t.Error("expected connection, got nil")
				}
			}
		})
	}
}

func TestWebSocketClient_ReadMessages(t *testing.T) {
	testMessage := RawMessage{
		Content: struct {
			Headers struct {
				To      []string `json:"To"`
				From    []string `json:"From"`
				Subject []string `json:"Subject"`
			} `json:"Headers"`
			Body string `json:"Body"`
		}{
			Headers: struct {
				To      []string `json:"To"`
				From    []string `json:"From"`
				Subject []string `json:"Subject"`
			}{
				To:      []string{"test@example.com"},
				From:    []string{"sender@example.com"},
				Subject: []string{"Test Subject"},
			},
			Body: "Test body",
		},
	}

	tests := []struct {
		name             string
		messages         []interface{}
		readErr          error
		expectMessages   int
		expectError      bool
		cancelAfterStart bool
	}{
		{
			name:           "successful message reading",
			messages:       []interface{}{testMessage},
			expectMessages: 1,
		},
		{
			name:    "read error",
			readErr: errors.New("read failed"),
			expectError: true,
		},
		{
			name:             "context cancellation",
			messages:         []interface{}{testMessage},
			cancelAfterStart: true,
			expectMessages:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			config := Config{}

			mockConn := &mockWSConn{
				messages: tt.messages,
				readErr:  tt.readErr,
			}

			client := NewWebSocketClient(nil, logger, config)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			msgChan := make(chan RawMessage, 10)
			errorChan := make(chan error, 1)

			if tt.cancelAfterStart {
				go func() {
					time.Sleep(5 * time.Millisecond)
					cancel()
				}()
			}

			go client.ReadMessages(ctx, mockConn, msgChan, errorChan)

			var receivedMessages []RawMessage
			var receivedError error

			timeout := time.After(100 * time.Millisecond)

		loop:
			for {
				select {
				case msg, ok := <-msgChan:
					if !ok {
						break loop
					}
					receivedMessages = append(receivedMessages, msg)

				case err, ok := <-errorChan:
					if !ok {
						break loop
					}
					receivedError = err
					break loop

				case <-timeout:
					break loop
				}
			}

			if tt.expectError {
				if receivedError == nil {
					t.Error("expected error, got nil")
				}
			} else if !tt.cancelAfterStart {
				if receivedError != nil {
					t.Errorf("unexpected error: %v", receivedError)
				}
			}

			if len(receivedMessages) != tt.expectMessages {
				t.Errorf("expected %d messages, got %d", tt.expectMessages, len(receivedMessages))
			}

			if tt.expectMessages > 0 && len(receivedMessages) > 0 {
				msg := receivedMessages[0]
				if len(msg.Content.Headers.To) == 0 || msg.Content.Headers.To[0] != "test@example.com" {
					t.Error("message content not preserved correctly")
				}
			}
		})
	}
}

func TestWebSocketClient_ReadMessages_ContextCancellation(t *testing.T) {
	logger := &mockLogger{}
	config := Config{}

	mockConn := &mockWSConn{
		messages: []interface{}{},
	}

	client := NewWebSocketClient(nil, logger, config)

	ctx, cancel := context.WithCancel(context.Background())

	msgChan := make(chan RawMessage, 10)
	errorChan := make(chan error, 1)

	go client.ReadMessages(ctx, mockConn, msgChan, errorChan)

	time.Sleep(5 * time.Millisecond)
	cancel()

	timeout := time.After(200 * time.Millisecond)

	var msgChannelClosed, errorChannelClosed bool

	for {
		select {
		case _, ok := <-msgChan:
			if !ok {
				msgChannelClosed = true
			}
		case _, ok := <-errorChan:
			if !ok {
				errorChannelClosed = true
			}
		case <-timeout:
			break
		}

		if msgChannelClosed && errorChannelClosed {
			break
		}
	}

	if !msgChannelClosed || !errorChannelClosed {
		t.Error("both channels should be closed on context cancellation")
	}
}

func TestWebSocketClient_ReadMessages_ChannelBlocking(t *testing.T) {
	logger := &mockLogger{}
	config := Config{}

	testMessage := RawMessage{
		Content: struct {
			Headers struct {
				To      []string `json:"To"`
				From    []string `json:"From"`
				Subject []string `json:"Subject"`
			} `json:"Headers"`
			Body string `json:"Body"`
		}{
			Headers: struct {
				To      []string `json:"To"`
				From    []string `json:"From"`
				Subject []string `json:"Subject"`
			}{
				To: []string{"test@example.com"},
			},
			Body: "Test",
		},
	}

	mockConn := &mockWSConn{
		messages: []interface{}{testMessage},
	}

	client := NewWebSocketClient(nil, logger, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgChan := make(chan RawMessage)
	errorChan := make(chan error, 1)

	go client.ReadMessages(ctx, mockConn, msgChan, errorChan)

	cancel()

	timeout := time.After(100 * time.Millisecond)
	select {
	case <-msgChan:
	case <-errorChan:
	case <-timeout:
		t.Error("ReadMessages should handle context cancellation when channels are blocked")
	}
}