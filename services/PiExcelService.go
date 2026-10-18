package services

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"OrderAI/models"

	"github.com/xuri/excelize/v2"
)

// PiExcelService 把一张 PI 导出为 Excel。
type PiExcelService struct {
	Settings     *SettingsService
	TemplatePath string // 模板路径，默认 excel/PI.xlsx（相对运行目录）。
}

func NewPiExcelService(settings *SettingsService) *PiExcelService {
	return &PiExcelService{Settings: settings}
}

const (
	piDefaultTemplatePath = "excel/PI.xlsx"
	// piHeaderRowCount 抬头 + 明细表头占用的模板行数（保留模板第 1..12 行）。
	piHeaderRowCount = 12
	piFirstDataRow   = piHeaderRowCount + 1
	piLastCol        = 8 // A..H

	// 【改动】产品图片列改为 C 列
	piImageCol = 3
	// 【改动】Description 起始和结束列改为 C 和 E（CDE三列合并）
	piDescStartCol = 3
	piDescEndCol   = 5

	// piMarkerCol J列，隐藏，用于 IsPaid 标记 1=未付 0=已付。
	piMarkerCol = 10
	// 【改动】产品图片列（C）列宽（字符），适配缩略图大小，保证不超A4
	piImageColWidth = 12.0
)

// piTemplateStyles 模板各区块的样式 ID（导出时克隆给新写入的单元格）。
type piTemplateStyles struct {
	groupTitle       int     // 分组标题（A:H 合并）
	model            int     // Model 列 (A)
	description      int     // Description（C:E 合并）
	price            int     // 单价 (F)
	qty              int     // 数量 (G)
	amount           int     // 金额 (H)
	subtotalText     int     // 分组小计文字
	subtotalQty      int     // 分组小计数量
	subtotalAmt      int     // 分组小计金额
	sectionLabel     int     // "Box orders" / "Box and item in Stock" 小标题（A:H 合并）
	chargeText       int     // 费用/汇总行文字（A:G 合并）
	chargeAmount     int     // 费用/汇总行金额（H）
	totalText        int     // 合计行文字
	totalAmount      int     // 合计行金额
	note             int     // 底部大段文本块
	dataRowHeight    float64 // 分组标题/明细行
	summaryRowHeight float64 // 小计/汇总/小标题行
	noteRowHeight    float64 // 库存说明文本块
	modelColWidthPx  float64 // Model 列像素宽（A）
	groupBoxWidthPx  float64 // 分组截图的 A:B 区域像素宽。
	imageColWidthPx  float64 // 【改动】产品图片列（C）像素宽。
}

// Build 生成 PI 工作簿字节流。
func (s *PiExcelService) Build(
	ctx context.Context,
	pi *models.ProformaInvoice,
	images map[uint64]string,
	orderImages map[uint64]string,
) ([]byte, error) {
	path := s.templatePath()
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("打开 PI 模板 %s 失败: %w", path, err)
	}
	defer f.Close()

	fullCalc := true
	calcMode := "auto"
	_ = f.SetCalcProps(&excelize.CalcPropsOptions{
		FullCalcOnLoad: &fullCalc,
		CalcMode:       &calcMode,
	})

	sheet := f.GetSheetName(0)
	if sheet == "" {
		sheet = "Sheet1"
	}
	styles := captureTemplateStyles(f, sheet)

	// 【改动】产品图片列按缩略图宽度单独设列宽（C列）
	_ = f.SetColWidth(sheet, "C", "C", piImageColWidth)

	if err := resetTemplateBody(f, sheet); err != nil {
		return nil, err
	}
	writePIHeader(f, sheet, pi)
	writePIBody(f, sheet, pi, styles, s.piTerms(ctx), images, orderImages)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *PiExcelService) templatePath() string {
	if p := strings.TrimSpace(s.TemplatePath); p != "" {
		return p
	}
	return piDefaultTemplatePath
}

func (s *PiExcelService) piTerms(ctx context.Context) string {
	if s.Settings == nil {
		return ""
	}
	return strings.TrimSpace(s.Settings.PITerms(ctx))
}

