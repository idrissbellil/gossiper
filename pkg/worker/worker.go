package worker

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Worker struct {
	wsClient       *WebSocketClient
	processor      *MessageProcessor
	webhookSender  *WebhookSender
	logger         Logger
	config         Config
	shutdownOnce   sync.Once
	shutdownChan   chan struct{}
	processingWg   sync.WaitGroup
}

type WorkerDependencies struct {
	JobRepo   JobRepository
	Logger    Logger
	Config    Config
	WSDialer  WebSocketDialer
	HTTPClient HTTPClient
}

func NewWorker(deps WorkerDependencies) *Worker {
	wsClient := NewWebSocketClient(deps.WSDialer, deps.Logger, deps.Config)
	processor := NewMessageProcessor(deps.JobRepo, deps.Logger)
	webhookSender := NewWebhookSender(deps.HTTPClient, deps.Logger, deps.Config)

	return &Worker{
		wsClient:      wsClient,
		processor:     processor,
		webhookSender: webhookSender,
		logger:        deps.Logger,
		config:        deps.Config,
		shutdownChan:  make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) error {
	w.logger.Println("worker starting...")

	conn, err := w.wsClient.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	defer conn.Close()

	msgChan := make(chan RawMessage, w.config.BufferSize)
	errorChan := make(chan error, 1)

	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go w.wsClient.ReadMessages(workCtx, conn, msgChan, errorChan)

	w.logger.Println("worker started, listening for messages...")

	for {
		select {
		case <-ctx.Done():
			w.logger.Println("worker stopping due to context cancellation")
			w.shutdown()
			return ctx.Err()

		case <-w.shutdownChan:
			w.logger.Println("worker stopping due to shutdown signal")
			return nil

		case err := <-errorChan:
			if err != nil {
				w.logger.Printf("WebSocket error: %v", err)
				return err
			}

		case rawMsg, ok := <-msgChan:
			if !ok {
				w.logger.Println("message channel closed")
				return nil
			}

			messages := w.processor.ParseRawMessage(rawMsg)
			for _, msg := range messages {
				w.processingWg.Add(1)
				go w.processMessageAsync(workCtx, msg)
			}
		}
	}
}

func (w *Worker) processMessageAsync(ctx context.Context, msg Message) {
	defer w.processingWg.Done()

	w.logger.Printf("processing message: %+v", msg)

	results, err := w.processor.ProcessMessage(ctx, msg)
	if err != nil {
		w.logger.Printf("error processing message: %v", err)
		return
	}

	if len(results) == 0 {
		w.logger.Printf("no matching jobs found for message to: %s", msg.To)
		return
	}

	webhookResults := w.webhookSender.SendWebhooks(ctx, results)

	for _, result := range webhookResults {
		if result.Error != nil {
			w.logger.Printf("webhook error for job %d: %v", result.JobID, result.Error)
		}
	}
}

func (w *Worker) Shutdown() {
	w.shutdownOnce.Do(func() {
		w.logger.Println("initiating worker shutdown...")
		close(w.shutdownChan)
	})
}

func (w *Worker) shutdown() {
	w.logger.Println("waiting for in-flight messages to complete...")

	done := make(chan struct{})
	go func() {
		w.processingWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Println("all messages processed, shutdown complete")
	case <-time.After(w.config.ShutdownTimeout):
		w.logger.Printf("shutdown timeout reached after %v, forcing exit", w.config.ShutdownTimeout)
	}
}

func (w *Worker) WaitForShutdown() {
	w.processingWg.Wait()
}