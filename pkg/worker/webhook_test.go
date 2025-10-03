package worker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockHTTPClient struct {
	responses map[string]*http.Response
	requests  []*http.Request
	err       error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)

	if m.err != nil {
		return nil, m.err
	}

	if response, exists := m.responses[req.URL.String()]; exists {
		return response, nil
	}

	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("OK")),
	}, nil
}

func TestWebhookSender_SendWebhook(t *testing.T) {
	tests := []struct {
		name           string
		processResult  ProcessResult
		httpResponse   *http.Response
		httpError      error
		expectedStatus int
		expectError    bool
	}{
		{
			name: "successful webhook",
			processResult: ProcessResult{
				JobID:   1,
				URL:     "http://example.com/webhook",
				Method:  "POST",
				Headers: map[string]string{"X-Custom": "value"},
				Payload: `{"test": "data"}`,
			},
			httpResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("OK")),
			},
			expectedStatus: 200,
		},
		{
			name: "HTTP error",
			processResult: ProcessResult{
				JobID:   2,
				URL:     "http://example.com/webhook",
				Method:  "POST",
				Headers: map[string]string{},
				Payload: `{"test": "data"}`,
			},
			httpError:   errors.New("connection failed"),
			expectError: true,
		},
		{
			name: "processing error - should skip webhook",
			processResult: ProcessResult{
				JobID: 3,
				Error: errors.New("processing failed"),
			},
			expectError: true,
		},
		{
			name: "webhook with 4xx status",
			processResult: ProcessResult{
				JobID:   4,
				URL:     "http://example.com/webhook",
				Method:  "POST",
				Headers: map[string]string{},
				Payload: `{"test": "data"}`,
			},
			httpResponse: &http.Response{
				StatusCode: 400,
				Body:       io.NopCloser(strings.NewReader("Bad Request")),
			},
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}

			mockClient := &mockHTTPClient{
				responses: make(map[string]*http.Response),
				err:       tt.httpError,
			}

			if tt.httpResponse != nil {
				mockClient.responses[tt.processResult.URL] = tt.httpResponse
			}

			config := Config{HTTPTimeout: 30}
			sender := NewWebhookSender(mockClient, logger, config)

			result := sender.SendWebhook(context.Background(), tt.processResult)

			if tt.expectError {
				if result.Error == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if result.Error != nil {
					t.Errorf("unexpected error: %v", result.Error)
				}
				if result.StatusCode != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, result.StatusCode)
				}
			}

			if result.JobID != tt.processResult.JobID {
				t.Errorf("expected job ID %d, got %d", tt.processResult.JobID, result.JobID)
			}

			if !tt.expectError && tt.processResult.Error == nil && len(mockClient.requests) == 1 {
				req := mockClient.requests[0]
				if req.Method != tt.processResult.Method {
					t.Errorf("expected method %s, got %s", tt.processResult.Method, req.Method)
				}
				if req.URL.String() != tt.processResult.URL {
					t.Errorf("expected URL %s, got %s", tt.processResult.URL, req.URL.String())
				}

				body, _ := io.ReadAll(req.Body)
				if string(body) != tt.processResult.Payload {
					t.Errorf("expected payload %s, got %s", tt.processResult.Payload, string(body))
				}

				for key, value := range tt.processResult.Headers {
					if req.Header.Get(key) != value {
						t.Errorf("expected header %s: %s, got %s", key, value, req.Header.Get(key))
					}
				}

				if req.Header.Get("Content-Type") == "" {
					t.Error("Content-Type header should be set to application/json by default")
				}
			}
		})
	}
}

func TestWebhookSender_buildRequest(t *testing.T) {
	logger := &mockLogger{}
	config := Config{}
	sender := NewWebhookSender(nil, logger, config)

	processResult := ProcessResult{
		JobID:   1,
		URL:     "http://example.com/webhook",
		Method:  "POST",
		Headers: map[string]string{"X-Custom": "value", "Content-Type": "application/xml"},
		Payload: `<xml>test</xml>`,
	}

	req, err := sender.buildRequest(context.Background(), processResult)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Method != processResult.Method {
		t.Errorf("expected method %s, got %s", processResult.Method, req.Method)
	}

	if req.URL.String() != processResult.URL {
		t.Errorf("expected URL %s, got %s", processResult.URL, req.URL.String())
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(body) != processResult.Payload {
		t.Errorf("expected payload %s, got %s", processResult.Payload, string(body))
	}

	if req.Header.Get("X-Custom") != "value" {
		t.Errorf("expected X-Custom header to be 'value', got %s", req.Header.Get("X-Custom"))
	}

	if req.Header.Get("Content-Type") != "application/xml" {
		t.Error("Content-Type should not be overridden when already set")
	}
}

func TestWebhookSender_buildRequest_DefaultContentType(t *testing.T) {
	logger := &mockLogger{}
	config := Config{}
	sender := NewWebhookSender(nil, logger, config)

	processResult := ProcessResult{
		JobID:   1,
		URL:     "http://example.com/webhook",
		Method:  "POST",
		Headers: map[string]string{},
		Payload: `{"test": "data"}`,
	}

	req, err := sender.buildRequest(context.Background(), processResult)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected default Content-Type to be 'application/json', got %s", req.Header.Get("Content-Type"))
	}
}

func TestWebhookSender_SendWebhooks(t *testing.T) {
	logger := &mockLogger{}
	config := Config{}

	mockClient := &mockHTTPClient{
		responses: map[string]*http.Response{
			"http://example1.com/webhook": {
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("OK")),
			},
			"http://example2.com/webhook": {
				StatusCode: 201,
				Body:       io.NopCloser(strings.NewReader("Created")),
			},
		},
	}

	sender := NewWebhookSender(mockClient, logger, config)

	results := []ProcessResult{
		{
			JobID:   1,
			URL:     "http://example1.com/webhook",
			Method:  "POST",
			Headers: map[string]string{},
			Payload: `{"test1": "data1"}`,
		},
		{
			JobID:   2,
			URL:     "http://example2.com/webhook",
			Method:  "PUT",
			Headers: map[string]string{},
			Payload: `{"test2": "data2"}`,
		},
	}

	webhookResults := sender.SendWebhooks(context.Background(), results)

	if len(webhookResults) != 2 {
		t.Errorf("expected 2 webhook results, got %d", len(webhookResults))
	}

	for i, result := range webhookResults {
		if result.Error != nil {
			t.Errorf("webhook result %d: unexpected error: %v", i, result.Error)
		}

		expectedStatus := []int{200, 201}[i]
		if result.StatusCode != expectedStatus {
			t.Errorf("webhook result %d: expected status %d, got %d", i, expectedStatus, result.StatusCode)
		}
	}

	if len(mockClient.requests) != 2 {
		t.Errorf("expected 2 HTTP requests, got %d", len(mockClient.requests))
	}
}