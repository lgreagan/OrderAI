package services

import (
	"context"
	"fmt"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

type SKUService struct {
	Repo *repository.SKURepository
}

func NewSKUService(repo *repository.SKURepository) *SKUService {
	return &SKUService{Repo: repo}
}

func (s *SKUService) List(
	ctx context.Context,
	keyword, status string,
) ([]models.SKU, error) {
	return s.Repo.List(ctx, keyword, status)
}

func (s *SKUService) Get(
	ctx context.Context,
	id uint64,
) (*models.SKU, error) {
	return s.Repo.GetByID(ctx, id)
}

func validateSKU(sku *models.SKU) error {
	sku.SKUCode = strings.TrimSpace(sku.SKUCode)
	sku.ProductName = strings.TrimSpace(sku.ProductName)
	sku.ProductNameZH = strings.TrimSpace(sku.ProductNameZH)
	sku.Model = strings.TrimSpace(sku.Model)

	if sku.SKUCode == "" {
		return fmt.Errorf("SKU编码不能为空")
	}
	if sku.ProductName == "" {
		return fmt.Errorf("英文产品名不能为空")
	}
	if sku.ProductNameZH == "" {
		return fmt.Errorf("产品中文名不能为空")
	}
	if sku.Model == "" {
		return fmt.Errorf("产品型号不能为空")
	}
	if sku.Price < 0 {
		return fmt.Errorf("价格不能小于0")
	}
	if sku.PackQuantity <= 0 {
		return fmt.Errorf("每箱数量必须大于0")
	}
	if sku.BoxLength < 0 || sku.BoxWidth < 0 ||
		sku.BoxHeight < 0 || sku.BoxWeight < 0 {
		return fmt.Errorf("箱规和重量不能小于0")
	}
	if sku.Status != "" &&
		sku.Status != "active" &&
		sku.Status != "inactive" {
		return fmt.Errorf("SKU状态无效")
	}
	return nil
}

func (s *SKUService) Create(
	ctx context.Context,
	sku *models.SKU,
) error {
	if err := validateSKU(sku); err != nil {
		return err
	}
	return s.Repo.Create(ctx, sku)
}

func (s *SKUService) Update(
	ctx context.Context,
	sku *models.SKU,
) error {
	if sku.ID == 0 {
		return fmt.Errorf("SKU ID无效")
	}
	if err := validateSKU(sku); err != nil {
		return err
	}
	return s.Repo.Update(ctx, sku)
}

func (s *SKUService) Delete(
	ctx context.Context,
	id uint64,
) error {
	if id == 0 {
		return fmt.Errorf("SKU ID无效")
	}
	return s.Repo.Delete(ctx, id)
}
