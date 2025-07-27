package handlers

import (
	"fmt"
	"math/rand"

	"gitea.v3m.net/idriss/gossiper/pkg/middleware"
	"gitea.v3m.net/idriss/gossiper/pkg/page"
	"gitea.v3m.net/idriss/gossiper/pkg/services"
	"gitea.v3m.net/idriss/gossiper/templates"
	"github.com/labstack/echo/v4"
)

const routeNameSearch = "search"

type (
	Search struct {
		*services.TemplateRenderer
	}

	searchResult struct {
		Title string
		URL   string
	}
)

func init() {
	Register(new(Search))
}

func (h *Search) Init(c *services.Container) error {
	h.TemplateRenderer = c.TemplateRenderer
	return nil
}

func (h *Search) Routes(g *echo.Group) {
	g.GET("/search", h.Page, middleware.RequireAuthentication()).Name = routeNameSearch
}

func (h *Search) Page(ctx echo.Context) error {
	p := page.New(ctx)
	p.Layout = templates.LayoutMain
	p.Name = templates.PageSearch

	// Fake search results
	var results []searchResult
	if search := ctx.QueryParam("query"); search != "" {
		for i := 0; i < 5; i++ {
			title := "Lorem ipsum example ddolor sit amet"
			index := rand.Intn(len(title))
			title = title[:index] + search + title[index:]
			results = append(results, searchResult{
				Title: title,
				URL:   fmt.Sprintf("https://www.%s.com", search),
			})
		}
	}
	p.Data = results

	return h.RenderPage(ctx, p)
}
