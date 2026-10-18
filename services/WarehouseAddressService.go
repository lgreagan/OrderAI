package services

import (
	"context"
	"fmt"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

// WarehouseAddressService 仓库地址库（FBA 仓库地址）维护。
type WarehouseAddressService struct {
	Repo *repository.WarehouseAddressRepository
}

func NewWarehouseAddressService(repo *repository.WarehouseAddressRepository) *WarehouseAddressService {
	return &WarehouseAddressService{Repo: repo}
}

func (s *WarehouseAddressService) List(
	ctx context.Context,
	keyword, country string,
	limit int,
) ([]models.WarehouseAddress, error) {
	return s.Repo.List(ctx, keyword, country, limit)
}

func (s *WarehouseAddressService) Get(
	ctx context.Context,
	id uint64,
) (*models.WarehouseAddress, error) {
	if id == 0 {
		return nil, fmt.Errorf("仓库地址 ID 无效")
	}
	return s.Repo.GetByID(ctx, id)
}

func (s *WarehouseAddressService) Countries(ctx context.Context) ([]string, error) {
	return s.Repo.Countries(ctx)
}

func (s *WarehouseAddressService) Create(
	ctx context.Context,
	a *models.WarehouseAddress,
) error {
	if err := validateWarehouseAddress(a); err != nil {
		return err
	}
	return s.Repo.Create(ctx, a)
}

func (s *WarehouseAddressService) Update(
	ctx context.Context,
	a *models.WarehouseAddress,
) error {
	if a.ID == 0 {
		return fmt.Errorf("仓库地址 ID 无效")
	}
	if err := validateWarehouseAddress(a); err != nil {
		return err
	}
	return s.Repo.Update(ctx, a)
}

func (s *WarehouseAddressService) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return fmt.Errorf("仓库地址 ID 无效")
	}
	return s.Repo.Delete(ctx, id)
}

func validateWarehouseAddress(a *models.WarehouseAddress) error {
	a.ShortName = strings.TrimSpace(a.ShortName)
	if a.ShortName == "" {
		return fmt.Errorf("地址简称不能为空（报关模板的下拉按简称匹配）")
	}
	if strings.TrimSpace(a.Address1) == "" {
		return fmt.Errorf("地址一不能为空")
	}
	return nil
}
