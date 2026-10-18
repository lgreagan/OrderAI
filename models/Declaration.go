package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ============================================================
// 出口报关资料（货代 xls 模板）领域模型
//
// 模板本身来自货代，系统只负责按模板映射把系统里已有的数据填进去，
// 模板上不变的信息（货代抬头、标签、下拉、辅助表、样式）一律保留。
// ============================================================

// ProductCategory 报关品类：装载报关资料上的申报要素。
//
// 报关资料按「发货明细」逐行出行（一行 = 一条发货明细），
// 品名/材质/海关编码/用途/链接/申报单价等申报要素取本表。
type ProductCategory struct {
	ID           uint64  `json:"id"`
	Code         string  `json:"code"`
	NameEN       string  `json:"name_en"` // 产品英文品名
	NameZH       string  `json:"name_zh"` // 产品中文品名
	Material     string  `json:"material"`
	HSCode       string  `json:"hs_code"`
	Purpose      string  `json:"purpose"`
	Brand        string  `json:"brand"`
	Model        string  `json:"model"`
	ProductURL   string  `json:"product_url"`
	DeclarePrice float64 `json:"declare_price"` // 默认申报单价
	Image        string  `json:"image"`
	Remark       string  `json:"remark"`
	Status       string  `json:"status"`

	// SKUCount 关联的 SKU 数（列表展示用）。
	SKUCount int `json:"sku_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WarehouseAddress 仓库地址库（FBA 仓库地址）：填地址库编码即带出收件人信息。
type WarehouseAddress struct {
	ID        uint64 `json:"id"`
	ShortName string `json:"short_name"` // 地址简称（与模板下拉一致）
	Code      string `json:"code"`
	FBACode   string `json:"fba_code"`
	Contact   string `json:"contact"`
	Company   string `json:"company"`
	Tel       string `json:"tel"`
	Mobile    string `json:"mobile"`
	Address1  string `json:"address1"`
	Address2  string `json:"address2"`
	Address3  string `json:"address3"`
	City      string `json:"city"`
	State     string `json:"state"`
	Country   string `json:"country"`
	Zip       string `json:"zip"`

	// DeclareExtra 报关附注多行文本（模板里的 Anmelder:/Declarant: 整段）。
	DeclareExtra string `json:"declare_extra"`
	VatNo        string `json:"vat_no"`
	Status       string `json:"status"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ShippingChannel 货代渠道字典（与 shipments.shipping_method 对齐）。
type ShippingChannel struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Country   string `json:"country"`
	SortOrder int    `json:"sort_order"`
	Status    string `json:"status"`
}

// DeclarationMapping 报关模板映射：语义字段 → 模板单元格 / 列字母。
//
// 新增模板只需登记一份 mapping，不必改代码。
type DeclarationMapping struct {
	Name    string                   `json:"name"`
	Country string                   `json:"country"`
	Carrier string                   `json:"carrier"`
	File    string                   `json:"file"`  // 模板 xlsx 路径
	Sheet   string                   `json:"sheet"` // 工作表名，空则取第 1 张
	Header  map[string]string        `json:"header"`
	Detail  DeclarationDetailMapping `json:"detail"`
}

// DeclarationDetailMapping 明细区映射。
type DeclarationDetailMapping struct {
	HeaderRow    int               `json:"headerRow"`
	FirstDataRow int               `json:"firstDataRow"`
	SampleRows   []int             `json:"sampleRows"` // [首行, 末行]，模板自带样例行，导出时清除
	Columns      map[string]string `json:"columns"`    // 语义列名 → 列字母
}

// SampleRange 返回样例行的首末行号（缺省时退化为 FirstDataRow）。
func (m DeclarationDetailMapping) SampleRange() (int, int) {
	if len(m.SampleRows) >= 2 && m.SampleRows[0] > 0 && m.SampleRows[1] >= m.SampleRows[0] {
		return m.SampleRows[0], m.SampleRows[1]
	}
	return m.FirstDataRow, m.FirstDataRow
}

