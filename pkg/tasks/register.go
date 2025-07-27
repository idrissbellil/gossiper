package tasks

import (
	"gitea.v3m.net/idriss/gossiper/pkg/services"
)

// Register registers all task queues with the task client
func Register(c *services.Container) {
	c.Tasks.Register(NewExampleTaskQueue(c))
}
