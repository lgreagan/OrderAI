package services

import (
	"context"

	"OrderAI/models"
	"OrderAI/repository"
)

// StatisticsService 输出数据统计总览。
type StatisticsService struct {
	Repo     *repository.StatisticsRepository
	Settings *SettingsService
}

func NewStatisticsService(
	repo *repository.StatisticsRepository,
	settings *SettingsService,
) *StatisticsService {
	return &StatisticsService{Repo: repo, Settings: settings}
}

func (s *StatisticsService) Overview(
	ctx context.Context,
) (*models.StatisticsOverview, error) {
	threshold := 0.0
	if s.Settings != nil {
		threshold = s.Settings.LowStockThreshold(ctx)
	}
	return s.Repo.Overview(ctx, threshold)
}
