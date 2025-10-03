package worker

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type WebhookSender struct {
	httpClient HTTPClient
	logger     Logger
	config     Config
}

func NewWebhookSender(httpClient HTTPClient, logger Logger, config Config) *WebhookSender {
	return &WebhookSender{
		httpClient: httpClient,
		logger:     logger,
		config:     config,
	}
}

type WebhookResult struct {
	JobID      int
	StatusCode int
	Error      error
}

func (w *WebhookSender) SendWebhook(ctx context.Context, result ProcessResult) WebhookResult {
	webhookResult := WebhookResult{
		JobID: result.JobID,
	}

	if result.Error != nil {
		webhookResult.Error = result.Error
		w.logger.Printf("skipping webhook for job %d due to processing error: %v", result.JobID, result.Error)
		return webhookResult
	}

	req, err := w.buildRequest(ctx, result)
	if err != nil {
		webhookResult.Error = fmt.Errorf("failed to build request: %w", err)
		w.logger.Printf("failed to build request for job %d: %v", result.JobID, err)
		return webhookResult
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		webhookResult.Error = fmt.Errorf("failed to send request: %w", err)
		w.logger.Printf("failed to send request for job %d: %v", result.JobID, err)
		return webhookResult
	}
	defer resp.Body.Close()

	webhookResult.StatusCode = resp.StatusCode
	w.logger.Printf("webhook call for job %d completed with status: %d", result.JobID, resp.StatusCode)

	return webhookResult
}

func (w *WebhookSender) buildRequest(ctx context.Context, result ProcessResult) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, result.Method, result.URL, strings.NewReader(result.Payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for key, value := range result.Headers {
		req.Header.Set(key, value)
	}

	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func (w *WebhookSender) SendWebhooks(ctx context.Context, results []ProcessResult) []WebhookResult {
	var webhookResults []WebhookResult

	for _, result := range results {
		webhookResult := w.SendWebhook(ctx, result)
		webhookResults = append(webhookResults, webhookResult)
	}

	return webhookResults
}