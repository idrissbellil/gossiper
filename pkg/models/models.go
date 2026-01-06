package models

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID        int       `gorm:"primaryKey"`
	Name      string    `gorm:"not null"`
	Email     string    `gorm:"uniqueIndex;not null"`
	Password  string    `gorm:"not null"`
	Verified  bool      `gorm:"default:false"`
	Credit    float64   `gorm:"default:0"`
	CreatedAt time.Time `gorm:"not null"`

	// Relations
	PasswordTokens []PasswordToken `gorm:"foreignKey:UserID"`
	Jobs           []Job           `gorm:"foreignKey:UserID"`
}

// BeforeSave is a GORM hook that normalizes email to lowercase
func (u *User) BeforeSave(tx *gorm.DB) error {
	u.Email = strings.ToLower(u.Email)
	return nil
}

// BeforeCreate is a GORM hook that sets the created_at timestamp
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	return nil
}

// PasswordToken represents a password reset token
type PasswordToken struct {
	ID        int       `gorm:"primaryKey"`
	Hash      string    `gorm:"not null"`
	UserID    int       `gorm:"not null;index"`
	CreatedAt time.Time `gorm:"not null"`

	// Relations
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

// BeforeCreate is a GORM hook that sets the created_at timestamp
func (pt *PasswordToken) BeforeCreate(tx *gorm.DB) error {
	if pt.CreatedAt.IsZero() {
		pt.CreatedAt = time.Now()
	}
	return nil
}

// Job represents a webhook job
type Job struct {
	ID              int               `gorm:"primaryKey"`
	Email           string            `gorm:"uniqueIndex;not null"` // Email address to watch
	FromRegex       string            `gorm:"default:'.*'"`
	URL             string            `gorm:"not null"` // Webhook URL
	Method          string            `gorm:"default:'GET'"`
	Headers         map[string]string `gorm:"serializer:json"`
	PayloadTemplate string            `gorm:"type:text"`
	IsActive        bool              `gorm:"default:true"`
	UserID          int               `gorm:"not null;index"`
	CreatedAt       time.Time         `gorm:"not null"`

	// Relations
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

// BeforeCreate is a GORM hook that sets the created_at timestamp
func (j *Job) BeforeCreate(tx *gorm.DB) error {
	if j.CreatedAt.IsZero() {
		j.CreatedAt = time.Now()
	}
	return nil
}

// SMTPMessage represents an incoming SMTP message waiting to be processed
type SMTPMessage struct {
	ID        int       `gorm:"primaryKey"`
	To        string    `gorm:"not null;index"` // Recipient email (already filtered for valid hostname)
	From      string    `gorm:"not null"`
	Subject   string    `gorm:"not null"`
	Body      string    `gorm:"type:text;not null"`
	Processed bool      `gorm:"default:false;index"`
	CreatedAt time.Time `gorm:"not null;index"`
}

// BeforeCreate is a GORM hook that sets the created_at timestamp
func (sm *SMTPMessage) BeforeCreate(tx *gorm.DB) error {
	if sm.CreatedAt.IsZero() {
		sm.CreatedAt = time.Now()
	}
	return nil
}

// DB wraps gorm.DB with additional helper methods
type DB struct {
	*gorm.DB
}

// NewDB creates a new DB instance
func NewDB(db *gorm.DB) *DB {
	return &DB{DB: db}
}

// WithContext returns a new DB instance with the given context
func (db *DB) WithContext(ctx context.Context) *DB {
	return &DB{DB: db.DB.WithContext(ctx)}
}

// AutoMigrate runs auto migration for all models
func (db *DB) AutoMigrate() error {
	return db.DB.AutoMigrate(
		&User{},
		&PasswordToken{},
		&Job{},
		&SMTPMessage{},
	)
}
