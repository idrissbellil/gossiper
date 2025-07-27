package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"gitea.v3m.net/idriss/gossiper/pkg/context"
	"gitea.v3m.net/idriss/gossiper/pkg/log"
	"gitea.v3m.net/idriss/gossiper/pkg/page"
	"gitea.v3m.net/idriss/gossiper/pkg/services"
	"gitea.v3m.net/idriss/gossiper/templates"
)

type Error struct {
	*services.TemplateRenderer
}

func (e *Error) Page(err error, ctx echo.Context) {
	if ctx.Response().Committed || context.IsCanceledError(err) {
		return
	}

	// Determine the error status code
	code := http.StatusInternalServerError
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
	}

	// Log the error
	logger := log.Ctx(ctx)
	switch {
	case code >= 500:
		logger.Error(err.Error())
	case code >= 400:
		logger.Warn(err.Error())
	}

	// Render the error page
	p := page.New(ctx)
	p.Layout = templates.LayoutMain
	p.Name = templates.PageError
	p.Title = http.StatusText(code)
	p.StatusCode = code
	p.HTMX.Request.Enabled = false

	if err = e.RenderPage(ctx, p); err != nil {
		log.Ctx(ctx).Error("failed to render error page",
			"error", err,
		)
	}
}
