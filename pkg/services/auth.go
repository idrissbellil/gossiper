package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
	"gitea.v3m.net/idriss/gossiper/config"
	"gitea.v3m.net/idriss/gossiper/pkg/context"
	"gitea.v3m.net/idriss/gossiper/pkg/models"
	"gitea.v3m.net/idriss/gossiper/pkg/session"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	// authSessionName stores the name of the session which contains authentication data
	authSessionName = "ua"

	// authSessionKeyUserID stores the key used to store the user ID in the session
	authSessionKeyUserID = "user_id"

	// authSessionKeyAuthenticated stores the key used to store the authentication status in the session
	authSessionKeyAuthenticated = "authenticated"
)

// NotAuthenticatedError is an error returned when a user is not authenticated
type NotAuthenticatedError struct{}

// Error implements the error interface.
func (e NotAuthenticatedError) Error() string {
	return "user not authenticated"
}

// InvalidPasswordTokenError is an error returned when an invalid token is provided
type InvalidPasswordTokenError struct{}

// Error implements the error interface.
func (e InvalidPasswordTokenError) Error() string {
	return "invalid password token"
}

// AuthClient is the client that handles authentication requests
type AuthClient struct {
	config *config.Config
	orm    *models.DB
}

// NewAuthClient creates a new authentication client
func NewAuthClient(cfg *config.Config, orm *models.DB) *AuthClient {
	return &AuthClient{
		config: cfg,
		orm:    orm,
	}
}

// Login logs in a user of a given ID
func (c *AuthClient) Login(ctx echo.Context, userID int) error {
	sess, err := session.Get(ctx, authSessionName)
	if err != nil {
		return err
	}
	sess.Values[authSessionKeyUserID] = userID
	sess.Values[authSessionKeyAuthenticated] = true
	return sess.Save(ctx.Request(), ctx.Response())
}

// Logout logs the requesting user out
func (c *AuthClient) Logout(ctx echo.Context) error {
	sess, err := session.Get(ctx, authSessionName)
	if err != nil {
		return err
	}
	sess.Values[authSessionKeyAuthenticated] = false
	return sess.Save(ctx.Request(), ctx.Response())
}

// GetAuthenticatedUserID returns the authenticated user's ID, if the user is logged in
func (c *AuthClient) GetAuthenticatedUserID(ctx echo.Context) (int, error) {
	sess, err := session.Get(ctx, authSessionName)
	if err != nil {
		return 0, err
	}

	if sess.Values[authSessionKeyAuthenticated] == true {
		return sess.Values[authSessionKeyUserID].(int), nil
	}

	return 0, NotAuthenticatedError{}
}

// GetAuthenticatedUser returns the authenticated user if the user is logged in
func (c *AuthClient) GetAuthenticatedUser(ctx echo.Context) (*models.User, error) {
	if userID, err := c.GetAuthenticatedUserID(ctx); err == nil {
		var user models.User
		result := c.orm.WithContext(ctx.Request().Context()).First(&user, userID)
		if result.Error != nil {
			return nil, result.Error
		}
		return &user, nil
	}

	return nil, NotAuthenticatedError{}
}

// HashPassword returns a hash of a given password
func (c *AuthClient) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword check if a given password matches a given hash
func (c *AuthClient) CheckPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GeneratePasswordResetToken generates a password reset token for a given user.
// For security purposes, the token itself is not stored in the database but rather
// a hash of the token, exactly how passwords are handled. This method returns both
// the generated token as well as the token entity which only contains the hash.
func (c *AuthClient) GeneratePasswordResetToken(ctx echo.Context, userID int) (string, *models.PasswordToken, error) {
	// Generate the token, which is what will go in the URL, but not the database
	token, err := c.RandomToken(c.config.App.PasswordToken.Length)
	if err != nil {
		return "", nil, err
	}

	// Hash the token, which is what will be stored in the database
	hash, err := c.HashPassword(token)
	if err != nil {
		return "", nil, err
	}

	// Create and save the password reset token
	pt := &models.PasswordToken{
		Hash:   hash,
		UserID: userID,
	}
	result := c.orm.WithContext(ctx.Request().Context()).Create(pt)

	return token, pt, result.Error
}

// GetValidPasswordToken returns a valid, non-expired password token entity for a given user, token ID and token.
// Since the actual token is not stored in the database for security purposes, if a matching password token entity is
// found a hash of the provided token is compared with the hash stored in the database in order to validate.
func (c *AuthClient) GetValidPasswordToken(ctx echo.Context, userID, tokenID int, token string) (*models.PasswordToken, error) {
	// Ensure expired tokens are never returned
	expiration := time.Now().Add(-c.config.App.PasswordToken.Expiration)

	// Query to find a password token entity that matches the given user and token ID
	var pt models.PasswordToken
	result := c.orm.WithContext(ctx.Request().Context()).
		Where("id = ? AND user_id = ? AND created_at >= ?", tokenID, userID, expiration).
		First(&pt)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, InvalidPasswordTokenError{}
		}
		if !context.IsCanceledError(result.Error) {
			return nil, result.Error
		}
		return nil, InvalidPasswordTokenError{}
	}

	// Check the token for a hash match
	if err := c.CheckPassword(token, pt.Hash); err == nil {
		return &pt, nil
	}

	return nil, InvalidPasswordTokenError{}
}

// DeletePasswordTokens deletes all password tokens in the database for a belonging to a given user.
// This should be called after a successful password reset.
func (c *AuthClient) DeletePasswordTokens(ctx echo.Context, userID int) error {
	result := c.orm.WithContext(ctx.Request().Context()).
		Where("user_id = ?", userID).
		Delete(&models.PasswordToken{})

	return result.Error
}

// RandomToken generates a random token string of a given length
func (c *AuthClient) RandomToken(length int) (string, error) {
	b := make([]byte, (length/2)+1)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	return token[:length], nil
}

// GenerateEmailVerificationToken generates an email verification token for a given email address using JWT which
// is set to expire based on the duration stored in configuration
func (c *AuthClient) GenerateEmailVerificationToken(email string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(c.config.App.EmailVerificationTokenExpiration).Unix(),
	})

	return token.SignedString([]byte(c.config.App.EncryptionKey))
}

// ValidateEmailVerificationToken validates an email verification token and returns the associated email address if
// the token is valid and has not expired
func (c *AuthClient) ValidateEmailVerificationToken(token string) (string, error) {
	t, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		return []byte(c.config.App.EncryptionKey), nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := t.Claims.(jwt.MapClaims); ok && t.Valid {
		return claims["email"].(string), nil
	}

	return "", errors.New("invalid or expired token")
}
