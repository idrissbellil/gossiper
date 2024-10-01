package handlers

import (
	"fmt"
	"html/template"

	"gitea.risky.info/risky-info/gossiper/pkg/middleware"
	"gitea.risky.info/risky-info/gossiper/pkg/page"
	"gitea.risky.info/risky-info/gossiper/pkg/services"
	"gitea.risky.info/risky-info/gossiper/templates"
	"github.com/labstack/echo/v4"
)

const (
	routeNameAbout = "about"
	routeNameHome  = "home"
)

type (
	Pages struct {
		*services.TemplateRenderer
	}

	post struct {
		Title string
		Body  string
	}

	aboutData struct {
		ShowCacheWarning bool
		FrontendTabs     []aboutTab
		BackendTabs      []aboutTab
	}

	aboutTab struct {
		Title string
		Body  template.HTML
	}
)

func init() {
	Register(new(Pages))
}

func (h *Pages) Init(c *services.Container) error {
	h.TemplateRenderer = c.TemplateRenderer
	return nil
}

func (h *Pages) Routes(g *echo.Group) {
	g.GET("/", h.Home, middleware.RequireAuthentication()).Name = routeNameHome
	// Require authentication on the following once the testing is figured out
	g.GET("/about", h.About).Name = routeNameAbout
}

func (h *Pages) Home(ctx echo.Context) error {
	p := page.New(ctx)
	p.Layout = templates.LayoutMain
	p.Name = templates.PageHome
	p.Metatags.Description = "Welcome to the homepage."
	p.Metatags.Keywords = []string{"Go", "MVC", "Web", "Software"}
	p.Pager = page.NewPager(ctx, 4)
	p.Data = h.fetchPosts(&p.Pager)

	return h.RenderPage(ctx, p)
}

// fetchPosts is an mock example of fetching posts to illustrate how paging works
func (h *Pages) fetchPosts(pager *page.Pager) []post {
	pager.SetItems(20)
	posts := make([]post, 20)

	for k := range posts {
		posts[k] = post{
			Title: fmt.Sprintf("Post example #%d", k+1),
			Body:  fmt.Sprintf("Lorem ipsum example #%d ddolor sit amet, consectetur adipiscing elit. Nam elementum vulputate tristique.", k+1),
		}
	}
	return posts[pager.GetOffset() : pager.GetOffset()+pager.ItemsPerPage]
}

func (h *Pages) About(ctx echo.Context) error {
	p := page.New(ctx)
	p.Layout = templates.LayoutMain
	p.Name = templates.PageAbout
	p.Title = "About"

	// This page will be cached!
	p.Cache.Enabled = true
	p.Cache.Tags = []string{"page_about", "page:list"}

	// A simple example of how the Data field can contain anything you want to send to the templates
	// even though you wouldn't normally send markup like this
	p.Data = aboutData{
		ShowCacheWarning: true,
		FrontendTabs:     []aboutTab{},
	}

	return h.RenderPage(ctx, p)
}
