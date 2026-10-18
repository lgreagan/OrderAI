package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

// DeclarationService 把发货单装配成一份报关资料：一条发货明细出一行。
type DeclarationService struct {
	Shipments  *repository.ShipmentRepository
	Decl       *repository.DeclarationRepository
	Templates  *repository.DeclarationTemplateRepository
	Warehouses *repository.WarehouseAddressRepository
}

func NewDeclarationService(
	shipments *repository.ShipmentRepository,
	decl *repository.DeclarationRepository,
	templates *repository.DeclarationTemplateRepository,
	warehouses *repository.WarehouseAddressRepository,
) *DeclarationService {
	return &DeclarationService{
		Shipments:  shipments,
		Decl:       decl,
		Templates:  templates,
		Warehouses: warehouses,
	}
}

// unclassifiedName 未挂报关品类的 SKU 归入该行。
const unclassifiedName = "未分类（请为 SKU 指定报关品类）"

// ListTemplates 返回可选的报关模板列表（模板下拉用）。
func (s *DeclarationService) ListTemplates(
	ctx context.Context,
) ([]models.DeclarationTemplate, error) {
	return s.Templates.List(ctx)
}

// GetTemplate 取单个报关模板（含映射 JSON）。
func (s *DeclarationService) GetTemplate(
	ctx context.Context,
	id uint64,
) (*models.DeclarationTemplate, error) {
	if id == 0 {
		return nil, fmt.Errorf("报关模板 ID 无效")
	}
	return s.Templates.GetByID(ctx, id)
}

// SaveTemplateMapping 保存模板映射（调用方需先校验 JSON 合法性）。
func (s *DeclarationService) SaveTemplateMapping(
	ctx context.Context,
	id uint64,
	mapping string,
) error {
	if id == 0 {
		return fmt.Errorf("报关模板 ID 无效")
	}
	if _, err := models.ParseDeclarationMapping(mapping); err != nil {
		return err
	}
	return s.Templates.UpdateMapping(ctx, id, mapping)
}

