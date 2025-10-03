package worker

import (
	"context"

	"gitea.v3m.net/idriss/gossiper/pkg/models"
)

type EntJobRepository struct {
	client *models.DB
}

func NewEntJobRepository(client *models.DB) *EntJobRepository {
	return &EntJobRepository{
		client: client,
	}
}

func (r *EntJobRepository) GetActiveJobs(ctx context.Context, email string) ([]*models.Job, error) {
	var jobs []*models.Job
	result := r.client.WithContext(ctx).
		Where("email = ? AND is_active = ?", email, true).
		Find(&jobs)

	if result.Error != nil {
		return nil, result.Error
	}

	return jobs, nil
}