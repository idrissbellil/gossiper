package worker

import (
	"context"

	"gitea.v3m.net/idriss/gossiper/ent"
	"gitea.v3m.net/idriss/gossiper/ent/job"
)

type EntJobRepository struct {
	client *ent.Client
}

func NewEntJobRepository(client *ent.Client) *EntJobRepository {
	return &EntJobRepository{
		client: client,
	}
}

func (r *EntJobRepository) GetActiveJobs(ctx context.Context, email string) ([]*ent.Job, error) {
	return r.client.Job.Query().
		Where(job.EmailEQ(email)).
		Where(job.IsActive(true)).
		All(ctx)
}