// captureTemplateStyles 读取模板里各区块的样式 ID，供写入时克隆。
func captureTemplateStyles(f *excelize.File, sheet string) piTemplateStyles {
	at := func(cell string) int {
		id, err := f.GetCellStyle(sheet, cell)
		if err != nil {
			return 0
		}
		return id
	}
	st := piTemplateStyles{
		groupTitle: at("A13"),
		model:      at("A14"),
		// 【改动】Description 样式从模板的 C 列读取（因为现在CDE合并）
		description:  at("C14"),
		price:        at("F14"),
		qty:          at("G14"),
		amount:       at("H14"),
		subtotalText: at("A19"),
		subtotalQty:  at("G19"),
		subtotalAmt:  at("H19"),
		sectionLabel: at("A29"),
		chargeText:   at("A34"),
		chargeAmount: at("H34"),
		totalText:    at("A38"),
		totalAmount:  at("H38"),
		note:         at("C33"),
	}
	st.dataRowHeight = 20
	if h, err := f.GetRowHeight(sheet, 14); err == nil && h > 0 {
		st.dataRowHeight = h
	}
	st.summaryRowHeight = 12.75
	if h, err := f.GetRowHeight(sheet, 19); err == nil && h > 0 {
		st.summaryRowHeight = h
	}
	st.noteRowHeight = 132
	if h, err := f.GetRowHeight(sheet, 33); err == nil && h > 0 {
		st.noteRowHeight = h
	}
	st.modelColWidthPx = 58
	if w, err := f.GetColWidth(sheet, "A"); err == nil && w > 0 {
		st.modelColWidthPx = w*7 + 5
	}
	st.groupBoxWidthPx = st.modelColWidthPx
	if w, err := f.GetColWidth(sheet, "B"); err == nil && w > 0 {
		st.groupBoxWidthPx += w*7 + 5
	}
	// 【改动】计算 C 列的像素宽
	st.imageColWidthPx = piImageColWidth*7 + 5
	return st
}

// resetTemplateBody 清掉模板里的样例明细。
func resetTemplateBody(f *excelize.File, sheet string) error {
	if cells, err := f.GetPictureCells(sheet); err == nil {
		for _, cell := range cells {
			if r, ok := rowOf(cell); ok && r >= piFirstDataRow {
				_ = f.DeletePicture(sheet, cell)
			}
		}
	}
	if merges, err := f.GetMergeCells(sheet, true); err == nil {
		for i := range merges {
			start, ok := rowOf(merges[i].GetStartAxis())
			if !ok || start < piFirstDataRow {
				continue
			}
			_ = f.UnmergeCell(sheet, merges[i].GetStartAxis(), merges[i].GetEndAxis())
		}
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return err
	}
	lastRow := len(rows)
	if lastRow < piHeaderRowCount {
		lastRow = piHeaderRowCount
	}
	for r := piFirstDataRow; r <= lastRow; r++ {
		_ = f.SetCellStyle(sheet, cellRef(r, 1), cellRef(r, piLastCol), 0)
		for c := 1; c <= piMarkerCol; c++ {
			_ = f.SetCellValue(sheet, cellRef(r, c), nil)
		}
	}
	_ = f.SetColVisible(sheet, "J", false)
	return nil
}

func writePIHeader(f *excelize.File, sheet string, pi *models.ProformaInvoice) {
	if number := sanitizePIText(pi.PINumber); number != "" {
		_ = f.SetCellValue(sheet, "H4", number)
	}
	if !pi.IssuedDate.Time.IsZero() {
		_ = f.SetCellValue(sheet, "H5", issuedDateValue(pi.IssuedDate.Time))
	}
	if customer := sanitizePIText(pi.CustomerName); customer != "" {
		_ = f.SetCellValue(sheet, "B7", customer)
	}
}

func issuedDateValue(t time.Time) int {
	value, err := strconv.Atoi(t.Format("20060102"))
	if err != nil {
		return 0
	}
	return value
}

func setCellFormula(f *excelize.File, sheet, cell, formula string) error {
	return f.SetCellFormula(sheet, cell, formula)
}

func strPtr(s string) *string { return &s }

