package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

var (
	errBadShipmentID  = errors.New("发货单 ID 无效")
	errBadDeclaration = errors.New("报关资料数据格式错误")
	errBadTemplateID  = errors.New("报关模板 ID 无效")
	errBadMapping     = errors.New("模板映射不是合法 JSON")
)

// DeclarationController 报关模板 / 报关资料预览与导出。
type DeclarationController struct {
	Service  *services.DeclarationService
	Excel    *services.DeclarationExcelService
	Channels *services.ShippingChannelService
}

func NewDeclarationController(
	service *services.DeclarationService,
	excel *services.DeclarationExcelService,
	channels *services.ShippingChannelService,
) *DeclarationController {
	return &DeclarationController{Service: service, Excel: excel, Channels: channels}
}

func declError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{"success": false, "message": err.Error()})
}

// ListTemplates 报关模板列表（含映射 JSON，前端用于渲染映射编辑器）。
func (ctl *DeclarationController) ListTemplates(c echo.Context) error {
	items, err := ctl.Service.ListTemplates(c.Request().Context())
	if err != nil {
		return declError(c, err, http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "data": items})
}

// SaveMapping 保存某个模板的映射 JSON（保存前做一次解析校验）。
func (ctl *DeclarationController) SaveMapping(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return declError(c, errBadTemplateID, http.StatusBadRequest)
	}

	var body struct {
		Mapping string `json:"mapping"`
	}
	if err := c.Bind(&body); err != nil {
		return declError(c, errBadDeclaration, http.StatusBadRequest)
	}

	text := strings.TrimSpace(body.Mapping)
	if !json.Valid([]byte(text)) {
		return declError(c, errBadMapping, http.StatusBadRequest)
	}
	if _, err := models.ParseDeclarationMapping(text); err != nil {
		return declError(c, err, http.StatusBadRequest)
	}

	tpl, err := ctl.Service.GetTemplate(c.Request().Context(), id)
	if err != nil {
		return declError(c, err, http.StatusNotFound)
	}
	if err := ctl.Service.SaveTemplateMapping(c.Request().Context(), tpl.ID, text); err != nil {
		return declError(c, err, http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "message": "模板映射已保存"})
}

// ListChannels 渠道字典（发货单的「服务/渠道」下拉）。
func (ctl *DeclarationController) ListChannels(c echo.Context) error {
	items, err := ctl.Channels.List(
		c.Request().Context(),
		strings.ToUpper(strings.TrimSpace(c.QueryParam("country"))),
	)
	if err != nil {
		return declError(c, err, http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "data": items})
}

// Preview 按模板装配报关资料（不落库，供页面预览/微调）。
func (ctl *DeclarationController) Preview(c echo.Context) error {
	shipmentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || shipmentID == 0 {
		return declError(c, errBadShipmentID, http.StatusBadRequest)
	}
	templateID, _ := strconv.ParseUint(c.QueryParam("template_id"), 10, 64)

	doc, err := ctl.Service.Assemble(c.Request().Context(), shipmentID, templateID)
	if err != nil {
		return declError(c, err, http.StatusBadRequest)
	}

	templates, err := ctl.Service.ListTemplates(c.Request().Context())
	if err != nil {
		return declError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"doc":       doc,
			"templates": templates,
		},
	})
}

// Records 某发货单的历史导出留档（快照）。
func (ctl *DeclarationController) Records(c echo.Context) error {
	shipmentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || shipmentID == 0 {
		return declError(c, errBadShipmentID, http.StatusBadRequest)
	}
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	items, err := ctl.Service.ListRecords(c.Request().Context(), shipmentID, limit)
	if err != nil {
		return declError(c, err, http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "data": items})
}

// Export 生成并下载报关资料 Excel。
//
// 请求体可以带上前端微调过的 doc（含 lines）；不带则按模板重新装配。
// 生成成功后会把这次的资料快照留档（declaration_records），便于事后复现。
func (ctl *DeclarationController) Export(c echo.Context) error {
	shipmentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || shipmentID == 0 {
		return declError(c, errBadShipmentID, http.StatusBadRequest)
	}

	var body struct {
		TemplateID uint64                   `json:"template_id"`
		Lines      []models.DeclarationLine `json:"lines"`
	}
	// 请求体允许为空（直接按模板装配）。
	_ = c.Bind(&body)

	var doc *models.DeclarationDoc
	doc, err = ctl.Service.Assemble(c.Request().Context(), shipmentID, body.TemplateID)
	if err != nil {
		return declError(c, err, http.StatusBadRequest)
	}
	// 前端微调过的明细优先（数量、箱号、删行等）。
	if len(body.Lines) > 0 {
		doc.Lines = body.Lines
	}
	services.NormalizeDoc(doc)

	content, err := ctl.Excel.Build(c.Request().Context(), doc)
	if err != nil {
		return declError(c, err, http.StatusInternalServerError)
	}

	filename := declarationFilename(doc)

	// 留档失败不阻断下载（文件已经在手），但要让前端知道这次没存档。
	if _, err := ctl.Service.SaveRecord(c.Request().Context(), doc, filename); err != nil {
		c.Logger().Errorf("留档报关资料失败: %v", err)
		c.Response().Header().Set(declarationRecordHeader, "failed")
	} else {
		c.Response().Header().Set(declarationRecordHeader, "saved")
	}

	c.Response().Header().Set(
		echo.HeaderContentType,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	)
	c.Response().Header().Set("Content-Disposition", contentDisposition(filename))
	return c.Blob(http.StatusOK, "application/octet-stream", content)
}

// declarationRecordHeader 告知前端本次导出的留档结果（saved/failed）。
const declarationRecordHeader = "X-Declaration-Record"

// declarationFilename 生成下载文件名（不含扩展名）。
func declarationFilename(doc *models.DeclarationDoc) string {
	name := fmt.Sprintf("报关资料-%d", doc.ShipmentID)
	if doc.TemplateName != "" {
		name = fmt.Sprintf("报关资料-%s-%s", doc.TemplateName, doc.OrderNo)
	}
	return name + ".xlsx"
}

// contentDisposition 生成兼容中文文件名的 Content-Disposition。
//
// HTTP 头只允许 ASCII，故同时给出 ASCII 回退名与 RFC 5987 的 filename*。
func contentDisposition(filename string) string {
	fallback := make([]rune, 0, len(filename))
	for _, r := range filename {
		if r < 128 {
			fallback = append(fallback, r)
		} else {
			fallback = append(fallback, '_')
		}
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		string(fallback), url.PathEscape(filename))
}
