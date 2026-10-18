package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

// SKUMatchService 负责把客户订单行匹配为单品 SKU。
//
// 匹配顺序：
//
//  1. 手动下单时显式选择的匹配产品：
//     直接匹配（MatchType = sku，按 sku_id 取单品）或
//     组合匹配（MatchType = rule，按 rule_id 展开规则的组成单品）。
//
//  2. 单品直配：
//     CONCAT(order_items.product_name, ' ', order_items.specification) = skus.product_name
//
//  3. 组合规则：订单行的
//     CONCAT(product_name, ' ', specification)
//     必须与规则的
//     CONCAT(customer_product_name, ' ', customer_specification)
//     完全一致（忽略首尾空格与大小写），命中后展开成
//     product_match_rule_items 中的多个单品 SKU。
type SKUMatchService struct {
	SKURepo  *repository.SKURepository
	RuleRepo *repository.ProductMatchRepository
}

func NewSKUMatchService(
	skuRepo *repository.SKURepository,
	ruleRepo *repository.ProductMatchRepository,
) *SKUMatchService {
	return &SKUMatchService{
		SKURepo:  skuRepo,
		RuleRepo: ruleRepo,
	}
}

// cartonUnits 是按“箱”计量的订单单位，命中时需要乘以规则的每箱套数。
var cartonUnits = map[string]struct{}{
	"箱":       {},
	"BOX":     {},
	"BOXES":   {},
	"CARTON":  {},
	"CARTONS": {},
	"CTN":     {},
	"CTNS":    {},
	"CASE":    {},
	"CASES":   {},
}

// isCartonUnit 判断订单单位是否按“箱”计量。
func isCartonUnit(unit string) bool {
	_, ok := cartonUnits[strings.ToUpper(strings.TrimSpace(unit))]
	return ok
}

// fullMatchKey 生成订单行与组合规则完全匹配用的键：
// 「名称 + 空格 + 规格」去掉首尾空格并忽略大小写。
func fullMatchKey(name, specification string) string {
	return matchKey(name + " " + specification)
}

// matchKey 生成匹配用的键：去掉首尾空格并忽略大小写。
//
// skus 与 product_match_rules 表使用 *_ci 排序规则，
// 数据库比较本身就是忽略大小写的，这里保持一致。
func matchKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// round3 按数据库 DECIMAL(15,3) 的精度取整。
func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}

// Match 计算每个订单行的 SKU 匹配结果（只读，不写库）。
func (s *SKUMatchService) Match(
	ctx context.Context,
	items []models.OrderItem,
) ([]models.OrderItemMatchPreview, error) {
	if s == nil || s.SKURepo == nil || s.RuleRepo == nil {
		return nil, errors.New("SKU 匹配服务尚未初始化")
	}

	previews := make([]models.OrderItemMatchPreview, len(items))

	for i := range items {
		previews[i] = models.OrderItemMatchPreview{
			LineNumber:     items[i].LineNumber,
			ProductName:    items[i].ProductName,
			Specification:  items[i].Specification,
			Quantity:       items[i].Quantity,
			Unit:           items[i].Unit,
			ActualQuantity: items[i].ActualQuantity,
			BaseQuantity:   items[i].Quantity,
			SKUs:           make([]models.OrderItemMatchSku, 0),
		}
	}

	if len(previews) == 0 {
		return previews, nil
	}

	// 手动下单时按所选产品匹配，优先于按名称匹配。
	if err := s.matchSelected(ctx, items, previews); err != nil {
		return nil, err
	}

	if err := s.matchDirectSKU(ctx, items, previews); err != nil {
		return nil, err
	}

	if err := s.matchRule(ctx, items, previews); err != nil {
		return nil, err
	}

	for i := range previews {
		if len(previews[i].SKUs) == 0 && previews[i].Message == "" {
			previews[i].Message = "未匹配到单品 SKU，请先在产品管理或组合规则中维护"
		}
	}

	return previews, nil
}

