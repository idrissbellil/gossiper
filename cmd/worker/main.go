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

	config := worker.Config{
		WebSocketURL:    c.Config.Mailcrab.Ws,
		APIURL:          c.Config.Mailcrab.Api,
		AllowedHostname: c.Config.Mailcrab.Hostname,
		HTTPTimeout:     30 * time.Second,
		MaxRetries:      3,
		BufferSize:      100,
		ShutdownTimeout: 10 * time.Second,
	}

	deps := worker.WorkerDependencies{
		JobRepo:    worker.NewEntJobRepository(c.ORM),
		Logger:     StdLogger{},
		Config:     config,
		WSDialer:   worker.DefaultWebSocketDialer{},
		HTTPClient: &http.Client{Timeout: config.HTTPTimeout},
	}

	w := worker.NewWorker(deps)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("received shutdown signal")
		w.Shutdown()
		cancel()
	}()

	if err := w.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("worker error: %v", err)
	}

	w.WaitForShutdown()
	log.Println("worker shutdown complete")
}
