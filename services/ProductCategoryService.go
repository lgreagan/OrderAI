package services

import (
	"context"
	"fmt"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

// ProductCategoryService 报关品类维护。
type ProductCategoryService struct {
	Repo *repository.ProductCategoryRepository
}

func NewProductCategoryService(repo *repository.ProductCategoryRepository) *ProductCategoryService {
	return &ProductCategoryService{Repo: repo}
}

func (s *ProductCategoryService) List(
	ctx context.Context,
	keyword, status string,
) ([]models.ProductCategory, error) {
	return s.Repo.List(ctx, keyword, status)
}

func (s *ProductCategoryService) Get(
	ctx context.Context,
	id uint64,
) (*models.ProductCategory, error) {
	if id == 0 {
		return nil, fmt.Errorf("报关品类 ID 无效")
	}
	return s.Repo.GetByID(ctx, id)
}

func validateProductCategory(c *models.ProductCategory) error {
	c.Code = strings.TrimSpace(c.Code)
	c.NameEN = strings.TrimSpace(c.NameEN)
	c.NameZH = strings.TrimSpace(c.NameZH)

	if c.NameZH == "" {
		return fmt.Errorf("产品中文品名不能为空")
	}
	if c.NameEN == "" {
		return fmt.Errorf("产品英文品名不能为空")
	}
	if c.DeclarePrice < 0 {
		return fmt.Errorf("申报单价不能小于 0")
	}
	if c.Status != "" && c.Status != "active" && c.Status != "inactive" {
		return fmt.Errorf("状态无效")
	}
	return nil
}

func (s *ProductCategoryService) Create(
	ctx context.Context,
	c *models.ProductCategory,
) error {
	if err := validateProductCategory(c); err != nil {
		return err
	}
	return s.Repo.Create(ctx, c)
}

func (s *ProductCategoryService) Update(
	ctx context.Context,
	c *models.ProductCategory,
) error {
	if c.ID == 0 {
		return fmt.Errorf("报关品类 ID 无效")
	}
	if err := validateProductCategory(c); err != nil {
		return err
	}
	return s.Repo.Update(ctx, c)
}

func (s *ProductCategoryService) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return fmt.Errorf("报关品类 ID 无效")
	}
	return s.Repo.Delete(ctx, id)
}
