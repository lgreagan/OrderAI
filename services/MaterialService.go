package services

import (
	"context"
	"fmt"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

// MaterialService 负责材料（包材/原材料）的业务逻辑。
type MaterialService struct {
	Repo *repository.MaterialRepository
}

func NewMaterialService(repo *repository.MaterialRepository) *MaterialService {
	return &MaterialService{Repo: repo}
}

func (s *MaterialService) List(
	ctx context.Context,
	keyword, category string,
) ([]models.Material, error) {
	return s.Repo.List(ctx, keyword, category)
}

func (s *MaterialService) Get(
	ctx context.Context,
	id uint64,
) (*models.Material, error) {
	return s.Repo.GetByID(ctx, id)
}

func validateMaterial(m *models.Material) error {
	m.Code = strings.TrimSpace(m.Code)
	m.Name = strings.TrimSpace(m.Name)
	m.NameZH = strings.TrimSpace(m.NameZH)

	if m.Code == "" {
		return fmt.Errorf("材料编码不能为空")
	}
	if m.Name == "" && m.NameZH == "" {
		return fmt.Errorf("材料名称不能为空")
	}
	if m.Price < 0 {
		return fmt.Errorf("单价不能小于0")
	}
	if m.SafetyStock < 0 {
		return fmt.Errorf("安全库存不能小于0")
	}
	switch m.Category {
	case models.MaterialCategoryPackaging, models.MaterialCategoryMaterial, "":
	default:
		return fmt.Errorf("材料分类无效")
	}
	if m.Status != "" && m.Status != "active" && m.Status != "inactive" {
		return fmt.Errorf("材料状态无效")
	}
	return nil
}

func (s *MaterialService) Create(
	ctx context.Context,
	m *models.Material,
) error {
	if err := validateMaterial(m); err != nil {
		return err
	}
	return s.Repo.Create(ctx, m)
}

func (s *MaterialService) Update(
	ctx context.Context,
	m *models.Material,
) error {
	if m.ID == 0 {
		return fmt.Errorf("材料 ID 无效")
	}
	if err := validateMaterial(m); err != nil {
		return err
	}
	return s.Repo.Update(ctx, m)
}

func (s *MaterialService) Delete(
	ctx context.Context,
	id uint64,
) error {
	if id == 0 {
		return fmt.Errorf("材料 ID 无效")
	}
	return s.Repo.Delete(ctx, id)
}
