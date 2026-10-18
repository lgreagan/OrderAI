package models

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrProductionOrderNotFound 表示生产订单不存在。
var ErrProductionOrderNotFound = errors.New("生产订单不存在")

// 生产单状态。
const (
	ProductionStatusDraft     = "draft"
	ProductionStatusProducing = "producing"
	ProductionStatusCompleted = "completed"
	ProductionStatusCancelled = "cancelled"
)

// 生产对象类型。
const (
	ProductionProductRule = "rule" // 组合套
	ProductionProductSku  = "sku"  // 单品
)

// 生产单作业类型：显式决定完工时是否产生成品入库。
//
// 引入前靠 source_purchase_order_id 是否有值来间接推断，导致「从客户订单下推」
// 或「手工新建」的生产单只要涉及采购到货的货，就会与采购入库重复记账。
const (
	ProductionWorkProduce = "produce" // 投料生产：领原料出库，完工成品入库
	ProductionWorkPack    = "pack"    // 包装装箱：货已在采购入库，只耗包装材料
	ProductionWorkNone    = "none"    // 只记录：仅排期与统计，不涉及库存
)

// NormalizeWorkType 归一化作业类型，空值或未知值一律按投料生产处理。
func NormalizeWorkType(value string) string {
	switch strings.TrimSpace(value) {
	case ProductionWorkPack:
		return ProductionWorkPack
	case ProductionWorkNone:
		return ProductionWorkNone
	default:
		return ProductionWorkProduce
	}
}

// WorkTypeInboundsFinished 判断该作业类型完工时是否产生成品入库。
//
// 只有投料生产需要入成品库；包装装箱的货已由采购到货入库，再入一次即重复。
func WorkTypeInboundsFinished(value string) bool {
	return NormalizeWorkType(value) == ProductionWorkProduce
}

// WorkTypeLabel 返回作业类型的中文名，用于界面展示。
func WorkTypeLabel(value string) string {
	switch NormalizeWorkType(value) {
	case ProductionWorkPack:
		return "包装装箱"
	case ProductionWorkNone:
		return "只记录"
	default:
		return "投料生产"
	}
}

// ProductionOrder 生产订单头（持久化）。
type ProductionOrder struct {
	ID            uint64    `json:"id"`
	PONumber      string    `json:"po_number"`
	SourceOrderID *uint64   `json:"source_order_id"`
	// SourcePurchaseOrderID 来源采购单。原「从采购订单下推」入口已移除——
	// 采购按单品入库、不体现组合套装，推不出与客户订单一致的产品。
	// 字段保留仅为兼容历史单据，新建的生产单不再写入。
	SourcePurchaseOrderID *uint64   `json:"source_purchase_order_id"`
	PlanDate              Date      `json:"plan_date"`
	StartDate             Date      `json:"start_date"`
	FinishDate            Date      `json:"finish_date"`
	Status                string    `json:"status"`
	// WorkType 作业类型（produce/pack/none）：决定完工时是否入成品库。
	WorkType              string    `json:"work_type"`
	Remark                string    `json:"remark"`
	CreatedBy             string    `json:"created_by"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`

	// SourceOrderNumber 关联订单号（展示用）。
	SourceOrderNumber string `json:"source_order_number"`
	// SourcePurchaseNumber 关联采购单号（展示用）。
	SourcePurchaseNumber string `json:"source_purchase_number"`

	Items []ProductionOrderItem `json:"items"`
}

// ProductionOrderItem 生产明细行：一个组合套或一个单品。
type ProductionOrderItem struct {
	ID                uint64  `json:"id"`
	ProductionOrderID uint64  `json:"production_order_id"`
	ProductType       string  `json:"product_type"`
	ProductRefID      *uint64 `json:"product_ref_id"`
	ProductName       string  `json:"product_name"`
	ShortName         string  `json:"short_name"`
	Spec              string  `json:"spec"`
	PlannedQuantity   float64 `json:"planned_quantity"`
	RequiredQuantity  float64 `json:"required_quantity"`
	ProducedQuantity  float64 `json:"produced_quantity"`
	SetsPerCarton     float64 `json:"sets_per_carton"`
	CartonCount       float64 `json:"carton_count"`
	Unit              string  `json:"unit"`
	Remark            string  `json:"remark"`
}

