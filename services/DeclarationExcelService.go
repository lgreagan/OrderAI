package services

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"

	"github.com/xuri/excelize/v2"
)

// DeclarationExcelService 按货代模板生成出口报关资料（Excel）。
//
// 生成方式与 PiExcelService 一致：以模板 xlsx 为基础，
//   - 保留模板上不变的信息：货代抬头、标签、数据验证（下拉）、辅助表、列宽行高、合并单元格、字体边框；
//   - 只清除模板自带的「样例行」（值与超链接、样例产品图），然后按映射写入系统数据；
//   - 系统里没有的数据一律不写（保持模板原样或留空）。
type DeclarationExcelService struct {
	Templates *repository.DeclarationTemplateRepository
}

func NewDeclarationExcelService(
	templates *repository.DeclarationTemplateRepository,
) *DeclarationExcelService {
	return &DeclarationExcelService{Templates: templates}
}

// maxDetailProbeCol 探测明细表头列数时的最大列（模板最宽 21 列，留足余量）。
const maxDetailProbeCol = 64

// Build 生成报关资料工作簿字节流。
func (s *DeclarationExcelService) Build(ctx context.Context, doc *models.DeclarationDoc) ([]byte, error) {
	if doc == nil || doc.ShipmentID == 0 {
		return nil, fmt.Errorf("报关资料数据为空")
	}

	tpl, err := s.Templates.GetByID(ctx, doc.TemplateID)
	if err != nil {
		return nil, err
	}
	mapping, err := tpl.ParseMapping()
	if err != nil {
		return nil, err
	}

	f, err := excelize.OpenFile(mapping.File)
	if err != nil {
		return nil, fmt.Errorf("打开报关模板 %s 失败: %w", mapping.File, err)
	}
	defer f.Close()

	fullCalc := true
	calcMode := "auto"
	_ = f.SetCalcProps(&excelize.CalcPropsOptions{
		FullCalcOnLoad: &fullCalc,
		CalcMode:       &calcMode,
	})

	sheet := strings.TrimSpace(mapping.Sheet)
	if sheet == "" {
		sheet = f.GetSheetName(0)
	}
	if sheet == "" {
		sheet = "Sheet1"
	}
	doc.Sheet = sheet

	detail := mapping.Detail
	firstRow := detail.FirstDataRow
	sampleStart, sampleEnd := detail.SampleRange()
	if sampleStart <= 0 {
		sampleStart = firstRow
		sampleEnd = firstRow
	}

	lastCol := detailColumnCount(f, sheet, detail.HeaderRow, detail.Columns)

	// 样例行样式母版：清除前先取好样式与行高。
	colStyles := make(map[int]int, lastCol)
	for c := 1; c <= lastCol; c++ {
		id, err := f.GetCellStyle(sheet, cellRef(sampleStart, c))
		if err == nil {
			colStyles[c] = id
		}
	}
	sampleHeight, err := f.GetRowHeight(sheet, sampleStart)
	if err != nil || sampleHeight <= 0 {
		sampleHeight = 20
	}

	// 1) 删除模板样例产品图（明细区里的图片），货代抬头区的图片保留。
	if cells, err := f.GetPictureCells(sheet); err == nil {
		for _, cell := range cells {
			if r, ok := rowOf(cell); ok && r >= firstRow {
				_ = f.DeletePicture(sheet, cell)
			}
		}
	}

	// 2) 清空样例行（值 + 超链接），样式与下拉保持不变。
	for r := sampleStart; r <= sampleEnd; r++ {
		clearRow(f, sheet, r, lastCol)
	}

	// 3) 写抬头：只覆盖系统里确实有值的字段，其余保留模板原值/留空。
	writeDeclarationHeader(f, sheet, doc, mapping.Header)

	// 4) 明细行数超过样例行数时，在样例行之后插入新行并克隆母版样式。
	lines := doc.Lines
	available := sampleEnd - sampleStart + 1
	if extra := len(lines) - available; extra > 0 {
		if err := f.InsertRows(sheet, sampleEnd+1, extra); err != nil {
			return nil, fmt.Errorf("插入报关明细行失败: %w", err)
		}
		for r := sampleEnd + 1; r <= sampleEnd+extra; r++ {
			applyRowTemplate(f, sheet, r, lastCol, colStyles, sampleHeight)
		}
	}

	// 5) 逐行写明细。
	for i, line := range lines {
		row := firstRow + i
		writeDeclarationLine(f, sheet, row, line, detail.Columns, lastCol, colStyles)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// detailColumnCount 取明细表实际用到的列数：映射列与模板表头行的非空列取较大者。
func detailColumnCount(
	f *excelize.File,
	sheet string,
	headerRow int,
	columns map[string]string,
) int {
	last := 0
	for _, letter := range columns {
		if idx, err := excelize.ColumnNameToNumber(letter); err == nil && idx > last {
			last = idx
		}
	}
	if headerRow > 0 {
		rows, err := f.GetRows(sheet)
		if err == nil && len(rows) >= headerRow {
			for i, v := range rows[headerRow-1] {
				if strings.TrimSpace(v) != "" && i+1 > last {
					last = i + 1
				}
			}
		}
	}
	if last <= 0 {
		last = 1
	}
	if last > maxDetailProbeCol {
		last = maxDetailProbeCol
	}
	return last
}

// clearRow 清空一行的值与超链接（保留样式）。
func clearRow(f *excelize.File, sheet string, row, lastCol int) {
	_ = f.SetCellStyle(sheet, cellRef(row, 1), cellRef(row, lastCol), 0)
	for c := 1; c <= lastCol; c++ {
		cell := cellRef(row, c)
		if ok, _, err := f.GetCellHyperLink(sheet, cell); err == nil && ok {
			_ = f.SetCellHyperLink(sheet, cell, "", "None")
		}
		_ = f.SetCellValue(sheet, cell, nil)
	}
}

// applyRowTemplate 把样例行（母版）的列样式与行高套到新插入的行上。
func applyRowTemplate(
	f *excelize.File,
	sheet string,
	row, lastCol int,
	colStyles map[int]int,
	height float64,
) {
	for c := 1; c <= lastCol; c++ {
		if id, ok := colStyles[c]; ok && id > 0 {
			_ = f.SetCellStyle(sheet, cellRef(row, c), cellRef(row, c), id)
		}
	}
	if height > 0 {
		_ = f.SetRowHeight(sheet, row, height)
	}
}

// writeDeclarationHeader 按映射写抬头。
func writeDeclarationHeader(
	f *excelize.File,
	sheet string,
	doc *models.DeclarationDoc,
	header map[string]string,
) {
	for key, cell := range header {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		value, ok := declarationHeaderValue(doc, key)
		if !ok {
			continue
		}
		_ = f.SetCellValue(sheet, cell, value)
	}
}

// declarationHeaderValue 返回抬头语义字段的取值；第二个返回值为 false 表示不覆盖模板。
func declarationHeaderValue(doc *models.DeclarationDoc, key string) (any, bool) {
	str := func(v string) (any, bool) {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, false
		}
		return v, true
	}
	flag := func(v bool) (any, bool) {
		if !v {
			return nil, false
		}
		return "是", true
	}

	switch key {
	case "order_no":
		return str(doc.OrderNo)
	case "service":
		return str(doc.Service)
	case "addr_code":
		return str(doc.AddrCode)
	case "recipient_name":
		return str(doc.RecipientName)
	case "recipient_company":
		return str(doc.RecipientCompany)
	case "recipient_addr1":
		return str(doc.RecipientAddr1)
	case "recipient_addr2":
		return str(doc.RecipientAddr2)
	case "recipient_addr3":
		return str(doc.RecipientAddr3)
	case "recipient_city":
		return str(doc.RecipientCity)
	case "recipient_state":
		return str(doc.RecipientState)
	case "recipient_zip":
		return str(doc.RecipientZip)
	case "recipient_country":
		return str(doc.RecipientCountry)
	case "recipient_tel":
		return str(doc.RecipientTel)
	case "recipient_email":
		return str(doc.RecipientEmail)
	case "amazon_internal_code":
		return str(doc.AmazonInternalCode)
	case "po_number":
		return str(doc.PO)
	case "carton_count":
		if doc.CartonSum <= 0 {
			return nil, false
		}
		return trimNumber(doc.CartonSum), true
	case "electric":
		return flag(doc.Electric)
	case "magnetic":
		return flag(doc.Magnetic)
	case "liquid":
		return flag(doc.Liquid)
	case "powder":
		return flag(doc.Powder)
	case "danger":
		return flag(doc.Dangerous)
	case "insurance_flag":
		return flag(doc.NeedInsurance)
	case "customs_mode":
		return str(doc.CustomsMode)
	case "clearance_mode":
		return str(doc.ClearanceMode)
	case "tax_mode":
		return str(doc.TaxMode)
	case "delivery_terms":
		return str(doc.DeliveryTerms)
	case "vat_no":
		return str(doc.VatNo)
	case "eori_no":
		return str(doc.EoriNo)
	case "vat_country":
		return str(doc.VatCountry)
	case "vat_company_en":
		return str(doc.VatCompanyEN)
	case "vat_address":
		return str(doc.VatAddress)
	case "currency":
		return str(doc.Currency)
	case "remark":
		return str(doc.Remark)
	}
	return nil, false
}

// writeDeclarationLine 写一行报关明细。
func writeDeclarationLine(
	f *excelize.File,
	sheet string,
	row int,
	line models.DeclarationLine,
	columns map[string]string,
	lastCol int,
	colStyles map[int]int,
) {
	// 逐列按语义键写入；映射里没有的键保持空白。
	for key, letter := range columns {
		col, err := excelize.ColumnNameToNumber(strings.TrimSpace(letter))
		if err != nil || col <= 0 {
			continue
		}

		if key == "image" {
			continue // 图片单独处理（见下）
		}

		value, ok := declarationLineValue(line, key)
		if !ok {
			continue
		}
		_ = f.SetCellValue(sheet, cellRef(row, col), value)
	}

	// 行样式：套用母版样式，保持与模板一致。
	for c := 1; c <= lastCol; c++ {
		if id, ok := colStyles[c]; ok && id > 0 {
			_ = f.SetCellStyle(sheet, cellRef(row, c), cellRef(row, c), id)
		}
	}

	// 产品图片：写图；无图或文件缺失时写「无」。
	letter, hasImageCol := columns["image"]
	if !hasImageCol {
		return
	}
	col, err := excelize.ColumnNameToNumber(strings.TrimSpace(letter))
	if err != nil || col <= 0 {
		return
	}

	path := resolvePIImagePath(line.Image)
	if path == "" {
		if strings.TrimSpace(line.Image) == "" {
			_ = f.SetCellValue(sheet, cellRef(row, col), "无")
		} else {
			_ = f.SetCellValue(sheet, cellRef(row, col), line.Image)
		}
		return
	}
	insertDeclarationImage(f, sheet, row, col, path)
}

// declarationLineValue 返回明细语义字段的取值；false 表示该字段无值。
func declarationLineValue(line models.DeclarationLine, key string) (any, bool) {
	str := func(v string) (any, bool) {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, false
		}
		return v, true
	}
	num := func(v float64) (any, bool) {
		if v == 0 {
			return nil, false
		}
		return trimNumber(v), true
	}

	switch key {
	case "carton_no":
		return str(line.CartonNo)
	case "po_number":
		return str(line.PO)
	case "gross_weight":
		return num(line.GrossWeight)
	case "weight":
		return num(line.Weight)
	case "length":
		return num(line.Length)
	case "width":
		return num(line.Width)
	case "height":
		return num(line.Height)
	case "name_en":
		return str(line.NameEN)
	case "name_zh":
		return str(line.NameZH)
	case "declare_price":
		return num(line.DeclarePrice)
	case "declare_qty":
		// DE 模板：申报数量是一格文本，形如 "321(321/箱，1箱）"。
		qty, ok := num(line.Quantity)
		if !ok {
			return nil, false
		}
		return fmt.Sprintf("%v(%v/箱，%v箱）",
			qty, trimNumber(line.QtyPerCarton), trimNumber(line.CartonCount)), true
	case "qty_per_carton":
		return num(line.QtyPerCarton)
	case "declare_total":
		return num(line.DeclareTotal)
	case "sale_price":
		return num(line.SalePrice)
	case "material":
		return str(line.Material)
	case "hs_code":
		return str(line.HSCode)
	case "purpose":
		return str(line.Purpose)
	case "brand":
		return str(line.Brand)
	case "model":
		return str(line.Model)
	case "url":
		return str(line.ProductURL)
	case "asin":
		return str(line.ASIN)
	case "fnsku":
		return str(line.FNSKU)
	case "sku_code":
		return str(line.SKUCode)
	}
	return nil, false
}

