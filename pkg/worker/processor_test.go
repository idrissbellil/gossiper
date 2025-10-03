package worker

import (
	"context"
	"errors"
	"testing"

	"gitea.v3m.net/idriss/gossiper/pkg/models"
)

type mockJobRepository struct {
	jobs map[string][]*models.Job
	err  error
}

func (m *mockJobRepository) GetActiveJobs(ctx context.Context, email string) ([]*models.Job, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.jobs[email], nil
}

type mockLogger struct {
	messages []string
}

func (m *mockLogger) Printf(format string, args ...interface{}) {
	m.messages = append(m.messages, format)
}

func (m *mockLogger) Println(args ...interface{}) {
	m.messages = append(m.messages, "println")
}

type mockMessageFetcher struct {
	messages map[string]*MailcrabMessage
	err      error
}

func (m *mockMessageFetcher) FetchMessage(messageID string) (*MailcrabMessage, error) {
	if m.err != nil {
		return nil, m.err
	}
	if msg, ok := m.messages[messageID]; ok {
		return msg, nil
	}
	return nil, errors.New("message not found")
}

func (m *mockMessageFetcher) GetMessageBody(msg *MailcrabMessage) string {
	if msg.Text != "" {
		return msg.Text
	}
	return msg.HTML
}

func TestMessageProcessor_ParseRawMessage(t *testing.T) {
	tests := []struct {
		name         string
		rawMsg       RawMessage
		fetcherMsgs  map[string]*MailcrabMessage
		fetcherErr   error
		expected     []Message
	}{
		{
			name: "single recipient with text body",
			rawMsg: RawMessage{
				ID:      "msg-123",
				Subject: "Test Subject",
				From:    EmailAddress{Name: "Sender", Email: "sender@example.com"},
				To:      []EmailAddress{{Name: "User", Email: "user@example.com"}},
			},
			fetcherMsgs: map[string]*MailcrabMessage{
				"msg-123": {
					ID:      "msg-123",
					Text:    "Test body",
					HTML:    "",
					Subject: "Test Subject",
					From:    EmailAddress{Name: "Sender", Email: "sender@example.com"},
					To:      []EmailAddress{{Name: "User", Email: "user@example.com"}},
				},
			},
			expected: []Message{
				{
					To:      "user@example.com",
					From:    "sender@example.com",
					Subject: "Test Subject",
					Body:    "Test body",
				},
			},
		},
		{
			name: "multiple recipients",
			rawMsg: RawMessage{
				ID:      "msg-456",
				Subject: "Test Subject",
				From:    EmailAddress{Name: "Sender", Email: "sender@example.com"},
				To: []EmailAddress{
					{Name: "User1", Email: "user1@example.com"},
					{Name: "User2", Email: "user2@example.com"},
				},
			},
			fetcherMsgs: map[string]*MailcrabMessage{
				"msg-456": {
					ID:      "msg-456",
					Text:    "Test body",
					HTML:    "",
					Subject: "Test Subject",
					From:    EmailAddress{Name: "Sender", Email: "sender@example.com"},
				},
			},
			expected: []Message{
				{
					To:      "user1@example.com",
					From:    "sender@example.com",
					Subject: "Test Subject",
					Body:    "Test body",
				},
				{
					To:      "user2@example.com",
					From:    "sender@example.com",
					Subject: "Test Subject",
					Body:    "Test body",
				},
			},
		},
		{
			name: "fetch error",
			rawMsg: RawMessage{
				ID:      "msg-error",
				Subject: "Test Subject",
				From:    EmailAddress{Name: "Sender", Email: "sender@example.com"},
				To:      []EmailAddress{{Name: "User", Email: "user@example.com"}},
			},
			fetcherErr: errors.New("fetch failed"),
			expected:   []Message{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &mockMessageFetcher{
				messages: tt.fetcherMsgs,
				err:      tt.fetcherErr,
			}

			logger := &mockLogger{}
			processor := &MessageProcessor{
				jobRepo: nil,
				logger:  logger,
				fetcher: mockFetcher,
			}

			result := processor.ParseRawMessage(tt.rawMsg)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d messages, got %d", len(tt.expected), len(result))
				return
			}

			for i, msg := range result {
				if msg.To != tt.expected[i].To ||
					msg.From != tt.expected[i].From ||
					msg.Subject != tt.expected[i].Subject ||
					msg.Body != tt.expected[i].Body {
					t.Errorf("message %d: expected %+v, got %+v", i, tt.expected[i], msg)
				}
			}
		})
	}
}

