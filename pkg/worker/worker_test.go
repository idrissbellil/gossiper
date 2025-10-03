package worker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"gitea.v3m.net/idriss/gossiper/ent"
	"gitea.v3m.net/idriss/gossiper/ent/job"
)

type mockWorkerDeps struct {
	jobRepo    *mockJobRepository
	logger     *mockLogger
	config     Config
	wsDialer   *mockWebSocketDialer
	httpClient *mockHTTPClient
}

func newMockWorkerDeps() mockWorkerDeps {
	return mockWorkerDeps{
		jobRepo:    &mockJobRepository{jobs: make(map[string][]*ent.Job)},
		logger:     &mockLogger{},
		config:     Config{BufferSize: 10, ShutdownTimeout: 1 * time.Second},
		wsDialer:   &mockWebSocketDialer{},
		httpClient: &mockHTTPClient{responses: make(map[string]*http.Response)},
	}
}

func (m mockWorkerDeps) toDependencies() WorkerDependencies {
	return WorkerDependencies{
		JobRepo:    m.jobRepo,
		Logger:     m.logger,
		Config:     m.config,
		WSDialer:   m.wsDialer,
		HTTPClient: m.httpClient,
	}
}

func TestWorker_Integration(t *testing.T) {
	method := job.MethodPOST

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
				To:      []string{"webhook@example.com"},
				From:    []string{"sender@example.com"},
				Subject: []string{"Test Alert"},
			},
			Body: "Alert: Something happened",
		},
	}

	mockJob := &ent.Job{
		ID:        1,
		Email:     "webhook@example.com",
		FromRegex: "sender@.*",
		URL:       "http://webhook.example.com/notify",
		Method:    method,
		Headers:   map[string]string{"X-API-Key": "secret"},
	}

	deps := newMockWorkerDeps()

	deps.jobRepo.jobs["webhook@example.com"] = []*ent.Job{mockJob}

	deps.wsDialer.conn = &mockWSConn{
		messages: []interface{}{testMessage},
	}

	deps.httpClient.responses["http://webhook.example.com/notify"] = &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("OK")),
	}

	worker := NewWorker(deps.toDependencies())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var workerErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		workerErr = worker.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	worker.Shutdown()

	wg.Wait()

	if workerErr != nil && workerErr != context.DeadlineExceeded && !strings.Contains(workerErr.Error(), "no more messages") {
		t.Errorf("unexpected worker error: %v", workerErr)
	}

	if len(deps.httpClient.requests) != 1 {
		t.Errorf("expected 1 HTTP request, got %d", len(deps.httpClient.requests))
		return
	}

	req := deps.httpClient.requests[0]
	if req.URL.String() != "http://webhook.example.com/notify" {
		t.Errorf("expected webhook URL, got %s", req.URL.String())
	}

	if req.Header.Get("X-API-Key") != "secret" {
		t.Errorf("expected API key header to be preserved")
	}
}

func TestWorker_StartStop(t *testing.T) {
	deps := newMockWorkerDeps()
	deps.wsDialer.conn = &mockWSConn{}

	worker := NewWorker(deps.toDependencies())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- worker.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	worker.Shutdown()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("worker did not shut down within timeout")
	}
}

func TestWorker_WebSocketConnectionFailure(t *testing.T) {
	deps := newMockWorkerDeps()
	deps.wsDialer.err = errors.New("connection failed")

	worker := NewWorker(deps.toDependencies())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := worker.Start(ctx)
	if err == nil {
		t.Error("expected error when WebSocket connection fails")
	}
}

func TestWorker_MessageProcessingWithMultipleJobs(t *testing.T) {
	method := job.MethodPOST

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
				To:      []string{"alert@example.com"},
				From:    []string{"system@example.com"},
				Subject: []string{"System Alert"},
			},
			Body: "Critical alert",
		},
	}

	job1 := &ent.Job{
		ID:        1,
		Email:     "alert@example.com",
		FromRegex: "system@.*",
		URL:       "http://webhook1.example.com/notify",
		Method:    method,
		Headers:   map[string]string{},
	}

	job2 := &ent.Job{
		ID:        2,
		Email:     "alert@example.com",
		FromRegex: "system@.*",
		URL:       "http://webhook2.example.com/notify",
		Method:    method,
		Headers:   map[string]string{},
	}

	deps := newMockWorkerDeps()
	deps.jobRepo.jobs["alert@example.com"] = []*ent.Job{job1, job2}
	deps.wsDialer.conn = &mockWSConn{
		messages: []interface{}{testMessage},
	}

	deps.httpClient.responses["http://webhook1.example.com/notify"] = &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("OK")),
	}
	deps.httpClient.responses["http://webhook2.example.com/notify"] = &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("OK")),
	}

	worker := NewWorker(deps.toDependencies())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		time.Sleep(200 * time.Millisecond)
		worker.Shutdown()
	}()

	worker.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	if len(deps.httpClient.requests) != 2 {
		t.Errorf("expected 2 HTTP requests, got %d", len(deps.httpClient.requests))
	}

	urls := make(map[string]bool)
	for _, req := range deps.httpClient.requests {
		urls[req.URL.String()] = true
	}

	expectedURLs := []string{
		"http://webhook1.example.com/notify",
		"http://webhook2.example.com/notify",
	}

	for _, expectedURL := range expectedURLs {
		if !urls[expectedURL] {
			t.Errorf("expected request to %s not found", expectedURL)
		}
	}
}

func TestWorker_NoMatchingJobs(t *testing.T) {
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
				To:   []string{"nobody@example.com"},
				From: []string{"sender@example.com"},
			},
			Body: "Test message",
		},
	}

	deps := newMockWorkerDeps()
	deps.wsDialer.conn = &mockWSConn{
		messages: []interface{}{testMessage},
	}

	worker := NewWorker(deps.toDependencies())

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() {
		time.Sleep(200 * time.Millisecond)
		worker.Shutdown()
	}()

	worker.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	if len(deps.httpClient.requests) != 0 {
		t.Errorf("expected no HTTP requests for no matching jobs, got %d", len(deps.httpClient.requests))
	}

	if len(deps.logger.messages) == 0 {
		t.Error("expected log messages to be generated")
	}
}

func TestWorker_ContextCancellation(t *testing.T) {
	testMessage := RawMessage{
		Content: struct {
			Headers struct {
				To      []string `json:"To"`
				From    []string `json:"From"`
				Subject []string `json:"Subject"`
			} `json:"Headers"`
			Body string `json:"Body"`
		}{},
	}

	deps := newMockWorkerDeps()
	deps.wsDialer.conn = &mockWSConn{
		messages: []interface{}{testMessage, testMessage, testMessage},
	}

	worker := NewWorker(deps.toDependencies())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- worker.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("worker did not stop after context cancellation")
	}
}