// writePIBody 按分组写明细、小计、箱费、费用汇总与库存说明。
func writePIBody(
	f *excelize.File,
	sheet string,
	pi *models.ProformaInvoice,
	st piTemplateStyles,
	terms string,
	images map[uint64]string,
	orderImages map[uint64]string,
) {
	row := piFirstDataRow
	for _, g := range groupPIItems(pi.Items) {
		f.MergeCell(sheet, cellRef(row, 1), cellRef(row, piLastCol))
		f.SetCellValue(sheet, cellRef(row, 1), sanitizePIText(g.title))
		styleRow(f, sheet, row, st.groupTitle)
		_ = f.SetRowHeight(sheet, row, st.dataRowHeight)
		row++

		groupStartRow := row
		var subQty, subAmount float64
		for _, it := range g.rows {
			writePIItemRow(f, sheet, row, it, st, images)
			if !it.IsPaid {
				subQty += it.Quantity
				subAmount += it.Amount
			}
			row++
		}
		groupEndRow := row - 1

		addPIGroupImage(f, sheet, groupStartRow, groupEndRow, orderImage(orderImages, g), st)

		// 【改动】小计行文字合并 A:E (1~5)，数量 G (7)，公式 H (8)
		f.MergeCell(sheet, cellRef(row, 1), cellRef(row, piDescEndCol))
		f.SetCellValue(sheet, cellRef(row, 1), "Quantity/Item Cost")
		f.SetCellValue(sheet, cellRef(row, 7), subQty)

		subFormula := fmt.Sprintf("=SUMPRODUCT(H%d:H%d)", groupStartRow, groupEndRow)
		_ = setCellFormula(f, sheet, cellRef(row, 8), subFormula)

		styleRow(f, sheet, row, st.subtotalText)
		f.SetCellStyle(sheet, cellRef(row, 7), cellRef(row, 7), st.subtotalQty)
		f.SetCellStyle(sheet, cellRef(row, 8), cellRef(row, 8), st.subtotalAmt)
		_ = f.SetRowHeight(sheet, row, st.summaryRowHeight)
		row++
	}

	// 箱费只汇总成一行总价，不按明细逐条列出。
	if boxAmount := sumUnpaidAmount(filterItems(pi.Items, models.PIItemBox)); boxAmount != 0 {
		writeSummaryRow(f, sheet, row, "box cost", boxAmount, st, false)
		row++
	}

	writeSummaryRow(
		f, sheet, row, "Item Cost + box cost",
		pi.ItemAmount+pi.BoxAmount, st, false,
	)
	row++
	for _, it := range filterItems(pi.Items, models.PIItemCharge) {
		writeSummaryRow(
			f, sheet, row, sanitizePIText(it.Title),
			it.Amount, st, false,
		)
		row++
	}
	writeSummaryRow(f, sheet, row, "Amount", pi.Amount, st, true)
	row++
	if pi.CarryoverAmount != 0 {
		writeSummaryRow(
			f, sheet, row, "Carryover (previous unpaid)",
			pi.CarryoverAmount, st, false,
		)
		row++
	}
	if pi.PaidAmount != 0 {
		writeSummaryRow(
			f, sheet, row, "Paid Amount",
			pi.PaidAmount, st, false,
		)
		row++
	}
	writeSummaryRow(
		f, sheet, row, "Total unpaid Amount",
		pi.TotalUnpaid, st, true,
	)
	row++

	if text := sanitizePILines(terms); text != "" {
		row++
		f.SetCellValue(sheet, cellRef(row, 1), "Terms:")
		f.SetCellStyle(sheet, cellRef(row, 1), cellRef(row, 1), st.chargeText)
		_ = f.SetRowHeight(sheet, row, st.summaryRowHeight)
		row++
		f.MergeCell(sheet, cellRef(row, 1), cellRef(row+3, piLastCol))
		f.SetCellValue(sheet, cellRef(row, 1), text)
		f.SetCellStyle(sheet, cellRef(row, 1), cellRef(row+3, piLastCol), st.note)
		row += 4
	}

	f.MergeCell(sheet, cellRef(row, 1), cellRef(row, piLastCol))
	f.SetCellValue(sheet, cellRef(row, 1), "Box and item in Stock")
	styleRow(f, sheet, row, st.sectionLabel)
	_ = f.SetRowHeight(sheet, row, st.summaryRowHeight)
	row++
	f.MergeCell(sheet, cellRef(row, 1), cellRef(row, piLastCol))
	f.SetCellValue(sheet, cellRef(row, 1), sanitizePILines(pi.StockNote))
	f.SetCellStyle(sheet, cellRef(row, 1), cellRef(row, piLastCol), st.note)
	_ = f.SetRowHeight(sheet, row, st.noteRowHeight)
}