// insertDeclarationImage 把产品图按行高缩放居中插入图片列。
func insertDeclarationImage(
	f *excelize.File,
	sheet string,
	row, col int,
	path string,
) {
	width, height, ok := imagePixelSize(path)
	if !ok || width <= 0 || height <= 0 {
		_ = f.SetCellValue(sheet, cellRef(row, col), "无")
		return
	}

	rowHeight, err := f.GetRowHeight(sheet, row)
	if err != nil || rowHeight <= 0 {
		rowHeight = 43
	}
	colWidth, err := f.GetColWidth(sheet, colName(col))
	if err != nil || colWidth <= 0 {
		colWidth = 8.43
	}

	colPx := colWidth*7 + 5
	rowPx := rowHeight * 96 / 72

	boxWidth := colPx - 4
	boxHeight := rowPx - 4
	if boxWidth <= 0 || boxHeight <= 0 {
		return
	}

	scale := math.Min(boxWidth/float64(width), boxHeight/float64(height))
	scale = math.Floor(scale*100) / 100
	if scale <= 0 {
		scale = 0.01
	}
	offsetX := int((colPx - float64(width)*scale) / 2)
	offsetY := int((rowPx - float64(height)*scale) / 2)
	if offsetX < 0 {
		offsetX = 0
	}
	if offsetY < 0 {
		offsetY = 0
	}

	printObject := true
	_ = f.AddPicture(sheet, cellRef(row, col), path, &excelize.GraphicOptions{
		ScaleX:      scale,
		ScaleY:      scale,
		OffsetX:     offsetX,
		OffsetY:     offsetY,
		Positioning: "oneCell",
		PrintObject: &printObject,
	})
}

// trimNumber 把浮点数格式化成去掉多余小数位的字符串。
func trimNumber(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(math.Round(v*1000)/1000, 'f', -1, 64)
}
