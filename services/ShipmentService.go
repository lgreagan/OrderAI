package services

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"OrderAI/models"
	"OrderAI/repository"
)

// ShipmentService 负责发货单业务逻辑（分批发货、运费/保险、已发货量与库存扣减）。
type ShipmentService struct {
	Repo      *repository.ShipmentRepository
	Inventory *InventoryService
	Settings  *SettingsService
	// Requirements 用于发货时按 BOM 自动核销包装材料（可为 nil，则跳过核销）。
	Requirements *MaterialRequirementService
}

func NewShipmentService(
	repo *repository.ShipmentRepository,
	inventory *InventoryService,
	settings *SettingsService,
	requirements *MaterialRequirementService,
) *ShipmentService {
	return &ShipmentService{
		Repo:         repo,
		Inventory:    inventory,
		Settings:     settings,
		Requirements: requirements,
	}
}

// defaultCurrency 默认币种（来自系统设置）。
func (s *ShipmentService) defaultCurrency(ctx context.Context) string {
	if s.Settings != nil {
		return s.Settings.DefaultCurrency(ctx)
	}
	return "USD"
}

// formatQty 输出不带多余小数位的数量文本（64、1、1.5）。
func formatQty(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// BuildShipmentDescription 按 PI 描述约定拼接：{简称}x{数量}pcs ({每箱}pcs/ctn, {箱数}ctn)。
func BuildShipmentDescription(shortName string, quantity, packQuantity float64) string {
	shortName = strings.TrimSpace(shortName)
	if shortName == "" {
		return ""
	}

	text := shortName + "x" + formatQty(quantity) + "pcs"
	if packQuantity > 0 {
		// 与 CartonCount 口径一致：箱数按整箱计，尾数箱也算一箱。
		text += fmt.Sprintf(
			" (%spcs/ctn, %sctn)",
			formatQty(packQuantity),
			formatQty(float64(math.Ceil(round3(quantity/packQuantity)))),
		)
	}
	return text
}

func (s *ShipmentService) List(
	ctx context.Context,
	keyword, status string,
) ([]models.ShipmentListItem, error) {
	return s.Repo.List(ctx, keyword, status)
}

func (s *ShipmentService) Get(
	ctx context.Context,
	id uint64,
) (*models.Shipment, error) {
	return s.Repo.GetByID(ctx, id)
}

func (s *ShipmentService) Delete(
	ctx context.Context,
	id uint64,
) error {
	if id == 0 {
		return fmt.Errorf("发货单 ID 无效")
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	if s.Inventory != nil {
		return s.Inventory.Repo.DeleteByRef(ctx, "shipment", id)
	}
	return nil
}

// OrderContext 返回新建发货单所需的订单上下文（表头 + 可发货行 + 下一批次号）。
func (s *ShipmentService) OrderContext(
	ctx context.Context,
	orderID uint64,
) (*models.OrderShipmentContext, error) {
	if orderID == 0 {
		return nil, fmt.Errorf("订单 ID 无效")
	}

	oc, err := s.Repo.GetOrderHeader(ctx, orderID)
	if err != nil {
		return nil, err
	}

	lines, err := s.Repo.ListOrderLines(ctx, orderID)
	if err != nil {
		return nil, err
	}
	oc.Lines = lines

	batch, err := s.Repo.NextBatchNo(ctx, orderID)
	if err != nil {
		return nil, err
	}
	oc.NextBatchNo = batch

	return oc, nil
}

// PushDown 由客户订单下推发货单草稿：默认把该订单的剩余未发数量**一次性全部发完**（一票发完），
// 之后由发货员按实际情况拆分批次、调整数量与运费。
//
// 同一订单若已有下推草稿则直接返回该草稿，避免重复创建导致已发货量翻倍。
func (s *ShipmentService) PushDown(
	ctx context.Context,
	orderID uint64,
) (*models.Shipment, bool, error) {
	if orderID == 0 {
		return nil, false, fmt.Errorf("订单 ID 无效")
	}

	if id, err := s.Repo.FindDraftByOrder(ctx, orderID); err != nil {
		return nil, false, err
	} else if id != 0 {
		ship, err := s.Repo.GetByID(ctx, id)
		if err != nil {
			return nil, false, err
		}
		return ship, false, nil
	}

	oc, err := s.OrderContext(ctx, orderID)
	if err != nil {
		return nil, false, err
	}

	// 按订单行分组：组合套装（rule）行的组成单品必须整套一起发，
	// 否则会出现「发了 72 个 A、只发 70 个 B」这种凑不成套的明细。
	type orderGroup struct {
		orderItemID uint64
		lines       []models.OrderShipmentLine
	}
	groupIndex := make(map[uint64]int)
	groups := make([]orderGroup, 0, len(oc.Lines))
	for i := range oc.Lines {
		l := oc.Lines[i]
		idx, ok := groupIndex[l.OrderItemID]
		if !ok {
			idx = len(groups)
			groupIndex[l.OrderItemID] = idx
			groups = append(groups, orderGroup{orderItemID: l.OrderItemID})
		}
		groups[idx].lines = append(groups[idx].lines, l)
	}

	// buildItem 按单个组成单品生成发货明细行。
	buildItem := func(l models.OrderShipmentLine, quantity float64) models.ShipmentItem {
		orderItemID := l.OrderItemID
		item := models.ShipmentItem{
			OrderItemID:  &orderItemID,
			ItemType:     models.InventoryItemSku,
			SkuID:        l.SkuID,
			ShortName:    l.ShortName,
			Quantity:     quantity,
			Unit:         l.Unit,
			PackQuantity: l.PackQuantity,
			Source:       models.ShipmentSourceStock,
			UnitPrice:    l.UnitPrice,
		}
		if item.ShortName == "" {
			item.ShortName = firstNonEmpty(l.ProductNameZH, l.CustomerProductName)
		}
		item.Description = BuildShipmentDescription(
			item.ShortName, item.Quantity, item.PackQuantity,
		)
		return item
	}

	items := make([]models.ShipmentItem, 0, len(oc.Lines))
	for i := range groups {
		g := &groups[i]
		isSet := len(g.lines) > 0 && g.lines[0].MatchType == models.MatchTypeRule

		if !isSet {
			for j := range g.lines {
				l := g.lines[j]
				if l.RemainingQuantity <= 0 {
					continue
				}
				items = append(items, buildItem(l, l.RemainingQuantity))
			}
			continue
		}

		// 组合套装：可发套数取各组成单品「剩余 ÷ 每套用量」的最小值（向下取整到 0.001），
		// 保证不会超出任何一个单品的剩余量。
		availableSets := -1.0
		for j := range g.lines {
			l := g.lines[j]
			perSet := l.QuantityPerSet
			if perSet <= 0 {
				perSet = 1
			}
			sets := l.RemainingQuantity / perSet
			if availableSets < 0 || sets < availableSets {
				availableSets = sets
			}
		}
		if availableSets <= 0 {
			continue
		}
		availableSets = math.Floor(availableSets*1000) / 1000

		for j := range g.lines {
			l := g.lines[j]
			perSet := l.QuantityPerSet
			if perSet <= 0 {
				perSet = 1
			}
			quantity := round3(availableSets * perSet)
			if quantity <= 0 {
				continue
			}
			items = append(items, buildItem(l, quantity))
		}
	}

	if len(items) == 0 {
		return nil, false, fmt.Errorf("该订单没有可下推的发货明细（可能已全部发完）")
	}

	ship := &models.Shipment{
		OrderID:        orderID,
		BatchNo:        oc.NextBatchNo,
		ShipDate:       models.Date{Time: time.Now()},
		ShippingMethod: oc.ShippingMethod,
		Currency:       s.defaultCurrency(ctx),
		Status:         models.ShipmentStatusDraft,
		Remark: fmt.Sprintf(
			"由订单 %s 下推，默认全部一票发完，请按实际情况修改数量、箱数与运费",
			oc.OrderNumber,
		),
		Items: items,
	}

	if err := s.Save(ctx, ship); err != nil {
		return nil, false, err
	}

	return ship, true, nil
}

// Save 新增(ID=0)或更新发货单，并按状态同步库存扣减。
func (s *ShipmentService) Save(
	ctx context.Context,
	ship *models.Shipment,
) error {
	if ship.OrderID == 0 {
		return fmt.Errorf("请选择客户订单")
	}
	if len(ship.Items) == 0 {
		return fmt.Errorf("请至少添加一条发货明细")
	}
	if ship.Currency == "" {
		ship.Currency = s.defaultCurrency(ctx)
	}
	if ship.Status == "" {
		ship.Status = models.ShipmentStatusDraft
	}
	switch ship.Status {
	case models.ShipmentStatusDraft,
		models.ShipmentStatusShipped,
		models.ShipmentStatusDelivered:
	default:
		return fmt.Errorf("发货单状态无效")
	}

	if ship.ShippingMethod == "" {
		if oc, err := s.Repo.GetOrderHeader(ctx, ship.OrderID); err == nil {
			ship.ShippingMethod = oc.ShippingMethod
		}
	}

	if ship.BatchNo == 0 {
		batch, err := s.Repo.NextBatchNo(ctx, ship.OrderID)
		if err != nil {
			return err
		}
		ship.BatchNo = batch
	}

	// 明细行箱数 = 本次数量 ÷ 每箱数量；组合套装的每箱数量已在取数时按组合箱规折算
	// （每箱 N 套 × 每套 M 个），因此算出的箱数对套装同样成立。
	//
	// 表头总箱数按**订单行分组**统计：一个组合套装的组成单品装在同一箱里，
	// 若逐行累加会把同一箱重复计入（1 套 = 1 个 A + 1 个 B 时，A、B 行各算出 1 箱 → 2 箱）。
	groupCartons := make(map[uint64]float64)
	var looseCartons float64

	for i := range ship.Items {
		it := &ship.Items[i]
		if it.Quantity <= 0 {
			return fmt.Errorf("第 %d 行发货数量必须大于0", i+1)
		}
		if it.ItemType == "" {
			it.ItemType = models.InventoryItemSku
		}
		if it.Source == "" {
			it.Source = models.ShipmentSourceStock
		}
		if it.PackQuantity > 0 {
			// 箱数按**整箱**计：尾数箱也要占一个箱子，与前端带出的箱数（Math.ceil）保持一致。
			// 否则 200 ÷ 400 会存成 0.5 箱，回填后 FBA 箱号按整数累计就排不到号。
			it.CartonCount = float64(math.Ceil(round3(it.Quantity / it.PackQuantity)))
		} else {
			it.CartonCount = 0
		}
		it.Amount = round2(it.UnitPrice * it.Quantity)
		if it.Description == "" {
			it.Description = BuildShipmentDescription(it.ShortName, it.Quantity, it.PackQuantity)
		}

		if it.OrderItemID != nil {
			if cur, ok := groupCartons[*it.OrderItemID]; !ok || it.CartonCount > cur {
				groupCartons[*it.OrderItemID] = it.CartonCount
			}
		} else {
			looseCartons += it.CartonCount
		}
	}

	totalCartons := looseCartons
	for _, cartons := range groupCartons {
		totalCartons += cartons
	}
	ship.CartonCount = round3(totalCartons)
	ship.FreightCost = round2(ship.FreightCost)
	ship.InsuranceCost = round2(ship.InsuranceCost)

	if ship.ID == 0 {
		if ship.ShipmentNumber == "" {
			number, err := s.Repo.GenerateNumber(ctx)
			if err != nil {
				return err
			}
			ship.ShipmentNumber = number
		}
		if err := s.Repo.Create(ctx, ship); err != nil {
			return err
		}
	} else {
		// 发货单号由系统生成、前端不提交；更新时若为空则沿用原编号，
		// 避免被置空后撞上 uk_shipment_number 唯一键（Duplicate entry ''）。
		if ship.ShipmentNumber == "" {
			existing, err := s.Repo.GetByID(ctx, ship.ID)
			if err != nil {
				return err
			}
			ship.ShipmentNumber = existing.ShipmentNumber
			if ship.ShipmentNumber == "" {
				number, err := s.Repo.GenerateNumber(ctx)
				if err != nil {
					return err
				}
				ship.ShipmentNumber = number
			}
		}
		if err := s.Repo.Update(ctx, ship); err != nil {
			return err
		}
	}

	return s.syncInventory(ctx, ship)
}

// PackagingNeeds 核算本次发货将随货消耗的包装材料（只读预览，不写库存）。
//
// 供发货单界面在保存前展示"本单会扣掉多少纸箱"，避免库存被静默调整。
func (s *ShipmentService) PackagingNeeds(
	ctx context.Context,
	req *models.ShipmentPackagingRequest,
) ([]models.MaterialRequirement, error) {
	if s.Requirements == nil || req == nil || len(req.Items) == 0 {
		return []models.MaterialRequirement{}, nil
	}

	products := make([]MaterialProduct, 0, len(req.Items))
	for i := range req.Items {
		it := &req.Items[i]
		if it.SkuID == 0 || it.Quantity <= 0 {
			continue
		}
		products = append(products, MaterialProduct{
			ProductType: models.ProductionProductSku,
			RefID:       it.SkuID,
			Quantity:    it.Quantity,
		})
	}
	if len(products) == 0 {
		return []models.MaterialRequirement{}, nil
	}

	return s.Requirements.PackagingNeeds(ctx, products)
}

// syncInventory 重算该发货单的库存扣减：先清旧流水，若状态为已发货再按明细出库。
//
// 出库分两部分：明细行本身（成品/材料），以及按 BOM 自动核销的包装材料——
// 纸箱等包材随货发出即消耗，不应再依赖人工去领料页面点一次。
func (s *ShipmentService) syncInventory(
	ctx context.Context,
	ship *models.Shipment,
) error {
	if s.Inventory == nil {
		return nil
	}

	if err := s.Inventory.Repo.DeleteByRef(ctx, "shipment", ship.ID); err != nil {
		return err
	}

	if ship.Status != models.ShipmentStatusShipped &&
		ship.Status != models.ShipmentStatusDelivered {
		return nil
	}

	warehouse := s.Inventory.Warehouse(ctx)

	for i := range ship.Items {
		it := &ship.Items[i]
		if it.ItemType == models.InventoryItemMaterial && it.MaterialID != nil {
			if err := s.Inventory.RecordOut(
				ctx, models.InventoryItemMaterial, *it.MaterialID,
				models.InventoryBizShipmentOut, "shipment", ship.ID,
				it.Quantity, it.UnitPrice, ship.Currency, warehouse,
				"发货出库",
			); err != nil {
				return err
			}
			continue
		}

		if it.SkuID != nil {
			if err := s.Inventory.RecordOut(
				ctx, models.InventoryItemSku, *it.SkuID,
				models.InventoryBizShipmentOut, "shipment", ship.ID,
				it.Quantity, it.UnitPrice, ship.Currency, warehouse,
				"发货出库",
			); err != nil {
				return err
			}
		}
	}

	return s.syncPackagingConsumption(ctx, ship, warehouse)
}

// syncPackagingConsumption 按发货数量核销包装材料（BOM 中 category = packaging 的用料）。
//
// 与成品出库共用 ref_type = shipment，因此发货单重新保存时会随旧流水一起清掉再重算，
// 天然幂等。明细里已显式列出的材料会被跳过，避免重复扣减。
//
// 核销口径是「整单累计」而不是「单批」：纸箱按整只计，若每批各自向上取整，
// 1000pcs（144/箱，整单 7 箱）拆成 500+500 两批会各按 4 箱核销、共扣 8 箱。
// 因此这里按该订单全部已发货数量算出总箱数，再减去同一订单其它发货单已核销的量，只补差额。
//
// 注意：包材只在这一处出库。生产领料页面不应再领包材，否则同一只纸箱会被扣两次。
func (s *ShipmentService) syncPackagingConsumption(
	ctx context.Context,
	ship *models.Shipment,
	warehouse string,
) error {
	if s.Requirements == nil || ship.OrderID == 0 {
		return nil
	}

	// 明细里显式列出的材料已自行出库，不再自动核销。
	explicit := make(map[uint64]bool)
	for i := range ship.Items {
		it := &ship.Items[i]
		if it.ItemType == models.InventoryItemMaterial && it.MaterialID != nil {
			explicit[*it.MaterialID] = true
		}
	}

	// 汇总该订单下所有「已发货/已签收」单（含本单）的发货数量，并记下其它单的 ID。
	cumulative := make(map[uint64]float64)
	otherShipmentIDs := make([]uint64, 0)

	orders, err := s.Repo.ListByOrderIDs(ctx, []uint64{ship.OrderID})
	if err != nil {
		return err
	}
	for i := range orders {
		o := &orders[i]
		if o.Status != models.ShipmentStatusShipped &&
			o.Status != models.ShipmentStatusDelivered {
			continue
		}
		if o.ID != ship.ID {
			otherShipmentIDs = append(otherShipmentIDs, o.ID)
		}
		for j := range o.Items {
			it := &o.Items[j]
			if it.SkuID == nil || it.Quantity <= 0 {
				continue
			}
			cumulative[*it.SkuID] += it.Quantity
		}
	}

	if len(cumulative) == 0 {
		return nil
	}

	products := make([]MaterialProduct, 0, len(cumulative))
	for skuID, qty := range cumulative {
		products = append(products, MaterialProduct{
			ProductType: models.ProductionProductSku,
			RefID:       skuID,
			Quantity:    qty,
		})
	}

	needs, err := s.Requirements.PackagingNeeds(ctx, products)
	if err != nil {
		return err
	}

	// 同一订单其它发货单已经核销掉的包材，本单不再重复核销。
	done := make(map[uint64]float64)
	if len(otherShipmentIDs) > 0 {
		done, err = s.Inventory.Repo.SumByRefs(
			ctx, models.InventoryItemMaterial,
			models.InventoryBizShipmentPack, "shipment", otherShipmentIDs,
		)
		if err != nil {
			return err
		}
	}

	for i := range needs {
		n := &needs[i]
		if explicit[n.MaterialID] {
			continue
		}
		// 整单应核销总量 − 其它单已核销 = 本单应补的差额。
		diff := round3(n.Quantity - done[n.MaterialID])
		if diff <= 0 {
			continue
		}
		if err := s.Inventory.RecordOut(
			ctx, models.InventoryItemMaterial, n.MaterialID,
			models.InventoryBizShipmentPack, "shipment", ship.ID,
			diff, n.Price, ship.Currency, warehouse,
			"发货随货消耗（按整单累计发货量核销包装材料）",
		); err != nil {
			return err
		}
	}

	return nil
}
