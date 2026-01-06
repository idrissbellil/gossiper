package smtp

import (
	"fmt"
	"io"
	"strings"
	"time"

	"gitea.v3m.net/idriss/gossiper/pkg/models"
	"github.com/emersion/go-smtp"
)

// Backend implements SMTP backend
type Backend struct {
	db              *models.DB
	allowedHostname string
	logger          Logger
}

// Logger interface for logging
type Logger interface {
	Printf(format string, args ...interface{})
	Println(args ...interface{})
}

// NewBackend creates a new SMTP backend
func NewBackend(db *models.DB, allowedHostname string, logger Logger) *Backend {
	return &Backend{
		db:              db,
		allowedHostname: allowedHostname,
		logger:          logger,
	}
}

// NewSession creates a new SMTP session
func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		backend: b,
		from:    "",
		to:      []string{},
	}, nil
}

// Session represents an SMTP session
type Session struct {
	backend *Backend
	from    string
	to      []string
}

// AuthPlain implements PLAIN authentication (we accept everything)
func (s *Session) AuthPlain(username, password string) error {
	return nil
}

// Mail is called when the client sends MAIL FROM
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

// Rcpt is called when the client sends RCPT TO
// This is where we filter by hostname - reject spam immediately!
func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	// Extract email from angle brackets if present (e.g., "<user@domain.com>")
	email := strings.Trim(to, "<>")
	
	// Check if email ends with our allowed hostname
	suffix := "@" + s.backend.allowedHostname
	if !strings.HasSuffix(email, suffix) {
		s.backend.logger.Printf("SMTP: rejected recipient %s (not @%s)", email, s.backend.allowedHostname)
		return &smtp.SMTPError{
			Code:    550,
			Message: "No such user here",
		}
	}
	
	s.to = append(s.to, email)
	s.backend.logger.Printf("SMTP: accepted recipient %s", email)
	return nil
}

// Data is called when the client sends the message body
func (s *Session) Data(r io.Reader) error {
	// Read the entire message
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	
	// Parse the message to extract subject
	subject := extractSubject(string(body))
	messageBody := extractBody(string(body))
	
	// Store each recipient as a separate message
	for _, recipient := range s.to {
		msg := &models.SMTPMessage{
			To:        recipient,
			From:      s.from,
			Subject:   subject,
			Body:      messageBody,
			Processed: false,
		}
		
		if err := s.backend.db.Create(msg).Error; err != nil {
			s.backend.logger.Printf("SMTP: failed to store message for %s: %v", recipient, err)
			return err
		}
		
		s.backend.logger.Printf("SMTP: stored message from %s to %s (subject: %s)", s.from, recipient, subject)
	}
	
	return nil
}

// Reset resets the session state
func (s *Session) Reset() {
	s.from = ""
	s.to = []string{}
}

// Logout is called when the session is closed
func (s *Session) Logout() error {
	return nil
}

// extractSubject extracts the subject from email headers
func extractSubject(message string) string {
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "subject:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
		}
	}
	return "(no subject)"
}

// extractBody extracts the body from the email message
func extractBody(message string) string {
	// Find the blank line that separates headers from body
	parts := strings.SplitN(message, "\n\n", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	
	// Alternative: try \r\n\r\n
	parts = strings.SplitN(message, "\r\n\r\n", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	
	return message
}

// StartServer starts the SMTP server
func StartServer(addr string, backend *Backend) error {
	s := smtp.NewServer(backend)
	
	s.Addr = addr
	s.Domain = backend.allowedHostname
	s.ReadTimeout = 30 * time.Second
	s.WriteTimeout = 30 * time.Second
	s.MaxMessageBytes = 10 * 1024 * 1024 // 10MB max
	s.MaxRecipients = 50
	s.AllowInsecureAuth = true // We're a catchall, we accept everything
	
	backend.logger.Printf("SMTP server starting on %s (domain: %s)", addr, s.Domain)
	
	if err := s.ListenAndServe(); err != nil {
		return fmt.Errorf("SMTP server error: %w", err)
	}
	
	return nil
}
