package tasks

import (
	"gitea.risky.info/risky-info/gossiper/pkg/services"
)

// Register registers all task queues with the task client
func Register(c *services.Container) {
	c.Tasks.Register(NewExampleTaskQueue(c))
}
