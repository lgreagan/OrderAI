package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

type OrderService struct {
	Repo    *repository.OrderRepository
	Matcher *SKUMatchService
}

// OrderValidationError 表示订单数据不合法，控制器据此返回 400。
type OrderValidationError struct {
	Message string
}

func (e *OrderValidationError) Error() string {
	return e.Message
}

func newOrderValidationError(format string, args ...any) error {
	return &OrderValidationError{Message: fmt.Sprintf(format, args...)}
}

func NewOrderService(
	repo *repository.OrderRepository,
	matcher *SKUMatchService,
) *OrderService {

	return &OrderService{
		Repo:    repo,
		Matcher: matcher,
	}
}

func (s *OrderService) Create(
	ctx context.Context,
	order *models.Order,
) error {

	if err := s.prepareOrder(ctx, order); err != nil {
		return err
	}

	return s.Repo.Create(
		ctx,
		order,
	)
}

// prepareOrder 规范化并校验订单，然后计算每个订单行的匹配结果。
//
// 下单时必须为每个产品行选择匹配产品（单品直配或组合规则），
// 匹配不上的行会直接拒绝保存。
func (s *OrderService) prepareOrder(
	ctx context.Context,
	order *models.Order,
) error {

	if order == nil {
		return newOrderValidationError("订单数据不能为空")
	}

	normalizeOrder(order)

	if err := validateOrderSubmission(order); err != nil {
		return err
	}

	if s.Matcher == nil {
		return errors.New("SKU 匹配服务尚未初始化")
	}

	previews, err := s.Matcher.ApplyMatches(ctx, order)
	if err != nil {
		return err
	}

	return validateOrderMatches(order, previews)
}

// normalizeOrder 去掉表头与明细的首尾空格，并重排行号。
func normalizeOrder(order *models.Order) {
	order.OrderNumber = strings.TrimSpace(order.OrderNumber)
	order.Country = strings.TrimSpace(order.Country)
	order.ShippingMethod = strings.TrimSpace(order.ShippingMethod)
	order.Customer = strings.TrimSpace(order.Customer)
	order.Status = strings.TrimSpace(order.Status)

	if order.Status == "" {
		order.Status = models.OrderStatusConfirmed
	}

	for i := range order.Items {
		item := &order.Items[i]

		item.LineNumber = i + 1
		item.ProductName = strings.TrimSpace(item.ProductName)
		item.Specification = strings.TrimSpace(item.Specification)
		item.Description = strings.TrimSpace(item.Description)
		item.Unit = strings.TrimSpace(item.Unit)
		item.DeclaredQuantity = strings.TrimSpace(item.DeclaredQuantity)

		if item.Unit == "" {
			item.Unit = "套"
		}

		if item.ActualQuantity <= 0 {
			item.ActualQuantity = item.Quantity
		}
	}
}

// validateOrderSubmission 校验订单表头与明细的必填项。
func validateOrderSubmission(order *models.Order) error {
	if order.OrderNumber == "" {
		return newOrderValidationError("订单号不能为空")
	}

	if !models.IsValidOrderStatus(order.Status) {
		return newOrderValidationError("订单状态无效")
	}

	if len(order.Items) == 0 {
		return newOrderValidationError("订单至少需要一个产品")
	}

	for i := range order.Items {
		item := &order.Items[i]

		if !hasSelectedMatch(*item) {
			return newOrderValidationError(
				"第 %d 行未选择匹配产品，请选择单品 SKU 或组合规则",
				item.LineNumber,
			)
		}

		if item.Quantity <= 0 {
			return newOrderValidationError(
				"第 %d 行数量必须大于 0",
				item.LineNumber,
			)
		}
	}

	return nil
}

// validateOrderMatches 校验每个订单行都匹配到了单品 SKU。
func validateOrderMatches(
	order *models.Order,
	previews []models.OrderItemMatchPreview,
) error {
	for i := range order.Items {
		if len(order.Items[i].Matches) > 0 {
			continue
		}

		reason := ""

		if i < len(previews) {
			reason = strings.TrimSpace(previews[i].Message)
		}

		if reason == "" {
			reason = "请检查所选产品是否已维护组成单品"
		}

		return newOrderValidationError(
			"第 %d 行未匹配到单品 SKU：%s",
			order.Items[i].LineNumber,
			reason,
		)
	}

	return nil
}

// List 查询订单列表，并附带每张订单的识别源图路径。
//
// 源图优先取提交给 AI 识别的那张原图（ai_results.image_path），
// 缺失时回退 orders.source_image；取不到源图不影响订单列表本身。
func (s *OrderService) List(
	ctx context.Context,
) ([]models.Order, error) {
	orders, err := s.Repo.List(ctx)
	if err != nil {
		return nil, err
	}

	if len(orders) == 0 {
		return orders, nil
	}

	ids := make([]uint64, 0, len(orders))
	for i := range orders {
		ids = append(ids, orders[i].ID)
	}

	images, err := s.Repo.SourceImagesByIDs(ctx, ids)
	if err != nil {
		return orders, nil
	}

	for i := range orders {
		orders[i].RecognitionImage = strings.TrimSpace(images[orders[i].ID])
	}

	return orders, nil
}

func (s *OrderService) Get(
	ctx context.Context,
	id uint64,
) (*models.Order, error) {

	return s.Repo.GetByID(
		ctx,
		id,
	)
}

func (s *OrderService) Update(
	ctx context.Context,
	order *models.Order,
) error {

	if err := s.prepareOrder(ctx, order); err != nil {
		return err
	}

	return s.Repo.Update(
		ctx,
		order,
	)
}

func (s *OrderService) Delete(
	ctx context.Context,
	id uint64,
) error {

	return s.Repo.Delete(
		ctx,
		id,
	)
}

func (s *OrderService) CreateWithAI(
	ctx context.Context,
	order *models.Order,
	rawJSON string,
	imagePath string,
	provider string,
	model string,
) error {

	if order.Status == "" {
		order.Status = "confirmed"
	}

	// 确认入库前计算订单行与 SKU 的匹配结果，
	// 匹配结果与订单数据在同一个事务中写入。
	if s.Matcher != nil {
		if _, err := s.Matcher.ApplyMatches(ctx, order); err != nil {
			return err
		}
	}

	return s.Repo.CreateWithAI(
		ctx,
		order,
		rawJSON,
		imagePath,
		provider,
		model,
	)
}