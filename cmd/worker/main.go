package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitea.v3m.net/idriss/gossiper/pkg/services"
	"gitea.v3m.net/idriss/gossiper/pkg/worker"
)

type StdLogger struct{}

func (s StdLogger) Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (s StdLogger) Println(args ...interface{}) {
	log.Println(args...)
}

func main() {
	c := services.NewContainer()
	defer func() {
		if err := c.Shutdown(); err != nil {
			log.Fatal(err)
		}
	}()

	logger := StdLogger{}
	
	// Create processor (no fetcher needed anymore!)
	jobRepo := worker.NewEntJobRepository(c.ORM)
	processor := worker.NewMessageProcessor(jobRepo, logger, nil, c.Config.SMTP.Hostname)
	
	// Create webhook sender
	config := worker.Config{
		HTTPTimeout:     90 * time.Second,
		MaxRetries:      3,
		ShutdownTimeout: 10 * time.Second,
	}
	
	// Create HTTP client with optional proxy support
	httpClient := &http.Client{Timeout: config.HTTPTimeout}
	
	// Check for proxy configuration
	if proxyURL := os.Getenv("HTTP_PROXY"); proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			log.Fatalf("invalid HTTP_PROXY: %v", err)
		}
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxy),
		}
		log.Printf("using HTTP proxy: %s", proxyURL)
	}
	
	webhookSender := worker.NewWebhookSender(httpClient, logger, config)

	// Create email replier for auto-replies
	emailReplier := worker.NewEmailReplier(
		c.Config.Mail.Hostname,
		int(c.Config.Mail.Port),
		c.Config.Mail.User,
		c.Config.Mail.Password,
		c.Config.Mail.FromAddress,
		logger,
	)

	// Create poller
	poller := worker.NewSMTPMessagePoller(c.ORM, processor, webhookSender, emailReplier, logger, 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("received shutdown signal")
		poller.Shutdown()
		cancel()
	}()

	if err := poller.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("poller error: %v", err)
	}

	log.Println("worker shutdown complete")
}
