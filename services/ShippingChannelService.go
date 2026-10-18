package services

import (
	"context"

	"OrderAI/models"
	"OrderAI/repository"
)

// ShippingChannelService 货代渠道字典读取。
type ShippingChannelService struct {
	Repo *repository.ShippingChannelRepository
}

func NewShippingChannelService(repo *repository.ShippingChannelRepository) *ShippingChannelService {
	return &ShippingChannelService{Repo: repo}
}

func (s *ShippingChannelService) List(ctx context.Context, country string) ([]models.ShippingChannel, error) {
	return s.Repo.List(ctx, country)
}
