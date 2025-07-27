package middleware

import (
	"os"
	"testing"

	"gitea.v3m.net/idriss/gossiper/config"
	"gitea.v3m.net/idriss/gossiper/ent"
	"gitea.v3m.net/idriss/gossiper/pkg/services"
	"gitea.v3m.net/idriss/gossiper/pkg/tests"
)

var (
	c   *services.Container
	usr *ent.User
)

func TestMain(m *testing.M) {
	// Set the environment to test
	config.SwitchEnvironment(config.EnvTest)

	// Create a new container
	c = services.NewContainer()

	// Create a user
	var err error
	if usr, err = tests.CreateUser(c.ORM); err != nil {
		panic(err)
	}

	// Run tests
	exitVal := m.Run()

	// Shutdown the container
	if err = c.Shutdown(); err != nil {
		panic(err)
	}

	os.Exit(exitVal)
}