// SaveRecord 留档一次导出：把导出那一刻的报关资料快照存库，便于事后复现。
func (s *DeclarationService) SaveRecord(
	ctx context.Context,
	doc *models.DeclarationDoc,
	fileName string,
) (*models.DeclarationRecord, error) {
	if doc == nil || doc.ShipmentID == 0 {
		return nil, fmt.Errorf("报关资料数据为空")
	}

	snapshot, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("序列化报关资料快照失败: %w", err)
	}

	rec := &models.DeclarationRecord{
		ShipmentID:   doc.ShipmentID,
		TemplateID:   doc.TemplateID,
		TemplateName: doc.TemplateName,
		OrderNo:      doc.OrderNo,
		FileName:     fileName,
		LineCount:    len(doc.Lines),
		CartonSum:    doc.CartonSum,
		Doc:          string(snapshot),
	}
	if err := s.Decl.InsertRecord(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// ListRecords 取某发货单的历史导出留档（时间倒序）。
func (s *DeclarationService) ListRecords(
	ctx context.Context,
	shipmentID uint64,
	limit int,
) ([]models.DeclarationRecord, error) {
	if shipmentID == 0 {
		return nil, fmt.Errorf("发货单 ID 无效")
	}
	return s.Decl.ListRecords(ctx, shipmentID, limit)
}

// Assemble 装配报关资料：抬头取发货单 + 订单 + 仓库地址库，明细按品类聚合。
//
// templateID 为 0 时优先用发货单上记录的模板，其次按目的国兜底。
func (s *DeclarationService) Assemble(
	ctx context.Context,
	shipmentID, templateID uint64,
) (*models.DeclarationDoc, error) {
	if shipmentID == 0 {
		return nil, fmt.Errorf("发货单 ID 无效")
	}

	shipment, err := s.Shipments.GetByID(ctx, shipmentID)
	if err != nil {
		return nil, err
	}

	tpl, err := s.resolveTemplate(ctx, shipment, templateID)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, fmt.Errorf("没有可用的报关模板，请先在「报关模板」里登记")
	}
	mapping, err := tpl.ParseMapping()
	if err != nil {
		return nil, err
	}

	items, err := s.Decl.ListSKUItems(ctx, shipmentID)
	if err != nil {
		return nil, err
	}

	doc := &models.DeclarationDoc{
		ShipmentID:   shipmentID,
		TemplateID:   tpl.ID,
		TemplateName: tpl.Name,
		Sheet:        strings.TrimSpace(mapping.Sheet),
		OrderNo:      strings.TrimSpace(shipment.OrderNumber),
		Service:      strings.TrimSpace(shipment.ShippingMethod),
		AddrCode:     strings.TrimSpace(shipment.WarehouseCode),
		PO:           strings.TrimSpace(shipment.PONumber),

		AmazonInternalCode: strings.TrimSpace(shipment.WarehouseCode),
		CustomsMode:        strings.TrimSpace(shipment.CustomsMode),
		TaxMode:            strings.TrimSpace(shipment.TaxMode),
		Currency:           strings.TrimSpace(shipment.DeclareCurrency),
		Electric:           shipment.IsElectric,
		Magnetic:           shipment.IsMagnetic,
		Liquid:             shipment.IsLiquid,
		Powder:             shipment.IsPowder,
		Dangerous:          shipment.IsDanger,
		NeedInsurance:      shipment.NeedInsurance,
		Remark:             strings.TrimSpace(shipment.Remark),
	}

	if doc.OrderNo == "" {
		doc.OrderNo = shipment.ShipmentNumber
	}
	if doc.Currency == "" {
		doc.Currency = "USD"
	}

	if err := s.fillRecipient(ctx, doc); err != nil {
		return nil, err
	}

	// 明细按「发货明细」逐行出行：一行 = 一条发货明细，海关能看清每箱装的是什么。
	unclassified := 0
	for i := range items {
		line := buildLineFromItem(&items[i])
		if items[i].CategoryID == 0 {
			unclassified++
		}
		doc.Lines = append(doc.Lines, line)
	}
	if unclassified > 0 {
		doc.Warnings = append(doc.Warnings, fmt.Sprintf(
			"有 %d 条发货明细的 SKU 未指定报关品类，已归入「%s」", unclassified, unclassifiedName))
	}

	// 总箱数单独算：套装的组成单品分属不同品类，按行累加会把同一箱算两次。
	doc.CartonSum = cartonSumOfLines(doc.Lines)

	if len(doc.Lines) == 0 {
		doc.Warnings = append(doc.Warnings, "该发货单没有产品类明细，报关资料明细为空")
	}
	return doc, nil
}

// resolveTemplate 依次尝试：显式指定 → 发货单上记录的模板 → 按目的国取第一个。
func (s *DeclarationService) resolveTemplate(
	ctx context.Context,
	shipment *models.Shipment,
	explicitID uint64,
) (*models.DeclarationTemplate, error) {
	if explicitID != 0 {
		return s.Templates.GetByID(ctx, explicitID)
	}
	if shipment.TemplateID != 0 {
		if tpl, err := s.Templates.GetByID(ctx, shipment.TemplateID); err == nil && tpl != nil {
			return tpl, nil
		}
	}
	if country := strings.TrimSpace(shipment.Country); country != "" {
		return s.Templates.GetByCountry(ctx, country)
	}
	return nil, nil
}

// fillRecipient 用「地址库编码」从仓库地址库带出收件人信息；带不出的保留模板原值。
//
// 收件人地址一是多行：报关附注（declare_extra，如 "Anmelder: POLZEN LTD" 整段）
// 加上地址一/二/三。模板（DE/UK 海运）里该格原本就写着货代的申报说明块，
// 因此没有对应地址库记录时不动模板，由业务员核对后补填。
func (s *DeclarationService) fillRecipient(
	ctx context.Context,
	doc *models.DeclarationDoc,
) error {
	if doc.AddrCode == "" {
		doc.Warnings = append(doc.Warnings,
			"发货单未填「地址库编码」，收件人信息沿用模板原样，请核对")
		return nil
	}

	addr, err := s.Warehouses.GetByShortName(ctx, doc.AddrCode)
	if err != nil {
		return err
	}
	if addr == nil {
		doc.Warnings = append(doc.Warnings,
			fmt.Sprintf("仓库地址库里没有编码「%s」，收件人信息请手工填写", doc.AddrCode))
		return nil
	}

	if name := strings.TrimSpace(addr.Contact); name != "" {
		doc.RecipientName = name
	} else {
		doc.RecipientName = addr.Company
	}
	doc.RecipientCompany = addr.Company
	doc.RecipientAddr1 = joinAddressLines(
		addr.DeclareExtra, addr.Address1, addr.Address2, addr.Address3,
	)
	doc.RecipientAddr2 = ""
	doc.RecipientAddr3 = ""
	doc.RecipientCity = addr.City
	doc.RecipientState = addr.State
	doc.RecipientZip = addr.Zip
	doc.RecipientCountry = addr.Country
	doc.RecipientTel = addr.Tel
	doc.VatNo = addr.VatNo
	if doc.RecipientCountry == "" {
		doc.RecipientCountry = addr.Code
	}
	return nil
}