// writePIItemRow 写入单条明细行
// writePIItemRow 写入单条明细行
func writePIItemRow(
	f *excelize.File,
	sheet string,
	row int,
	it models.ProformaInvoiceItem,
	st piTemplateStyles,
	images map[uint64]string,
) {
	// Model 写在 A 列
	if model := sanitizePIText(it.Model); model != "" {
		f.SetCellValue(sheet, cellRef(row, 1), model)
	}
	log.Printf("writePIItemRow: row=%d, model=%s, description=%s", row, it.Model, it.Description)

	// 判断是否有图片，如果有则插入图片
	hasImage := false
	if it.ItemType == models.PIItemGoods && it.SkuID != nil {
		if resolvePIImagePath(images[*it.SkuID]) != "" {
			hasImage = true
			addPIRowImage(f, sheet, row, images[*it.SkuID], st)
		}
	}

	// 【改动】Description 合并 C:D:E
	f.MergeCell(sheet, cellRef(row, piDescStartCol), cellRef(row, piDescEndCol))
	descText := sanitizePIText(it.Description)

	if descText != "" {
		if hasImage {
			// 【核心改动】有图片时，不再加空格，而是将文字写在图片后面的无边框文本框里
			addPIDescTextBox(f, sheet, row, descText, st)
		} else {
			// 没有图片时，正常将富文本写入单元格即可
			richText := []excelize.RichTextRun{
				{
					Text: descText,
					Font: &excelize.Font{Size: 6, Bold: false, Italic: false},
				},
			}
			_ = f.SetCellRichText(sheet, cellRef(row, piDescStartCol), richText)
		}
	}

	// F 列 Unit Price
	if it.UnitPrice > 0 {
		f.SetCellValue(sheet, cellRef(row, 6), it.UnitPrice)
	}
	// G 列 Quantity
	f.SetCellValue(sheet, cellRef(row, 7), it.Quantity)

	// H 列 Amount
	if it.IsPaid {
		f.SetCellValue(sheet, cellRef(row, 8), "Paid")
	} else {
		if it.UnitPrice > 0 {
			formula := fmt.Sprintf("=IF(F%d=\"\",0,F%d*G%d)", row, row, row)
			_ = setCellFormula(f, sheet, cellRef(row, 8), formula)
		} else {
			_ = f.SetCellValue(sheet, cellRef(row, 8), it.Amount)
		}
	}

	// J 列隐藏辅助标记
	mark := 0.0
	if !it.IsPaid {
		mark = 1.0
	}
	_ = f.SetCellValue(sheet, cellRef(row, piMarkerCol), mark)

	// 应用样式
	f.SetCellStyle(sheet, cellRef(row, 1), cellRef(row, 2), st.model)
	f.SetCellStyle(sheet, cellRef(row, piDescStartCol), cellRef(row, piDescEndCol), st.description)
	f.SetCellStyle(sheet, cellRef(row, 6), cellRef(row, 6), st.price)
	f.SetCellStyle(sheet, cellRef(row, 7), cellRef(row, 7), st.qty)
	f.SetCellStyle(sheet, cellRef(row, 8), cellRef(row, 8), st.amount)
	_ = f.SetRowHeight(sheet, row, st.dataRowHeight)
}