func TestMessageProcessor_ProcessMessage(t *testing.T) {
	method := "POST"

	tests := []struct {
		name             string
		message          Message
		jobs             []*models.Job
		repoErr          error
		expectedResults  int
		expectedJobID    int
		expectedPayload  string
		expectError      bool
	}{
		{
			name: "matching job with template",
			message: Message{
				To:      "test@example.com",
				From:    "sender@example.com",
				Subject: "Test",
				Body:    "Hello",
			},
			jobs: []*models.Job{
				{
					ID:              1,
					Email:           "test@example.com",
					FromRegex:       "sender@.*",
					URL:             "http://example.com/webhook",
					Method:          method,
					PayloadTemplate: "Subject: {{.Subject}}, Body: {{.Body}}",
					Headers:         map[string]string{"X-Custom": "value"},
				},
			},
			expectedResults: 1,
			expectedJobID:   1,
			expectedPayload: "Subject: Test, Body: Hello",
		},
		{
			name: "matching job with JSON payload",
			message: Message{
				To:      "test@example.com",
				From:    "sender@example.com",
				Subject: "Test",
				Body:    "Hello",
			},
			jobs: []*models.Job{
				{
					ID:              2,
					Email:           "test@example.com",
					FromRegex:       "sender@.*",
					URL:             "http://example.com/webhook",
					Method:          method,
					PayloadTemplate: "",
					Headers:         map[string]string{},
				},
			},
			expectedResults: 1,
			expectedJobID:   2,
			expectedPayload: `{"From":"sender@example.com","To":"test@example.com","Subject":"Test","Body":"Hello"}`,
		},
		{
			name: "non-matching from regex",
			message: Message{
				To:   "test@example.com",
				From: "other@example.com",
			},
			jobs: []*models.Job{
				{
					ID:        3,
					Email:     "test@example.com",
					FromRegex: "sender@.*",
					URL:       "http://example.com/webhook",
					Method:    method,
				},
			},
			expectedResults: 0,
		},
		{
			name:            "repository error",
			message:         Message{To: "test@example.com"},
			repoErr:         errors.New("database error"),
			expectError:     true,
			expectedResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockJobRepository{
				jobs: map[string][]*models.Job{
					tt.message.To: tt.jobs,
				},
				err: tt.repoErr,
			}

			logger := &mockLogger{}
			processor := &MessageProcessor{
				jobRepo: mockRepo,
				logger:  logger,
				fetcher: nil, // Not needed for ProcessMessage tests
			}

			results, err := processor.ProcessMessage(context.Background(), tt.message)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(results) != tt.expectedResults {
				t.Errorf("expected %d results, got %d", tt.expectedResults, len(results))
				return
			}

			if tt.expectedResults > 0 {
				result := results[0]
				if result.JobID != tt.expectedJobID {
					t.Errorf("expected job ID %d, got %d", tt.expectedJobID, result.JobID)
				}
				if result.Payload != tt.expectedPayload {
					t.Errorf("expected payload %q, got %q", tt.expectedPayload, result.Payload)
				}
			}
		})
	}
}

func TestMessageProcessor_generatePayload(t *testing.T) {
	processor := &MessageProcessor{
		jobRepo: nil,
		logger:  &mockLogger{},
		fetcher: nil,
	}

	msg := Message{
		From:    "sender@example.com",
		To:      "test@example.com",
		Subject: "Test Subject",
		Body:    "Test Body",
	}

	t.Run("template payload", func(t *testing.T) {
		method := "POST"
		job := &models.Job{
			ID:              1,
			PayloadTemplate: "From: {{.From}}, Subject: {{.Subject}}",
			Method:          method,
		}

		payload, err := processor.generatePayload(job, msg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := "From: sender@example.com, Subject: Test Subject"
		if payload != expected {
			t.Errorf("expected %q, got %q", expected, payload)
		}
	})

	t.Run("JSON payload", func(t *testing.T) {
		method := "POST"
		job := &models.Job{
			ID:              2,
			PayloadTemplate: "",
			Method:          method,
		}

		payload, err := processor.generatePayload(job, msg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := `{"From":"sender@example.com","To":"test@example.com","Subject":"Test Subject","Body":"Test Body"}`
		if payload != expected {
			t.Errorf("expected %q, got %q", expected, payload)
		}
	})

	t.Run("invalid template", func(t *testing.T) {
		method := "POST"
		job := &models.Job{
			ID:              3,
			PayloadTemplate: "{{.Invalid",
			Method:          method,
		}

		_, err := processor.generatePayload(job, msg)
		if err == nil {
			t.Error("expected error for invalid template")
		}
	})
}
