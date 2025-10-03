package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"gitea.v3m.net/idriss/gossiper/pkg/context"
	"gitea.v3m.net/idriss/gossiper/pkg/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// LoadUser loads the user based on the ID provided as a path parameter
func LoadUser(orm *models.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, err := strconv.Atoi(c.Param("user"))
			if err != nil {
				return echo.NewHTTPError(http.StatusNotFound)
			}

			var u models.User
			result := orm.WithContext(c.Request().Context()).First(&u, userID)

			if result.Error != nil {
				if errors.Is(result.Error, gorm.ErrRecordNotFound) {
					return echo.NewHTTPError(http.StatusNotFound)
				}
				return echo.NewHTTPError(
					http.StatusInternalServerError,
					fmt.Sprintf("error querying user: %v", result.Error),
				)
			}

			c.Set(context.UserKey, &u)
			return next(c)
		}
	}
}