// addPIDescTextBox 在 C 列产品图片的右侧插入一个无边框、与行同高的文本框来显示描述。
func addPIDescTextBox(
	f *excelize.File,
	sheet string,
	row int,
	text string,
	st piTemplateStyles,
) {
	// 1. 计算文本框尺寸 (单位: pt)
	// 图片占用的宽度 + 留一点间距 (例如 4pt)
	offsetX := st.imageColWidthPx + 4.0
	// 文本框可用宽度：CDE三列总宽 - 图片占用宽 - 左右间距
	// 假设 D 和 E 列保持默认宽度 (约 8.43字符)，这里按实际情况估算，或者直接给一个足够大的固定值
	totalCDEWidthPx := st.imageColWidthPx + (8.43*7+5)*2
	boxWidth := totalCDEWidthPx - offsetX - 2.0
	if boxWidth < 200 {
		boxWidth = 200 // 保底宽度
	}
	if offsetX > 60 {
		offsetX = 60 // 保底间距，避免文本框被挤得太靠右
	}

	// 高度与行高一致 (Excelize 中行高单位为 pt)
	boxHeight := st.dataRowHeight

	// 2. 定义无边框、透明背景的样式
	lineColor := "FFFFFF"       // 颜色其实无所谓，因为 width 为 0
	var lineWidth float64 = 0.0 // 【关键】线宽设为 0，即无边框

	fillNone := "none" // 【关键】背景透明，不遮挡单元格底色
	// 3. 插入文本框 (Shape)
	err := f.AddShape(sheet, &excelize.Shape{
		Cell:   cellRef(row, piDescStartCol), // 锚定在 C 列当前行
		Type:   "rect",                       // 矩形文本框
		Width:  uint(boxWidth),
		Height: uint(boxHeight),
		Line: excelize.LineOptions{
			Fill: excelize.Fill{
				Color: []string{lineColor},
			},
			Width: lineWidth,
		},
		Fill: excelize.Fill{
			Type:    fillNone, // 无填充
			Pattern: 0,
		},
		Format: excelize.GraphicOptions{
			OffsetX:     int(offsetX), // 紧挨着图片右侧开始
			OffsetY:     1,            // 顶部对齐（与行同高）
			Positioning: "oneCell",    // 随单元格移动和改变大小
		},
		Paragraph: []excelize.RichTextRun{
			{
				Text: text,
				Font: &excelize.Font{
					Size:   6, // 与你原来设定的字号保持一致
					Bold:   false,
					Italic: false,
					// 如果需要文字垂直居中，可以配合 Family 等属性调整
				},
			},
		},
	})
	if err != nil {
		// 如果插入失败，回退到直接写入单元格，防止数据丢失
		_ = f.SetCellValue(sheet, cellRef(row, piDescStartCol), text)
	}
}

// addPIRowImage 在货款行的 C 列插入产品图片。
func addPIRowImage(
	f *excelize.File,
	sheet string,
	row int,
	imagePath string,
	st piTemplateStyles,
) {
	path := resolvePIImagePath(imagePath)
	if path == "" {
		return
	}
	width, height, ok := imagePixelSize(path)
	if !ok || width <= 0 || height <= 0 {
		return
	}

	areaWidth := st.imageColWidthPx
	areaHeight := st.dataRowHeight * 96 / 72

	// 限制图片最大高度，防止撑破行高（虽然你说不要改行高，但图片本身需要缩放适应当前行高）
	scale := math.Round(math.Min(
		(areaWidth-4)/float64(width),
		(areaHeight-4)/float64(height),
	)*10000) / 10000
	if scale <= 0 {
		return
	}

	// 【核心改动】图片左对齐，紧靠左边。OffsetX 设为 2 像素留一点点呼吸感，OffsetY 居中
	offsetX := 2
	offsetY := int((areaHeight - float64(height)*scale) / 2)
	if offsetY < 0 {
		offsetY = 0
	}

	printObject := true
	_ = f.AddPicture(sheet, cellRef(row, piImageCol), path, &excelize.GraphicOptions{
		ScaleX:      scale,
		ScaleY:      scale,
		OffsetX:     offsetX, // 强制靠左
		OffsetY:     offsetY,
		Positioning: "oneCell",
		PrintObject: &printObject,
	})
}

// addPIGroupImage 在分组明细行左侧的 A:B 区域插入客户原始订单截图（保持不变）。
func addPIGroupImage(
	f *excelize.File,
	sheet string,
	startRow, endRow int,
	imagePath string,
	st piTemplateStyles,
) {
	if endRow < startRow {
		return
	}
	path := resolvePIImagePath(imagePath)
	if path == "" {
		return
	}
	width, height, ok := imagePixelSize(path)
	if !ok || width <= 0 || height <= 0 {
		return
	}

	areaWidth := st.groupBoxWidthPx
	areaHeight := 0.0
	for row := startRow; row <= endRow; row++ {
		h, err := f.GetRowHeight(sheet, row)
		if err != nil || h <= 0 {
			h = st.dataRowHeight
		}
		areaHeight += h * 96 / 72
	}
	if areaWidth <= 0 || areaHeight <= 0 {
		return
	}

	scale := math.Round(math.Min(1, math.Min(
		(areaWidth-4)/float64(width),
		(areaHeight-4)/float64(height),
	))*10000) / 10000
	if scale <= 0 {
		return
	}

	f.MergeCell(sheet, cellRef(startRow, 1), cellRef(endRow, 2))
	f.SetCellStyle(sheet, cellRef(startRow, 1), cellRef(endRow, 2), st.model)
	addPIPicture(f, sheet, cellRef(startRow, 1), path, areaWidth, areaHeight, width, height, scale)
}

