package services

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"gitea.v3m.net/idriss/gossiper/config"
	"gitea.v3m.net/idriss/gossiper/pkg/funcmap"
	"gitea.v3m.net/idriss/gossiper/pkg/models"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Container contains all services used by the application and provides an easy way to handle dependency
// injection including within tests
type Container struct {
	// Validator stores a validator
	Validator *Validator

	// Web stores the web framework
	Web *echo.Echo

	// Config stores the application configuration
	Config *config.Config

	// Cache contains the cache client
	Cache *CacheClient

	// Database stores the connection to the database
	Database      *sql.DB
	QueueDatabase *sql.DB

	// ORM stores a client to the ORM
	ORM *models.DB

	// Mail stores an email sending client
	Mail *MailClient

	// Auth stores an authentication client
	Auth *AuthClient

	// TemplateRenderer stores a service to easily render and cache templates
	TemplateRenderer *TemplateRenderer

	// Tasks stores the task client
	Tasks *TaskClient
}

// NewContainer creates and initializes a new Container
func NewContainer() *Container {
	c := new(Container)
	c.initConfig()
	c.initValidator()
	c.initWeb()
	c.initCache()
	c.initDatabase()
	c.initQueueDatabase()
	c.initORM()
	c.initAuth()
	c.initTemplateRenderer()
	c.initMail()
	c.initTasks()
	return c
}

// Shutdown shuts the Container down and disconnects all connections.
// If the task runner was started, cancel the context to shut it down prior to calling this.
func (c *Container) Shutdown() error {
	// Get underlying *sql.DB from GORM to close it
	sqlDB, err := c.ORM.DB.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}
	if err := c.Database.Close(); err != nil {
		return err
	}
	c.Cache.Close()

	return nil
}

// initConfig initializes configuration
func (c *Container) initConfig() {
	cfg, err := config.GetConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	c.Config = &cfg

	// Configure logging
	switch cfg.App.Environment {
	case config.EnvProduction:
		slog.SetLogLoggerLevel(slog.LevelInfo)
	default:
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
}

// initValidator initializes the validator
func (c *Container) initValidator() {
	c.Validator = NewValidator()
}

// initWeb initializes the web framework
func (c *Container) initWeb() {
	c.Web = echo.New()
	c.Web.HideBanner = true
	c.Web.Validator = c.Validator
}

// initCache initializes the cache
func (c *Container) initCache() {
	store, err := newInMemoryCache(c.Config.Cache.Capacity)
	if err != nil {
		panic(err)
	}

	c.Cache = NewCacheClient(store)
}

// initDatabase initializes the database
func (c *Container) initDatabase() {
	var err error
	var connection string

	switch c.Config.App.Environment {
	case config.EnvTest:
		// TODO: Drop/recreate the DB, if this isn't in memory?
		connection = c.Config.Database.TestConnection
	default:
		connection = c.Config.Database.Connection
	}

	c.Database, err = openDB(c.Config.Database.Driver, connection)
	if err != nil {
		panic(err)
	}
}

func (c *Container) initQueueDatabase() {
	var err error
	var connection string

	switch c.Config.App.Environment {
	case config.EnvTest:
		// TODO: Drop/recreate the DB, if this isn't in memory?
		connection = c.Config.Database.TestConnection
	default:
		connection = c.Config.Database.Connection
	}

	c.QueueDatabase, err = openDB(c.Config.QueueDatabase.Driver, connection)
	if err != nil {
		panic(err)
	}
}

// initORM initializes the ORM
func (c *Container) initORM() {
	var dialector gorm.Dialector

	switch c.Config.Database.Driver {
	case "postgres":
		dialector = postgres.Open(c.Config.Database.Connection)
	case "sqlite3":
		dialector = sqlite.Open(c.Config.Database.Connection)
	default:
		panic(fmt.Sprintf("unsupported database driver: %s", c.Config.Database.Driver))
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		panic(err)
	}

	c.ORM = models.NewDB(db)

	// Run the auto migration tool.
	if err := c.ORM.AutoMigrate(); err != nil {
		panic(err)
	}
}

// initAuth initializes the authentication client
func (c *Container) initAuth() {
	c.Auth = NewAuthClient(c.Config, c.ORM)
}

// initTemplateRenderer initializes the template renderer
func (c *Container) initTemplateRenderer() {
	c.TemplateRenderer = NewTemplateRenderer(c.Config, c.Cache, funcmap.NewFuncMap(c.Web))
}

// initMail initialize the mail client
func (c *Container) initMail() {
	var err error
	c.Mail, err = NewMailClient(c.Config, c.TemplateRenderer)
	if err != nil {
		panic(fmt.Sprintf("failed to create mail client: %v", err))
	}
}

// initTasks initializes the task client
func (c *Container) initTasks() {
	var err error
	// You could use a separate database for tasks, if you'd like. but using one
	// makes transaction support easier
	c.Tasks, err = NewTaskClient(c.Config.Tasks, c.QueueDatabase)
	if err != nil {
		panic(fmt.Sprintf("failed to create task client: %v", err))
	}
}

// openDB opens a database connection
func openDB(driver, connection string) (*sql.DB, error) {
	// Helper to automatically create the directories that the specified sqlite file
	// should reside in, if one
	if driver == "sqlite3" {
		d := strings.Split(connection, "/")

		if len(d) > 1 {
			path := strings.Join(d[:len(d)-1], "/")

			if err := os.MkdirAll(path, 0755); err != nil {
				return nil, err
			}
		}
	}

	return sql.Open(driver, connection)
}
