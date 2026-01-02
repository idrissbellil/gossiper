package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"

	"gitea.v3m.net/idriss/gossiper/pkg/models"
)

type MessageProcessor struct {
	jobRepo JobRepository
	logger  Logger
	fetcher MessageFetcherInterface
}

func NewMessageProcessor(jobRepo JobRepository, logger Logger, fetcher MessageFetcherInterface) *MessageProcessor {
	return &MessageProcessor{
		jobRepo: jobRepo,
		logger:  logger,
		fetcher: fetcher,
	}
}

type ProcessResult struct {
	JobID   int
	URL     string
	Method  string
	Headers map[string]string
	Payload string
	Error   error
}

func (p *MessageProcessor) ParseRawMessage(rawMsg RawMessage) []Message {
	var messages []Message

	// Fetch full message details from API
	fullMsg, err := p.fetcher.FetchMessage(rawMsg.ID)
	if err != nil {
		p.logger.Printf("failed to fetch message %s: %v", rawMsg.ID, err)
		return messages
	}

	// Get the message body (text or converted HTML)
	body := p.fetcher.GetMessageBody(fullMsg)

	// Create a message for each recipient
	for _, to := range rawMsg.To {
		msg := Message{
			To:      to.Email,
			From:    rawMsg.From.Email,
			Subject: rawMsg.Subject,
			Body:    body,
		}
		messages = append(messages, msg)
	}

	return messages
}

func (p *MessageProcessor) ProcessMessage(ctx context.Context, msg Message) ([]ProcessResult, error) {
	jobs, err := p.jobRepo.GetActiveJobs(ctx, msg.To)
	if err != nil {
		return nil, fmt.Errorf("failed to get active jobs: %w", err)
	}

	var results []ProcessResult

	for _, job := range jobs {
		result := ProcessResult{
			JobID:   job.ID,
			URL:     job.URL,
			Method:  job.Method,
			Headers: job.Headers,
		}

		if !p.matchesFromRegex(job.FromRegex, msg.From) {
			continue
		}

		payload, err := p.generatePayload(job, msg)
		if err != nil {
			result.Error = fmt.Errorf("failed to generate payload: %w", err)
			results = append(results, result)
			continue
		}

		result.Payload = payload
		results = append(results, result)
	}

	return results, nil
}

func (p *MessageProcessor) matchesFromRegex(pattern, from string) bool {
	matched, err := regexp.MatchString(pattern, from)
	if err != nil {
		p.logger.Printf("invalid from_regex pattern '%s': %v", pattern, err)
		return false
	}
	return matched
}

func (p *MessageProcessor) generatePayload(job *models.Job, msg Message) (string, error) {
	if job.PayloadTemplate != "" {
		return p.executeTemplate(job.PayloadTemplate, msg)
	}

	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func (p *MessageProcessor) executeTemplate(templateStr string, msg Message) (string, error) {
	tmpl, err := template.New("payload").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, msg)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

