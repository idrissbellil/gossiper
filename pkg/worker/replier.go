package worker

import (
	"fmt"
	"net/smtp"
)

// EmailReplier sends auto-reply emails
type EmailReplier struct {
	smtpHost     string
	smtpPort     int
	smtpUser     string
	smtpPassword string
	fromAddress  string
	logger       Logger
}

// NewEmailReplier creates a new email replier
func NewEmailReplier(host string, port int, user, password, fromAddress string, logger Logger) *EmailReplier {
	return &EmailReplier{
		smtpHost:     host,
		smtpPort:     port,
		smtpUser:     user,
		smtpPassword: password,
		fromAddress:  fromAddress,
		logger:       logger,
	}
}

// SendReply sends an auto-reply email
func (e *EmailReplier) SendReply(to, subject, body string) error {
	if body == "" {
		return nil // No reply configured
	}

	// Build email message
	message := fmt.Sprintf("From: %s\r\n", e.fromAddress)
	message += fmt.Sprintf("To: %s\r\n", to)
	message += fmt.Sprintf("Subject: Re: %s\r\n", subject)
	message += "\r\n"
	message += body

	// SMTP auth
	auth := smtp.PlainAuth("", e.smtpUser, e.smtpPassword, e.smtpHost)

	// Send email
	addr := fmt.Sprintf("%s:%d", e.smtpHost, e.smtpPort)
	err := smtp.SendMail(addr, auth, e.fromAddress, []string{to}, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send reply email: %w", err)
	}

	e.logger.Printf("sent auto-reply to %s", to)
	return nil
}
