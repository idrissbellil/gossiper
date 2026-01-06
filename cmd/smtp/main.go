package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"gitea.v3m.net/idriss/gossiper/pkg/services"
	"gitea.v3m.net/idriss/gossiper/pkg/smtp"
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

	backend := smtp.NewBackend(c.ORM, c.Config.SMTP.Hostname, StdLogger{})

	// Start SMTP server in a goroutine
	go func() {
		if err := smtp.StartServer(":25", backend); err != nil {
			log.Fatalf("SMTP server failed: %v", err)
		}
	}()

	log.Println("SMTP server started on :25")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("SMTP server shutting down")
}
