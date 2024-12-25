package handlers

import (
	"context"
	"html/template"
	"log"

	"gitea.risky.info/risky-info/gossiper/ent"
	gocontext "gitea.risky.info/risky-info/gossiper/pkg/context"
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
	g.POST("/jobs", h.JobAdd, middleware.RequireAuthentication()).Name = "jobadd"
	g.PUT("/jobs/:id", h.JobUpdate, middleware.RequireAuthentication()).Name = "jobupdate"
	g.DELETE("/jobs/:id", h.JobDelete, middleware.RequireAuthentication()).Name = "jobdelete"
	// Require authentication on the following once the testing is figured out
	g.GET("/about", h.About).Name = routeNameAbout
}

func (h *Pages) JobAdd(ctx echo.Context) error {
	user := ctx.Get(gocontext.AuthenticatedUserKey).(*ent.User)
	return h.Home(ctx)
}

func (h *Pages) JobDelete(ctx echo.Context) error {
	// Delete ..
	return h.Home(ctx)
}

func (h *Pages) JobUpdate(ctx echo.Context) error {
	// Update ..
	return h.Home(ctx)
}

func (h *Pages) Home(ctx echo.Context) error {
	p := page.New(ctx)
	p.Layout = templates.LayoutMain
	p.Name = templates.PageHome
	p.Metatags.Description = "Welcome to the homepage."
	p.Metatags.Keywords = []string{"gossip", "email", "api"}
	p.Pager = page.NewPager(ctx, 4)
	p.Data = h.fetchPosts(&p.Pager, p.AuthUser)

	return h.RenderPage(ctx, p)
}

// fetchPosts is an mock example of fetching posts to illustrate how paging works
func (h *Pages) fetchPosts(pager *page.Pager, user *ent.User) []*ent.Job {
	pager.SetItems(20)

	// Query jobs for the user
	jobs, err := user.QueryJobs().
		Order(ent.Desc("created_at")).
		Limit(pager.ItemsPerPage).
		Offset(pager.GetOffset()).
		All(context.Background())
	if err != nil {
		// Handle error appropriately in your application
		log.Printf("Error fetching jobs: %v", err)
		return []*ent.Job{}
	}

	return jobs
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