// DeclarationTemplate 报关模板登记。
type DeclarationTemplate struct {
	ID         uint64    `json:"id"`
	Name       string    `json:"name"`
	Country    string    `json:"country"`
	Carrier    string    `json:"carrier"`
	ShipType   string    `json:"ship_type"`
	FilePath   string    `json:"file_path"`
	SourceFile string    `json:"source_file"`
	Mapping    string    `json:"mapping"`
	Remark     string    `json:"remark"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ParseMapping 解析模板映射 JSON。
func (t *DeclarationTemplate) ParseMapping() (*DeclarationMapping, error) {
	return ParseDeclarationMapping(t.Mapping)
}

// ParseDeclarationMapping 解析映射 JSON 并做基础校验。
func ParseDeclarationMapping(raw string) (*DeclarationMapping, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("模板映射为空")
	}

	m := &DeclarationMapping{}
	if err := json.Unmarshal([]byte(raw), m); err != nil {
		return nil, fmt.Errorf("解析模板映射失败: %w", err)
	}
	if strings.TrimSpace(m.File) == "" {
		return nil, errors.New("模板映射缺少 file（模板文件路径）")
	}
	if m.Detail.FirstDataRow <= 0 {
		return nil, errors.New("模板映射缺少 detail.firstDataRow（明细起始行）")
	}
	if len(m.Detail.Columns) == 0 {
		return nil, errors.New("模板映射缺少 detail.columns（明细列映射）")
	}
	return m, nil
}

// DeclarationLine 报关明细行（一个品类一行）。
type DeclarationLine struct {
	CategoryID   uint64  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	NameEN       string  `json:"name_en"`
	NameZH       string  `json:"name_zh"`
	Material     string  `json:"material"`
	HSCode       string  `json:"hs_code"`
	Purpose      string  `json:"purpose"`
	Brand        string  `json:"brand"`
	Model        string  `json:"model"`
	ProductURL   string  `json:"product_url"`
	DeclarePrice float64 `json:"declare_price"`
	SalePrice    float64 `json:"sale_price"`

	Quantity     float64 `json:"quantity"`       // 该品类合计数量
	CartonCount  float64 `json:"carton_count"`   // 该品类合计箱数
	QtyPerCarton float64 `json:"qty_per_carton"` // 单箱申报数量
	DeclareTotal float64 `json:"declare_total"`  // 申报总价

	CartonNo    string  `json:"carton_no"` // 货箱编号（组内去重拼接）
	PO          string  `json:"po"`
	GrossWeight float64 `json:"gross_weight"`
	Length      float64 `json:"length"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Weight      float64 `json:"weight"`

	ASIN    string `json:"asin"`
	FNSKU   string `json:"fnsku"`
	SKUCode string `json:"sku_code"`
	Image   string `json:"image"`

	// SKUCount 组内 SKU 数；ItemCount 组内发货明细行数。
	SKUCount  int `json:"sku_count"`
	ItemCount int `json:"item_count"`
}

// DeclarationRecord 报关资料导出留档：每次导出存一份当时的快照，便于事后复现。
type DeclarationRecord struct {
	ID           uint64    `json:"id"`
	ShipmentID   uint64    `json:"shipment_id"`
	TemplateID   uint64    `json:"template_id"`
	TemplateName string    `json:"template_name"`
	OrderNo      string    `json:"order_no"`
	FileName     string    `json:"file_name"`
	LineCount    int       `json:"line_count"`
	CartonSum    float64   `json:"carton_sum"`
	Doc          string    `json:"doc"` // 导出时的 DeclarationDoc 快照（JSON）
	CreatedAt    time.Time `json:"created_at"`
}

// DeclarationDoc 一份待生成的报关资料（抬头 + 明细行）。
type DeclarationDoc struct {
	ShipmentID   uint64 `json:"shipment_id"`
	TemplateID   uint64 `json:"template_id"`
	TemplateName string `json:"template_name"`
	Sheet        string `json:"sheet"`

	OrderNo   string  `json:"order_no"`
	Service   string  `json:"service"`
	AddrCode  string  `json:"addr_code"`
	CartonSum float64 `json:"carton_sum"`
	PO        string  `json:"po"`

	RecipientName    string `json:"recipient_name"`
	RecipientCompany string `json:"recipient_company"`
	RecipientAddr1   string `json:"recipient_addr1"`
	RecipientAddr2   string `json:"recipient_addr2"`
	RecipientAddr3   string `json:"recipient_addr3"`
	RecipientCity    string `json:"recipient_city"`
	RecipientState   string `json:"recipient_state"`
	RecipientZip     string `json:"recipient_zip"`
	RecipientCountry string `json:"recipient_country"`
	RecipientTel     string `json:"recipient_tel"`
	RecipientEmail   string `json:"recipient_email"`

	AmazonInternalCode string `json:"amazon_internal_code"`
	VatNo              string `json:"vat_no"`
	EoriNo             string `json:"eori_no"`
	VatCountry         string `json:"vat_country"`
	VatCompanyEN       string `json:"vat_company_en"`
	VatAddress         string `json:"vat_address"`

	CustomsMode   string `json:"customs_mode"`
	ClearanceMode string `json:"clearance_mode"`
	TaxMode       string `json:"tax_mode"`
	DeliveryTerms string `json:"delivery_terms"`
	Currency      string `json:"currency"`
	Electric      bool   `json:"electric"`
	Magnetic      bool   `json:"magnetic"`
	Liquid        bool   `json:"liquid"`
	Powder        bool   `json:"powder"`
	Dangerous     bool   `json:"dangerous"`
	NeedInsurance bool   `json:"need_insurance"`
	Remark        string `json:"remark"`

	Lines []DeclarationLine `json:"lines"`

	// Warnings 装配过程中的提醒（如未挂品类的 SKU）。
	Warnings []string `json:"warnings"`
}
