package services

import (
	"context"
	"fmt"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

// ProductBomService 负责产品用料清单（BOM）的业务逻辑。
type ProductBomService struct {
	Repo *repository.ProductBomRepository
}

func NewProductBomService(repo *repository.ProductBomRepository) *ProductBomService {
	return &ProductBomService{Repo: repo}
}

func (s *ProductBomService) List(
	ctx context.Context,
	productType, keyword string,
) ([]models.ProductBom, error) {
	return s.Repo.List(ctx, productType, keyword)
}

func (s *ProductBomService) Get(
	ctx context.Context,
	id uint64,
) (*models.ProductBom, error) {
	return s.Repo.GetByID(ctx, id)
}

func validateBom(b *models.ProductBom) error {
	switch b.ProductType {
	case models.ProductionProductRule, models.ProductionProductSku:
	default:
		return fmt.Errorf("产品类型无效")
	}
	if b.ProductRefID == 0 {
		return fmt.Errorf("请选择产品")
	}
	if b.MaterialID == 0 {
		return fmt.Errorf("请选择材料")
	}
	if b.QuantityPerUnit <= 0 {
		return fmt.Errorf("每单位用量必须大于0")
	}
	if b.Status != "" && b.Status != "active" && b.Status != "inactive" {
		return fmt.Errorf("状态无效")
	}
	return nil
}

func (s *ProductBomService) Create(
	ctx context.Context,
	b *models.ProductBom,
) error {
	if err := validateBom(b); err != nil {
		return err
	}
	if err := s.Repo.Create(ctx, b); err != nil {
		return decorateBomError(err)
	}
	return nil
}

func (s *ProductBomService) Update(
	ctx context.Context,
	b *models.ProductBom,
) error {
	if b.ID == 0 {
		return fmt.Errorf("用料记录 ID 无效")
	}
	if err := validateBom(b); err != nil {
		return err
	}
	if err := s.Repo.Update(ctx, b); err != nil {
		return decorateBomError(err)
	}
	return nil
}

func (s *ProductBomService) Delete(
	ctx context.Context,
	id uint64,
) error {
	if id == 0 {
		return fmt.Errorf("用料记录 ID 无效")
	}
	return s.Repo.Delete(ctx, id)
}

// decorateBomError 把唯一键冲突转换为更友好的提示。
func decorateBomError(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		return fmt.Errorf("该产品已存在相同材料的用料记录")
	}
	return err
}
