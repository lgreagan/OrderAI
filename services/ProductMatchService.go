package services

import (
	"context"
	"errors"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

var (
	ErrProductMatchInvalidStatus = errors.New("无效的组合规则状态")
	ErrProductMatchEmptyKeyword  = errors.New("搜索关键词不能为空")

	ErrProductMatchRuleCodeRequired  = errors.New("规则编号不能为空")
	ErrProductMatchRuleNameRequired  = errors.New("客户商品名称不能为空")
	ErrProductMatchRuleInvalidItem   = errors.New("组成单品无效，请重新选择 SKU")
	ErrProductMatchRuleInvalidQty    = errors.New("每套数量必须大于 0")
	ErrProductMatchRuleDuplicateSKU  = errors.New("组成单品 SKU 重复，请合并相同的 SKU")
	ErrProductMatchRuleItemsRequired = errors.New("请至少添加一个组成单品 SKU")
)

// ProductMatchService 负责组合 SKU 匹配规则的业务逻辑。
type ProductMatchService struct {
	Repo *repository.ProductMatchRepository
}

// NewProductMatchService 创建组合规则业务服务。
func NewProductMatchService(
	repo *repository.ProductMatchRepository,
) *ProductMatchService {
	return &ProductMatchService{
		Repo: repo,
	}
}

// List 查询组合匹配规则列表。
//
// keyword：匹配规则编号、客户商品名称或客户规格。
// status：空字符串表示全部；active 表示启用；inactive 表示停用。
func (s *ProductMatchService) List(
	ctx context.Context,
	keyword string,
	status string,
) ([]models.ProductMatchRule, error) {
	if s == nil || s.Repo == nil {
		return nil, errors.New("组合规则 Repository 尚未初始化")
	}

	keyword = strings.TrimSpace(keyword)
	status = strings.TrimSpace(status)

	if status != "" &&
		status != "active" &&
		status != "inactive" {
		return nil, ErrProductMatchInvalidStatus
	}

	rules, err := s.Repo.ListRule(ctx, keyword, status)
	if err != nil {
		return nil, err
	}

	ids := make([]uint64, 0, len(rules))
	for i := range rules {
		ids = append(ids, rules[i].ID)
	}

	itemsByRule, err := s.Repo.ListItemsByRuleIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	for i := range rules {
		items := itemsByRule[rules[i].ID]
		if items == nil {
			items = []models.ProductMatchRuleItemDetail{}
		}
		rules[i].Items = items
		rules[i].ItemCount = len(items)
	}

	return rules, nil
}

// Create 新建组合匹配规则，同时写入组成明细。
//
// 规则编号与客户商品名称必填；至少一个组成单品；
// 同一规则内不允许出现重复 SKU。
func (s *ProductMatchService) Create(
	ctx context.Context,
	rule *models.ProductMatchRule,
	items []models.ProductMatchRuleItem,
) (*models.ProductMatchRule, error) {
	if s == nil || s.Repo == nil {
		return nil, errors.New("组合规则 Repository 尚未初始化")
	}
	if rule == nil {
		return nil, errors.New("组合规则数据不能为空")
	}

	normalized, err := validateProductMatchRule(rule, items)
	if err != nil {
		return nil, err
	}

	if err := s.Repo.CreateRuleWithItems(ctx, rule, normalized); err != nil {
		return nil, err
	}

	rule.Items = make([]models.ProductMatchRuleItemDetail, 0, len(normalized))
	for _, item := range normalized {
		rule.Items = append(rule.Items, models.ProductMatchRuleItemDetail{
			ProductMatchRuleItem: item,
		})
	}
	rule.ItemCount = len(normalized)

	return rule, nil
}

// Update 修改组合匹配规则，组成明细按提交内容整体替换。
func (s *ProductMatchService) Update(
	ctx context.Context,
	id uint64,
	rule *models.ProductMatchRule,
	items []models.ProductMatchRuleItem,
) (*models.ProductMatchRule, error) {
	if s == nil || s.Repo == nil {
		return nil, errors.New("组合规则 Repository 尚未初始化")
	}
	if rule == nil {
		return nil, errors.New("组合规则数据不能为空")
	}
	if id <= 0 {
		return nil, errors.New("组合规则 ID 必须大于 0")
	}

	normalized, err := validateProductMatchRule(rule, items)
	if err != nil {
		return nil, err
	}

	rule.ID = id

	if err := s.Repo.UpdateRuleWithItems(ctx, rule, normalized); err != nil {
		return nil, err
	}

	rule.Items = make([]models.ProductMatchRuleItemDetail, 0, len(normalized))
	for _, item := range normalized {
		rule.Items = append(rule.Items, models.ProductMatchRuleItemDetail{
			ProductMatchRuleItem: item,
		})
	}
	rule.ItemCount = len(normalized)

	return rule, nil
}

// validateProductMatchRule 校验并规范化规则与组成明细，返回规范化后的明细。
//
// 规则编号与客户商品名称必填；至少一个组成单品；同一规则内不允许重复 SKU。
func validateProductMatchRule(
	rule *models.ProductMatchRule,
	items []models.ProductMatchRuleItem,
) ([]models.ProductMatchRuleItem, error) {
	rule.RuleCode = strings.TrimSpace(rule.RuleCode)
	rule.CustomerProductName = strings.TrimSpace(rule.CustomerProductName)
	rule.CustomerSpecification = strings.TrimSpace(rule.CustomerSpecification)
	rule.ProductNameZH = strings.TrimSpace(rule.ProductNameZH)
	rule.OrderUnit = strings.TrimSpace(rule.OrderUnit)
	rule.Remark = strings.TrimSpace(rule.Remark)

	if rule.RuleCode == "" {
		return nil, ErrProductMatchRuleCodeRequired
	}
	if rule.CustomerProductName == "" {
		return nil, ErrProductMatchRuleNameRequired
	}
	if rule.OrderUnit == "" {
		rule.OrderUnit = "套"
	}
	if rule.CartonWeight < 0 {
		rule.CartonWeight = 0
	}
	if rule.Status == "" {
		rule.Status = "active"
	}
	if rule.Status != "active" && rule.Status != "inactive" {
		return nil, ErrProductMatchInvalidStatus
	}

	normalized := make([]models.ProductMatchRuleItem, 0, len(items))
	seen := make(map[uint64]struct{}, len(items))

	for _, item := range items {
		if item.SkuID == 0 {
			return nil, ErrProductMatchRuleInvalidItem
		}
		if item.QuantityPerSet <= 0 {
			return nil, ErrProductMatchRuleInvalidQty
		}
		if _, ok := seen[item.SkuID]; ok {
			return nil, ErrProductMatchRuleDuplicateSKU
		}
		seen[item.SkuID] = struct{}{}

		item.Remark = strings.TrimSpace(item.Remark)
		normalized = append(normalized, item)
	}

	if len(normalized) == 0 {
		return nil, ErrProductMatchRuleItemsRequired
	}

	return normalized, nil
}

// GetByID 根据 ID 查询组合规则详情。
func (s *ProductMatchService) GetByID(
	ctx context.Context,
	id uint64,
) (*models.ProductMatchRule, error) {
	if s == nil || s.Repo == nil {
		return nil, errors.New("组合规则 Repository 尚未初始化")
	}

	if id <= 0 {
		return nil, errors.New("组合规则 ID 必须大于 0")
	}

	return s.Repo.GetRuleByID(ctx, id)
}

// CountReferences 统计规则被订单行引用的数量。
func (s *ProductMatchService) CountReferences(
	ctx context.Context,
	id uint64,
) (int64, error) {
	if s == nil || s.Repo == nil {
		return 0, errors.New("组合规则 Repository 尚未初始化")
	}
	if id <= 0 {
		return 0, errors.New("组合规则 ID 必须大于 0")
	}
	return s.Repo.CountRuleReferences(ctx, id)
}

// Delete 删除组合匹配规则，组成明细随外键级联删除。
//
// 返回删除前被订单行引用的数量，供上层提示历史订单已解除关联。
func (s *ProductMatchService) Delete(
	ctx context.Context,
	id uint64,
) (int64, error) {
	if s == nil || s.Repo == nil {
		return 0, errors.New("组合规则 Repository 尚未初始化")
	}
	if id <= 0 {
		return 0, errors.New("组合规则 ID 必须大于 0")
	}

	references, err := s.Repo.CountRuleReferences(ctx, id)
	if err != nil {
		return 0, err
	}

	if err := s.Repo.DeleteRule(ctx, id); err != nil {
		return 0, err
	}

	return references, nil
}

// UpdateStatus 修改组合规则状态。
func (s *ProductMatchService) UpdateStatus(
	ctx context.Context,
	id uint64,
	status string,
) error {
	if s == nil || s.Repo == nil {
		return errors.New("组合规则 Repository 尚未初始化")
	}

	if id <= 0 {
		return errors.New("组合规则 ID 必须大于 0")
	}

	status = strings.TrimSpace(status)
	if status != "active" && status != "inactive" {
		return ErrProductMatchInvalidStatus
	}

	return s.Repo.UpdateRuleStatus(ctx, id, status)
}
