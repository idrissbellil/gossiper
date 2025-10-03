package middleware

import (
	"fmt"
	"testing"

	"gitea.v3m.net/idriss/gossiper/pkg/context"
	"gitea.v3m.net/idriss/gossiper/pkg/models"
	"gitea.v3m.net/idriss/gossiper/pkg/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadUser(t *testing.T) {
	ctx, _ := tests.NewContext(c.Web, "/")
	ctx.SetParamNames("user")
	ctx.SetParamValues(fmt.Sprintf("%d", usr.ID))
	_ = tests.ExecuteMiddleware(ctx, LoadUser(c.ORM))
	ctxUsr, ok := ctx.Get(context.UserKey).(*models.User)
	require.True(t, ok)
	assert.Equal(t, usr.ID, ctxUsr.ID)
}
