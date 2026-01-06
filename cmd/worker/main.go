package main

import (
	"context"
	"log"
	"net/http"
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
		HTTPTimeout:     30 * time.Second,
		MaxRetries:      3,
		ShutdownTimeout: 10 * time.Second,
	}
	httpClient := &http.Client{Timeout: config.HTTPTimeout}
	webhookSender := worker.NewWebhookSender(httpClient, logger, config)

	// Create poller
	poller := worker.NewSMTPMessagePoller(c.ORM, processor, webhookSender, logger, 1*time.Second)

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
