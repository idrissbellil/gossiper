package worker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown"
)

type MessageFetcher struct {
	httpClient HTTPClient
	apiURL     string
	logger     Logger
}

func NewMessageFetcher(httpClient HTTPClient, apiURL string, logger Logger) *MessageFetcher {
	return &MessageFetcher{
		httpClient: httpClient,
		apiURL:     apiURL,
		logger:     logger,
	}
}

func (f *MessageFetcher) FetchMessage(messageID string) (*MailcrabMessage, error) {
	url := fmt.Sprintf("%s/message/%s", f.apiURL, messageID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var msg MailcrabMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

func (f *MessageFetcher) GetMessageBody(msg *MailcrabMessage) string {
	// Prefer text version if available
	if msg.Text != "" {
		return msg.Text
	}

	// Convert HTML to markdown if text is not available
	if msg.HTML != "" {
		converter := md.NewConverter("", true, nil)
		markdown, err := converter.ConvertString(msg.HTML)
		if err != nil {
			f.logger.Printf("failed to convert HTML to markdown: %v", err)
			// Return HTML as fallback, stripping basic tags
			return stripBasicHTML(msg.HTML)
		}
		return markdown
	}

	return ""
}

// stripBasicHTML removes basic HTML tags as a fallback
func stripBasicHTML(html string) string {
	// Simple tag stripper - replaces common tags
	html = strings.ReplaceAll(html, "<br>", "\n")
	html = strings.ReplaceAll(html, "<br/>", "\n")
	html = strings.ReplaceAll(html, "<br />", "\n")
	html = strings.ReplaceAll(html, "</p>", "\n\n")

	// Remove all remaining HTML tags (basic approach)
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	return strings.TrimSpace(result.String())
}