func addPIPicture(
	f *excelize.File,
	sheet, cell, path string,
	areaWidth, areaHeight float64,
	width, height int,
	scale float64,
) {
	offsetX := int((areaWidth - float64(width)*scale) / 2)
	offsetY := int((areaHeight - float64(height)*scale) / 2)
	if offsetX < 0 {
		offsetX = 0
	}
	if offsetY < 0 {
		offsetY = 0
	}
	printObject := true
	_ = f.AddPicture(sheet, cell, path, &excelize.GraphicOptions{
		ScaleX:      scale,
		ScaleY:      scale,
		OffsetX:     offsetX,
		OffsetY:     offsetY,
		Positioning: "oneCell",
		PrintObject: &printObject,
	})
}

func orderImage(orderImages map[uint64]string, g piGroup) string {
	if g.orderID == nil {
		return ""
	}
	return orderImages[*g.orderID]
}

func resolvePIImagePath(stored string) string {
	value := strings.TrimSpace(stored)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	for strings.Contains(value, "//") {
		value = strings.ReplaceAll(value, "//", "/")
	}
	value = strings.TrimPrefix(value, "/")
	if value == "" {
		return ""
	}
	if _, err := os.Stat(value); err != nil {
		return ""
	}
	return value
}

func imagePixelSize(path string) (int, int, bool) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, false
	}
	return config.Width, config.Height, true
}

func writeSummaryRow(
	f *excelize.File,
	sheet string,
	row int,
	label string,
	amount float64,
	st piTemplateStyles,
	total bool,
) {
	textStyle, amountStyle := st.chargeText, st.chargeAmount
	if total {
		textStyle, amountStyle = st.totalText, st.totalAmount
	}
	f.MergeCell(sheet, cellRef(row, 1), cellRef(row, 7))
	f.SetCellValue(sheet, cellRef(row, 1), label)
	f.SetCellValue(sheet, cellRef(row, 8), amount)
	f.SetCellStyle(sheet, cellRef(row, 1), cellRef(row, 7), textStyle)
	f.SetCellStyle(sheet, cellRef(row, 8), cellRef(row, 8), amountStyle)
	_ = f.SetRowHeight(sheet, row, st.summaryRowHeight)
}

func styleRow(f *excelize.File, sheet string, row, style int) {
	f.SetCellStyle(sheet, cellRef(row, 1), cellRef(row, piLastCol), style)
}

func rowOf(axis string) (int, bool) {
	i := len(axis)
	for i > 0 && axis[i-1] >= '0' && axis[i-1] <= '9' {
		i--
	}
	if i == len(axis) {
		return 0, false
	}
	row, err := strconv.Atoi(axis[i:])
	if err != nil {
		return 0, false
	}
	return row, true
}

type piGroup struct {
	title   string
	orderID *uint64
	rows    []models.ProformaInvoiceItem
}

func groupPIItems(items []models.ProformaInvoiceItem) []piGroup {
	groups := make([]piGroup, 0)
	indexByNo := make(map[int]int)
	for _, it := range items {
		switch it.ItemType {
		case models.PIItemGroup:
			indexByNo[it.GroupNo] = len(groups)
			groups = append(groups, piGroup{title: it.GroupTitle, orderID: it.OrderID})
		case models.PIItemGoods, models.PIItemBox:
			if idx, ok := indexByNo[it.GroupNo]; ok {
				groups[idx].rows = append(groups[idx].rows, it)
			}
		}
	}
	return groups
}

// sumUnpaidAmount 汇总未付款行的金额（与 recalcTotals 的口径一致）。
func sumUnpaidAmount(items []models.ProformaInvoiceItem) float64 {
	var total float64
	for _, it := range items {
		if it.IsPaid {
			continue
		}
		total += it.Amount
	}
	return round2(total)
}

func filterItems(
	items []models.ProformaInvoiceItem,
	itemType string,
) []models.ProformaInvoiceItem {
	result := make([]models.ProformaInvoiceItem, 0)
	for _, it := range items {
		if it.ItemType == itemType {
			result = append(result, it)
		}
	}
	return result
}

func cellRef(row, col int) string {
	return fmt.Sprintf("%s%d", colName(col), row)
}

func colName(col int) string {
	name := ""
	for col > 0 {
		col--
		name = string(rune('A'+col%26)) + name
		col /= 26
	}
	return strings.ToUpper(name)
}