// ApplyMatches 把匹配结果填充到订单明细上，供入库时在同一个事务中保存。
//
// 返回的匹配预览可用于校验（例如未匹配的行）与提示。
func (s *SKUMatchService) ApplyMatches(
	ctx context.Context,
	order *models.Order,
) ([]models.OrderItemMatchPreview, error) {
	if order == nil || len(order.Items) == 0 {
		return nil, nil
	}

	previews, err := s.Match(ctx, order.Items)
	if err != nil {
		return nil, err
	}

	for i := range order.Items {
		preview := previews[i]

		matches := make([]models.OrderItemMatch, 0, len(preview.SKUs))

		for _, sku := range preview.SKUs {
			matches = append(matches, models.OrderItemMatch{
				SkuID:               sku.SkuID,
				RuleID:              preview.RuleID,
				QuantityPerSet:      sku.QuantityPerSet,
				OrderedQuantity:     preview.Quantity,
				RequiredQuantity:    sku.RequiredQuantity,
				SkuCodeSnapshot:     sku.SkuCode,
				ProductNameSnapshot: sku.ProductName,
				ModelSnapshot:       sku.Model,
				ColorSnapshot:       sku.Color,
				RuleCodeSnapshot:    preview.RuleCode,
			})
		}

		order.Items[i].Matches = matches
	}

	return previews, nil
}

// SummarizeMatches 统计匹配结果的行数分布。
func SummarizeMatches(
	previews []models.OrderItemMatchPreview,
) models.OrderMatchSummary {
	summary := models.OrderMatchSummary{Total: len(previews)}

	for _, preview := range previews {
		switch {
		case !preview.Matched:
			summary.UnmatchedCount++
		case preview.MatchType == models.MatchTypeSKU:
			summary.DirectCount++
		default:
			summary.RuleCount++
		}
	}

	return summary
}