// joinAddressLines 把多段地址拼成多行文本，空段自动跳过。
func joinAddressLines(parts ...string) string {
	lines := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			lines = append(lines, p)
		}
	}
	return strings.Join(lines, "\n")
}

// ============================================================
// 明细出行：一条发货明细 = 一行
// ============================================================

// buildLineFromItem 把一条发货明细转成一行报关明细。
//
// 申报要素（品名/材质/海关编码/用途/品牌/链接/申报单价）取 SKU 挂的报关品类；
// 数量/箱数/箱号/重量/尺寸取本条明细自身；品类没填型号时用本行规格补位，
// 让海关和货代能逐箱看清装的是哪个规格的产品。
func buildLineFromItem(it *repository.DeclarationItemRow) models.DeclarationLine {
	line := models.DeclarationLine{
		CategoryID: it.CategoryID,
		ItemCount:  1,
		SKUCount:   1,
		SKUCode:    strings.TrimSpace(it.SKUCode),
		ASIN:       strings.TrimSpace(it.SKUASIN),
		FNSKU:      strings.TrimSpace(it.SKUFNSKU),
	}

	line.NameEN = it.CategoryNameEN
	line.NameZH = it.CategoryNameZH
	line.Material = it.CategoryMaterial
	line.HSCode = it.CategoryHSCode
	line.Purpose = it.CategoryPurpose
	line.Brand = it.CategoryBrand
	line.Model = it.CategoryModel
	line.ProductURL = it.CategoryURL
	// 申报单价：SKU 自己填的申报价优先（同 HS 不同规格可各自申报），否则用品类的默认申报价。
	line.DeclarePrice = it.CategoryPrice
	if it.SKUDeclare > 0 {
		line.DeclarePrice = it.SKUDeclare
	}
	line.Image = it.CategoryImage
	line.CategoryName = it.CategoryNameZH

	// 未挂品类的兜底：退回 SKU 自身的中文品名/英文品名/海关编码。
	if it.CategoryID == 0 {
		line.CategoryName = unclassifiedName
		if line.NameZH == "" {
			line.NameZH = it.ProductNameZ
		}
		if line.NameEN == "" {
			line.NameEN = it.ProductName
		}
		if line.HSCode == "" {
			line.HSCode = it.SKUHSCode
		}
	}

	// 品类没填型号时用本行规格补位：同 HS 的多个规格靠这一列区分。
	if m := strings.TrimSpace(line.Model); m == "" || m == "无" {
		line.Model = itemSpec(it)
	}

	// 数量与箱数：箱数优先取发货明细填的，没填时按整箱数量折算。
	qty := it.Quantity
	cartons := it.CartonCount
	if cartons <= 0 && it.PackQuantity > 0 && qty > 0 {
		cartons = math.Ceil(qty / it.PackQuantity)
	}
	line.Quantity = round3(qty)
	line.CartonCount = round3(cartons)

	if s := strings.TrimSpace(it.CartonNo); s != "" {
		line.CartonNo = mergeCartonNumbers([]string{s})
	}
	line.PO = strings.TrimSpace(it.PONumber)

	// 申报单价以品类为准；品类没填价时退回本条明细的单价。
	if line.DeclarePrice <= 0 {
		if it.UnitPrice > 0 {
			line.DeclarePrice = it.UnitPrice
		} else if it.SKUPriceUSD > 0 {
			line.DeclarePrice = it.SKUPriceUSD
		}
	}
	// 销售价格（模板「产品销售价格」列）：取本条明细的成交单价。
	if it.UnitPrice > 0 {
		line.SalePrice = it.UnitPrice
	} else {
		line.SalePrice = it.SKUPriceUSD
	}

	// 箱规与重量：优先取发货明细上的实填值，回退 SKU 的箱规。
	line.GrossWeight = firstNonZero(it.CartonWeight, it.BoxWeight)
	line.Length = firstNonZero(it.CartonLength, it.BoxLength)
	line.Width = firstNonZero(it.CartonWidth, it.BoxWidth)
	line.Height = firstNonZero(it.CartonHeight, it.BoxHeight)
	line.Weight = firstNonZero(it.BoxWeight, it.CartonWeight)

	if line.Image == "" {
		line.Image = it.SKUImage
	}

	if line.CartonCount > 0 {
		line.QtyPerCarton = round3(line.Quantity / line.CartonCount)
	} else {
		line.QtyPerCarton = round3(line.Quantity)
	}
	line.DeclarePrice = round4(line.DeclarePrice)
	line.DeclareTotal = round2(line.DeclarePrice * line.Quantity)

	if strings.TrimSpace(line.Brand) == "" {
		line.Brand = "无"
	}
	if strings.TrimSpace(line.Model) == "" {
		line.Model = "无"
	}
	if strings.TrimSpace(line.Image) == "" {
		line.Image = "无"
	}
	if line.SalePrice <= 0 {
		line.SalePrice = line.DeclarePrice
	}

	return line
}

