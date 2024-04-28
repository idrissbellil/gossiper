package middleware

import (
	"os"
	"testing"

	"gitea.risky.info/risky-info/gossiper/config"
	"gitea.risky.info/risky-info/gossiper/ent"
	"gitea.risky.info/risky-info/gossiper/pkg/services"
	"gitea.risky.info/risky-info/gossiper/pkg/tests"
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
