package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"math/rand"
	"strconv"

	"gitea.v3m.net/idriss/gossiper/config"
	gocontext "gitea.v3m.net/idriss/gossiper/pkg/context"
	"gitea.v3m.net/idriss/gossiper/pkg/middleware"
	"gitea.v3m.net/idriss/gossiper/pkg/models"
	"gitea.v3m.net/idriss/gossiper/pkg/page"
	"gitea.v3m.net/idriss/gossiper/pkg/services"
	"gitea.v3m.net/idriss/gossiper/templates"
	"github.com/labstack/echo/v4"
)

const (
	routeNameAbout = "about"
	routeNameHome  = "home"
)

type (
	Pages struct {
		*services.TemplateRenderer
		ORM    *models.DB
		Config *config.Config
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
	jobRead struct {
		URL       string `json:"url" form:"url"`
		Method    string `json:"method" form:"method"`
		Headers   string `json:"headers" form:"headers"`
		Payload   string `json:"payload" form:"payload"`
		FromRegex string `json:"from_regex" form:"from_regex"`
	}
	inputField struct {
		Name  string
		Label string
		Extra string
		Type  string
	}
	renderData struct {
		Jobs        []*models.Job
		InputFields []inputField
	}
)

func init() {
	Register(new(Pages))
}

func (h *Pages) Init(c *services.Container) error {
	h.TemplateRenderer = c.TemplateRenderer
	h.ORM = c.ORM
	h.Config = c.Config
	return nil
}

func (h *Pages) Routes(g *echo.Group) {
	g.GET("/", h.Home, middleware.RequireAuthentication()).Name = routeNameHome
	g.POST("/jobs", h.JobAdd, middleware.RequireAuthentication()).Name = "jobadd"
	g.DELETE("/jobs/:id", h.JobDelete, middleware.RequireAuthentication()).Name = "jobdelete"
	// Require authentication on the following once the testing is figured out
	g.GET("/about", h.About).Name = routeNameAbout
}

func generateRandomEmail(hostname string) string {
	// FIXME check for collisions
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b) + "@" + hostname
}

func (h *Pages) JobAdd(ctx echo.Context) error {
	user := ctx.Get(gocontext.AuthenticatedUserKey).(*models.User)
	jobRead := jobRead{}
	if err := ctx.Bind(&jobRead); err != nil {
		log.Printf("Error loading form data: %v", err)
		return h.Home(ctx)
	}
	var headersMap map[string]string
	if jobRead.Headers != "" {
		if err := json.Unmarshal([]byte(jobRead.Headers), &headersMap); err != nil {
			log.Printf("Error loading headers: %v", err)
		}
	}
	dbJob := &models.Job{
		Email:           generateRandomEmail(h.Config.Mailcrab.Hostname),
		URL:             jobRead.URL,
		Method:          jobRead.Method,
		FromRegex:       jobRead.FromRegex,
		UserID:          user.ID,
		PayloadTemplate: jobRead.Payload,
		Headers:         headersMap,
	}
	result := h.ORM.WithContext(context.Background()).Create(dbJob)
	if result.Error != nil {
		log.Printf("Error saving the job %v", result.Error)
	}

	log.Println(dbJob)

	return h.Home(ctx)
}

func (h *Pages) JobDelete(ctx echo.Context) error {
	jobId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		log.Printf("Error loading job ID: %v", err)
		return h.Home(ctx)
	}
	result := h.ORM.WithContext(context.Background()).Delete(&models.Job{}, jobId)
	if result.Error != nil {
		log.Printf("Error deleting ID: %v", result.Error)
		return h.Home(ctx)
	}
	return h.Home(ctx)
}

func (h *Pages) Home(ctx echo.Context) error {
	p := page.New(ctx)
	p.Layout = templates.LayoutMain
	p.Name = templates.PageHome
	p.Metatags.Description = "Welcome to the homepage."
	p.Metatags.Keywords = []string{"gossip", "email", "api"}
	p.Pager = page.NewPager(ctx, 4)

	p.Data = renderData{
		Jobs: h.fetchPosts(&p.Pager, p.AuthUser),
		InputFields: []inputField{
			{Name: "url", Label: "URL", Type: "input", Extra: "required"},
			{Name: "method", Label: "HTTP Method", Type: "input", Extra: ""},
			{Name: "from_regex", Label: "From Regex", Type: "input", Extra: ""},
			{Name: "headers", Label: "Headers", Type: "textarea", Extra: ""},
			{Name: "payload", Label: "Payload", Type: "textarea", Extra: ""},
		},
	}
	return h.RenderPage(ctx, p)
}

func (h *Pages) fetchPosts(pager *page.Pager, user *models.User) []*models.Job {
	pager.SetItems(20)

	var jobs []*models.Job
	result := h.ORM.WithContext(context.Background()).
		Where("user_id = ?", user.ID).
		Order("created_at DESC").
		Limit(pager.ItemsPerPage).
		Offset(pager.GetOffset()).
		Find(&jobs)
	if result.Error != nil {
		log.Printf("Error fetching jobs: %v", result.Error)
		return []*models.Job{}
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