// itemSpec 返回本条明细的规格描述（型号列兜底用）。
func itemSpec(it *repository.DeclarationItemRow) string {
	for _, v := range []string{it.ShortName, it.ProductNameZ, it.SKUCode} {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func firstNonZero(values ...float64) float64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

// NormalizeDoc 依据明细重算派生字段（单箱数量、申报总价、总箱数）。
//
// 预览页允许业务员改数量/箱号，导出前统一走一次归一，保证写进 Excel 的数字自洽。
func NormalizeDoc(doc *models.DeclarationDoc) {
	if doc == nil {
		return
	}

	for i := range doc.Lines {
		l := &doc.Lines[i]
		if l.Quantity < 0 {
			l.Quantity = 0
		}
		if l.CartonCount < 0 {
			l.CartonCount = 0
		}
		if l.CartonCount > 0 {
			l.QtyPerCarton = round3(l.Quantity / l.CartonCount)
		} else {
			l.QtyPerCarton = round3(l.Quantity)
		}
		l.DeclarePrice = round4(l.DeclarePrice)
		l.DeclareTotal = round2(l.DeclarePrice * l.Quantity)
		if strings.TrimSpace(l.Brand) == "" {
			l.Brand = "无"
		}
		if strings.TrimSpace(l.Model) == "" {
			l.Model = "无"
		}
		if strings.TrimSpace(l.Image) == "" {
			l.Image = "无"
		}
		// 导出前再合并一次箱号区间（幂等）：预览页回传的可能是旧格式的拼接串，
		// 或者业务员手工补了箱号，统一按连号合并后写进 Excel。
		if s := strings.TrimSpace(l.CartonNo); s != "" {
			l.CartonNo = mergeCartonNumbers([]string{s})
		}
	}
	// 与 Assemble 同口径：有箱号时按箱号并集算，避免套装同箱被算两次。
	doc.CartonSum = cartonSumOfLines(doc.Lines)
}

// uniqueList 保序去重的字符串集合。
type uniqueList struct {
	seen map[string]bool
	list []string
}

func newUniqueList() *uniqueList {
	return &uniqueList{seen: make(map[string]bool)}
}

func (u *uniqueList) add(value string) {
	v := strings.TrimSpace(value)
	if v == "" || u.seen[v] {
		return
	}
	u.seen[v] = true
	u.list = append(u.list, v)
}

func (u *uniqueList) join(sep string) string {
	return strings.Join(u.list, sep)
}

func (u *uniqueList) len() int { return len(u.list) }

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
