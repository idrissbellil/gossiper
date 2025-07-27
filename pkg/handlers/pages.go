package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"math/rand"
	"strconv"

	"gitea.v3m.net/idriss/gossiper/config"
	"gitea.v3m.net/idriss/gossiper/ent"
	"gitea.v3m.net/idriss/gossiper/ent/job"
	gocontext "gitea.v3m.net/idriss/gossiper/pkg/context"
	"gitea.v3m.net/idriss/gossiper/pkg/middleware"
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
		ORM    *ent.Client
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
		Jobs        []*ent.Job
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
	user := ctx.Get(gocontext.AuthenticatedUserKey).(*ent.User)
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
	dbJob, err := h.ORM.Job.Create().
		SetEmail(generateRandomEmail(h.Config.Mailhog.Hostname)).
		SetURL(jobRead.URL).
		SetMethod(job.Method(jobRead.Method)).
		SetFromRegex(jobRead.FromRegex).
		SetUser(user).
		SetPayloadTemplate(jobRead.Payload).
		SetHeaders(headersMap).
		Save(context.Background())
	if err != nil {
		log.Printf("Error saving the job %v", err)
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
	err = h.ORM.Job.DeleteOneID(jobId).Exec(context.Background())
	if err != nil {
		log.Printf("Error deleting ID: %v", err)
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

func (h *Pages) fetchPosts(pager *page.Pager, user *ent.User) []*ent.Job {
	pager.SetItems(20)

	jobs, err := user.QueryJobs().
		Order(ent.Desc("created_at")).
		Limit(pager.ItemsPerPage).
		Offset(pager.GetOffset()).
		All(context.Background())
	if err != nil {
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