// ProductionOrderListItem 生产单列表项。
type ProductionOrderListItem struct {
	ID                   uint64    `json:"id"`
	PONumber             string    `json:"po_number"`
	SourceOrderID        *uint64   `json:"source_order_id"`
	SourceOrderNumber    string    `json:"source_order_number"`
	SourcePurchaseOrderID *uint64  `json:"source_purchase_order_id"`
	SourcePurchaseNumber string    `json:"source_purchase_number"`
	PlanDate             Date      `json:"plan_date"`
	StartDate         Date      `json:"start_date"`
	FinishDate        Date      `json:"finish_date"`
	Status            string    `json:"status"`
	// WorkType 作业类型（produce/pack/none）。
	WorkType          string    `json:"work_type"`
	// WorkTypeLabel 作业类型中文名（展示用）。
	WorkTypeLabel     string    `json:"work_type_label"`
	ItemCount         int       `json:"item_count"`
	CompletedCount    int       `json:"completed_count"`
	TotalCartons      float64   `json:"total_cartons"`
	CreatedAt         time.Time `json:"created_at"`
}

// ProductionCompleteItem 单行完工数量。
type ProductionCompleteItem struct {
	ID               uint64  `json:"id"`
	ProducedQuantity float64 `json:"produced_quantity"`
}

// ProductionCompleteRequest 完工登记请求。
type ProductionCompleteRequest struct {
	Items []ProductionCompleteItem `json:"items"`
}

// ProductionOrderSuggestion 由订单生成生产单时的建议（表头 + 明细）。
type ProductionOrderSuggestion struct {
	OrderID     uint64                `json:"order_id"`
	OrderNumber string                `json:"order_number"`
	Customer    string                `json:"customer"`
	Country     string                `json:"country"`
	Items       []ProductionOrderItem `json:"items"`
}

// RuleComponent 组合规则的组成单品（用于完工时按套数展开成品入库）。
type RuleComponent struct {
	RuleID         uint64  `json:"rule_id"`
	SkuID          uint64  `json:"sku_id"`
	SkuCode        string  `json:"sku_code"`
	ProductName    string  `json:"product_name"`
	ProductNameZH  string  `json:"product_name_zh"`
	QuantityPerSet float64 `json:"quantity_per_set"`
}

// ProductionMaterialSummary 生产报表行：多张生产单合计的材料需求量。
type ProductionMaterialSummary struct {
	MaterialID       uint64  `json:"material_id"`
	MaterialCode     string  `json:"material_code"`
	MaterialName     string  `json:"material_name"`
	Spec             string  `json:"spec"`
	Unit             string  `json:"unit"`
	Category         string  `json:"category"`
	IsPackaging      bool    `json:"is_packaging"`
	RequiredQuantity float64 `json:"required_quantity"`
	ProductCount     int     `json:"product_count"`
}

// PackInfo 产品装箱信息（下推纸箱用料时使用）。
//
// 组合套取 product_match_rules 的箱规，单品取 skus 的箱规；
// 纸箱材料规格 = 长 × 宽 × 高，用量 = 1 / 每箱数量（按套/个折算成箱）。
type PackInfo struct {
	ProductType   string  `json:"product_type"`
	ProductRefID  uint64  `json:"product_ref_id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	PackPerCarton float64 `json:"pack_per_carton"`
	Length        float64 `json:"length"`
	Width         float64 `json:"width"`
	Height        float64 `json:"height"`
	Unit          string  `json:"unit"`
}

// PackKey 返回装箱信息的键（"类型:ID"）。
func PackKey(productType string, refID uint64) string {
	return productType + ":" + fmt.Sprintf("%d", refID)
}

// CartonSpec 返回纸箱规格文本（如 40×30×25cm），尺寸为空时返回空串。
func (p PackInfo) CartonSpec() string {
	if p.Length <= 0 && p.Width <= 0 && p.Height <= 0 {
		return ""
	}
	unit := p.Unit
	if unit == "" {
		unit = "cm"
	}
	return fmt.Sprintf(
		"%s×%s×%s%s",
		formatPackNumber(p.Length), formatPackNumber(p.Width),
		formatPackNumber(p.Height), unit,
	)
}

func formatPackNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
