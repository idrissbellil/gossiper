package worker

import (
	"context"
	"time"

	"gitea.v3m.net/idriss/gossiper/pkg/models"
)

// SMTPMessagePoller polls for unprocessed SMTP messages
type SMTPMessagePoller struct {
	db              *models.DB
	processor       *MessageProcessor
	webhookSender   *WebhookSender
	emailReplier    *EmailReplier
	logger          Logger
	pollInterval    time.Duration
	batchSize       int
	shutdownChan    chan struct{}
}

// NewSMTPMessagePoller creates a new poller
func NewSMTPMessagePoller(db *models.DB, processor *MessageProcessor, webhookSender *WebhookSender, emailReplier *EmailReplier, logger Logger, pollInterval time.Duration) *SMTPMessagePoller {
	return &SMTPMessagePoller{
		db:            db,
		processor:     processor,
		webhookSender: webhookSender,
		emailReplier:  emailReplier,
		logger:        logger,
		pollInterval:  pollInterval,
		batchSize:     10,
		shutdownChan:  make(chan struct{}),
	}
}

// Start begins polling for messages
func (p *SMTPMessagePoller) Start(ctx context.Context) error {
	p.logger.Println("SMTP message poller starting...")
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Println("poller stopping due to context cancellation")
			return ctx.Err()

		case <-p.shutdownChan:
			p.logger.Println("poller stopping due to shutdown signal")
			return nil

		case <-ticker.C:
			if err := p.pollAndProcess(ctx); err != nil {
				p.logger.Printf("error processing messages: %v", err)
			}
		}
	}
}

// pollAndProcess fetches and processes unprocessed messages
func (p *SMTPMessagePoller) pollAndProcess(ctx context.Context) error {
	var messages []models.SMTPMessage

	// Fetch unprocessed messages
	err := p.db.WithContext(ctx).
		Where("processed = ?", false).
		Order("created_at ASC").
		Limit(p.batchSize).
		Find(&messages).Error

	if err != nil {
		return err
	}

	if len(messages) == 0 {
		return nil
	}

	p.logger.Printf("processing %d messages", len(messages))

	for _, smtpMsg := range messages {
		// Convert to worker.Message format
		msg := Message{
			To:      smtpMsg.To,
			From:    smtpMsg.From,
			Subject: smtpMsg.Subject,
			Body:    smtpMsg.Body,
		}

		// Process the message
		results, err := p.processor.ProcessMessage(ctx, msg)
		if err != nil {
			p.logger.Printf("error processing message ID %d: %v", smtpMsg.ID, err)
			continue
		}

		if len(results) > 0 {
			// Send webhooks
			webhookResults := p.webhookSender.SendWebhooks(ctx, results)
			
			// Check webhook results and send auto-replies for successful ones
			for _, result := range webhookResults {
				if result.Error != nil {
					p.logger.Printf("webhook error for job %d: %v", result.JobID, result.Error)
				} else if result.Response != "" {
					// Webhook succeeded and auto-reply is configured
					err := p.emailReplier.SendReply(smtpMsg.From, smtpMsg.Subject, result.Response)
					if err != nil {
						p.logger.Printf("failed to send auto-reply for job %d: %v", result.JobID, err)
					}
				}
			}
		} else {
			p.logger.Printf("no matching jobs found for message to: %s", msg.To)
		}

		// Mark as processed
		smtpMsg.Processed = true
		if err := p.db.WithContext(ctx).Save(&smtpMsg).Error; err != nil {
			p.logger.Printf("failed to mark message %d as processed: %v", smtpMsg.ID, err)
		}
	}

	return nil
}

// Shutdown signals the poller to stop
func (p *SMTPMessagePoller) Shutdown() {
	close(p.shutdownChan)
}