// matchDirectSKU 执行单品直配。
func (s *SKUMatchService) matchDirectSKU(
	ctx context.Context,
	items []models.OrderItem,
	previews []models.OrderItemMatchPreview,
) error {
	keys := make([]string, 0, len(items)*2)
	seen := make(map[string]struct{}, len(items)*2)

	for i := range items {
		if previews[i].Matched || hasSelectedMatch(items[i]) {
			continue
		}

		for _, key := range directKeys(items[i].ProductName, items[i].Specification) {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}

	if len(keys) == 0 {
		return nil
	}

	skus, err := s.SKURepo.FindByProductNameKeys(ctx, keys)
	if err != nil {
		return err
	}

	// 同一个 product_name 对应多个 SKU 时，优先启用状态，其次取 id 较小者。
	byProductName := make(map[string]models.SKU, len(skus))

	for _, sku := range skus {
		key := matchKey(sku.ProductName)

		current, ok := byProductName[key]
		if !ok || betterSKU(sku, current) {
			byProductName[key] = sku
		}
	}

	for i := range items {
		if previews[i].Matched || hasSelectedMatch(items[i]) {
			continue
		}

		var sku models.SKU
		var found bool

		for _, key := range directKeys(items[i].ProductName, items[i].Specification) {
			if sku, found = byProductName[key]; found {
				break
			}
		}

		if !found {
			continue
		}

		// 单品直配时订单行就是该 SKU，需求数量用 AI 算出的实际数量。
		required := items[i].ActualQuantity
		if required <= 0 {
			required = items[i].Quantity
		}

		applyDirectMatch(&previews[i], sku, required)
	}

	return nil
}

// applyDirectMatch 把单品 SKU 作为订单行的直配结果写入预览。
func applyDirectMatch(
	preview *models.OrderItemMatchPreview,
	sku models.SKU,
	required float64,
) {
	preview.Matched = true
	preview.MatchType = models.MatchTypeSKU
	preview.SKUs = []models.OrderItemMatchSku{
		{
			SkuID:            sku.ID,
			SkuCode:          sku.SKUCode,
			ProductName:      sku.ProductName,
			ProductNameZH:    sku.ProductNameZH,
			Model:            sku.Model,
			Color:            sku.Color,
			Status:           sku.Status,
			QuantityPerSet:   1,
			RequiredQuantity: round3(required),
		},
	}
}

// directKeys 返回订单行用于单品直配的名称键：
// 先按 CONCAT(商品名称, ' ', 规格) 匹配，再退化为单独的客户商品名称。
func directKeys(productName, specification string) []string {
	keys := make([]string, 0, 2)

	if key := matchKey(productName + " " + specification); key != "" {
		keys = append(keys, key)
	}

	if key := matchKey(productName); key != "" && (len(keys) == 0 || key != keys[0]) {
		keys = append(keys, key)
	}

	return keys
}

// betterSKU 判断 candidate 是否比 current 更适合作为匹配结果。
func betterSKU(candidate, current models.SKU) bool {
	candidateActive := candidate.Status == "active"
	currentActive := current.Status == "active"

	if candidateActive != currentActive {
		return candidateActive
	}

	return candidate.ID < current.ID
}

// hasSelectedMatch 判断订单行是否由下单人显式选择了匹配产品
// （直接匹配单品 SKU 或组合匹配规则）。
func hasSelectedMatch(item models.OrderItem) bool {
	if item.MatchID == 0 {
		return false
	}

	return item.MatchType == models.MatchTypeSKU || item.MatchType == models.MatchTypeRule
}

// matchSelected 处理下单时显式选择的匹配产品：直接匹配的单品 SKU
// 与组合匹配的规则；按名称匹配只处理没有选择的行。
func (s *SKUMatchService) matchSelected(
	ctx context.Context,
	items []models.OrderItem,
	previews []models.OrderItemMatchPreview,
) error {
	skus := make(map[uint64]models.SKU)
	chosen := make([]*models.ProductMatchRule, len(items))
	ruleIDs := make([]uint64, 0, len(items))
	seenRule := make(map[uint64]struct{}, len(items))

	for i := range items {
		item := items[i]

		if !hasSelectedMatch(item) {
			continue
		}

		if item.MatchType == models.MatchTypeSKU {
			sku, ok := skus[item.MatchID]

			if !ok {
				loaded, err := s.SKURepo.GetByID(ctx, item.MatchID)
				if err != nil {
					previews[i].Message = fmt.Sprintf(
						"所选单品 SKU（ID %d）不存在，请重新选择",
						item.MatchID,
					)
					continue
				}

				sku = *loaded
				skus[sku.ID] = sku
			}

			required := item.ActualQuantity
			if required <= 0 {
				required = item.Quantity
			}

			applyDirectMatch(&previews[i], sku, required)
			continue
		}

		rule, err := s.RuleRepo.GetRuleByID(ctx, item.MatchID)
		if err != nil {
			previews[i].Message = fmt.Sprintf(
				"所选组合规则（ID %d）不存在，请重新选择",
				item.MatchID,
			)
			continue
		}

		chosen[i] = rule

		if _, ok := seenRule[rule.ID]; !ok {
			seenRule[rule.ID] = struct{}{}
			ruleIDs = append(ruleIDs, rule.ID)
		}
	}

	if len(ruleIDs) == 0 {
		return nil
	}

	ruleItems, err := s.RuleRepo.ListItemsByRuleIDs(ctx, ruleIDs)
	if err != nil {
		return err
	}

	for i := range items {
		if chosen[i] == nil {
			continue
		}

		applyRuleMatch(
			items[i],
			&previews[i],
			*chosen[i],
			ruleItems[chosen[i].ID],
		)
	}

	return nil
}

// applyRuleMatch 把命中的组合规则展开成订单行的匹配结果。
func applyRuleMatch(
	item models.OrderItem,
	preview *models.OrderItemMatchPreview,
	rule models.ProductMatchRule,
	details []models.ProductMatchRuleItemDetail,
) {
	if len(details) == 0 {
		preview.Message = fmt.Sprintf(
			"组合规则 %s 尚未配置组成单品 SKU",
			rule.RuleCode,
		)
		return
	}

	// 订单按“箱”下单时，先换算成套：套数 = 订单数量 × 每箱套数
	base := item.Quantity
	cartonConverted := false

	if isCartonUnit(item.Unit) && rule.SetsPerCarton > 0 {
		base = base * rule.SetsPerCarton
		cartonConverted = true
	}

	ruleID := rule.ID

	skus := make([]models.OrderItemMatchSku, 0, len(details))

	for _, detail := range details {
		quantityPerSet := detail.QuantityPerSet
		if quantityPerSet <= 0 {
			quantityPerSet = 1
		}

		skus = append(skus, models.OrderItemMatchSku{
			SkuID:            detail.SkuID,
			SkuCode:          detail.SKUCode,
			ProductName:      detail.ProductName,
			ProductNameZH:    detail.ProductNameZH,
			Model:            detail.Model,
			Color:            detail.Color,
			Status:           detail.Status,
			QuantityPerSet:   quantityPerSet,
			RequiredQuantity: round3(base * quantityPerSet),
		})
	}

	preview.Matched = true
	preview.MatchType = models.MatchTypeRule
	preview.RuleID = &ruleID
	preview.RuleCode = rule.RuleCode
	preview.SetsPerCarton = rule.SetsPerCarton
	preview.BaseQuantity = round3(base)
	preview.CartonConverted = cartonConverted
	preview.SKUs = skus
}

// matchRule 对未直配成功的订单行执行组合规则完全匹配。
func (s *SKUMatchService) matchRule(
	ctx context.Context,
	items []models.OrderItem,
	previews []models.OrderItemMatchPreview,
) error {
	keys := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))

	for i := range items {
		if previews[i].Matched || hasSelectedMatch(items[i]) {
			continue
		}

		key := fullMatchKey(items[i].ProductName, items[i].Specification)
		if key == "" {
			continue
		}

		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	if len(keys) == 0 {
		return nil
	}

	rules, err := s.RuleRepo.FindActiveRulesByFullNames(ctx, keys)
	if err != nil {
		return err
	}

	rulesByKey := make(map[string]models.ProductMatchRule, len(rules))

	for _, rule := range rules {
		key := fullMatchKey(rule.CustomerProductName, rule.CustomerSpecification)

		// 同一拼接结果对应多条规则时保留 id 较小的一条
		if _, ok := rulesByKey[key]; !ok {
			rulesByKey[key] = rule
		}
	}

	chosen := make([]*models.ProductMatchRule, len(items))
	ruleIDs := make([]uint64, 0, len(items))
	seenRule := make(map[uint64]struct{}, len(items))

	for i := range items {
		if previews[i].Matched || hasSelectedMatch(items[i]) {
			continue
		}

		rule, ok := rulesByKey[fullMatchKey(
			items[i].ProductName,
			items[i].Specification,
		)]
		if !ok {
			continue
		}

		chosen[i] = &rule

		if _, ok := seenRule[rule.ID]; !ok {
			seenRule[rule.ID] = struct{}{}
			ruleIDs = append(ruleIDs, rule.ID)
		}
	}

	if len(ruleIDs) == 0 {
		return nil
	}

	ruleItems, err := s.RuleRepo.ListItemsByRuleIDs(ctx, ruleIDs)
	if err != nil {
		return err
	}

	for i := range items {
		if chosen[i] == nil {
			continue
		}

		applyRuleMatch(items[i], &previews[i], *chosen[i], ruleItems[chosen[i].ID])
	}

	return nil
}
