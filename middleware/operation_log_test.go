package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"OrderAI/models"

	"github.com/labstack/echo/v4"
)

func match(t *testing.T, method, route string) (operationRule, bool) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(method, route, nil)
	c := e.NewContext(req, httptest.NewRecorder())
	c.SetPath(route)
	return matchOperationRule(c)
}

func TestMatchOperationRule(t *testing.T) {
	cases := []struct {
		method, route string
		wantOK        bool
		wantModule    string
		wantAction    string
	}{
		{"POST", "/api/orders", true, moduleOrders, models.OperationActionCreate},
		{"PUT", "/api/orders/:id", true, moduleOrders, models.OperationActionUpdate},
		{"DELETE", "/api/settings/:key", true, moduleSettings, models.OperationActionDelete},
		{"GET", "/api/proforma-invoices/:id/export", true, modulePI, models.OperationActionExport},
		{"GET", "/api/skus", false, "", ""},
		{"GET", "/api/operation-logs", false, "", ""},
		{"POST", "/api/order/upload", false, "", ""},
		{"POST", "/api/skus/upload-image", false, "", ""},
		{"POST", "/api/order/match", false, "", ""},
		{"POST", "/uploads/pic.png", false, "", ""},
		// 未登记的写接口走兜底规则。
		{"POST", "/api/unknown", true, "system", models.OperationActionCreate},
		{"POST", "/api/unknown/thing", true, "system", models.OperationActionExecute},
		{"PATCH", "/api/unknown/thing", true, "system", models.OperationActionUpdate},
		{"DELETE", "/api/unknown/thing", true, "system", models.OperationActionDelete},
	}

	for _, tc := range cases {
		got, ok := match(t, tc.method, tc.route)
		if ok != tc.wantOK {
			t.Fatalf("%s %s: ok = %v, want %v", tc.method, tc.route, ok, tc.wantOK)
		}
		if !ok {
			continue
		}
		if got.Module != tc.wantModule || got.Action != tc.wantAction {
			t.Fatalf("%s %s: got %s/%s, want %s/%s",
				tc.method, tc.route, got.Module, got.Action, tc.wantModule, tc.wantAction)
		}
		if got.Description == "" {
			t.Fatalf("%s %s: 描述为空", tc.method, tc.route)
		}
	}
}

func TestActionByMethodAndSubActionPath(t *testing.T) {
	if actionByMethod(http.MethodPost, "/api/orders") != models.OperationActionCreate {
		t.Fatal("集合地址的 POST 应为新增")
	}
	for _, path := range []string{
		"/api/orders/confirm",
		"/api/inventory/in",
		"/api/proforma-invoices/generate",
		"/api/items/:id/toggle",
	} {
		if actionByMethod(http.MethodPost, path) != models.OperationActionExecute {
			t.Fatalf("%s 的 POST 应为执行", path)
		}
	}
	if actionByMethod(http.MethodPost, "/api/orders/:id") != models.OperationActionCreate {
		t.Fatal("路由参数结尾的 POST 应保持新增")
	}
	if isSubActionPath("/api/orders") {
		t.Fatal("/api/orders 不是子动作")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abc", 5); got != "abc" {
		t.Fatalf("got %q", got)
	}
	if got := truncate("用户操作日志", 2); got != "用户…" {
		t.Fatalf("got %q", got)
	}
}

func TestResponseRecorderPassThroughAndRestore(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/orders", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/orders")

	original := c.Response().Writer
	capture := newResponseRecorder(c)
	if c.Response().Writer == original {
		t.Fatal("未接管响应writer")
	}

	// 业务写出的内容要原样透传给客户端，同时进入采集缓冲区。
	if _, err := c.Response().Write([]byte(`{"success":false,"message":"失败原因"}`)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("透传状态码 = %d", rec.Code)
	}
	if rec.Body.String() != `{"success":false,"message":"失败原因"}` {
		t.Fatalf("透传响应体 = %q", rec.Body.String())
	}

	capture.restore()
	if c.Response().Writer != original {
		t.Fatal("响应writer未还原")
	}

	// 还原后 Echo 的错误处理仍写到客户端。
	errorRec := httptest.NewRecorder()
	errorCtx := e.NewContext(httptest.NewRequest(http.MethodPost, "/api/orders", nil), errorRec)
	e.HTTPErrorHandler(echo.NewHTTPError(http.StatusBadRequest, "bad"), errorCtx)
	if errorRec.Code != http.StatusBadRequest {
		t.Fatalf("错误状态码 = %d", errorRec.Code)
	}

	payload := capture.payload()
	if payload == nil || payload.Message != "失败原因" {
		t.Fatal("采集到的响应体解析失败")
	}
	if payload.DataID() != "" {
		t.Fatal("无 data 时不应解析出主键")
	}
}

func TestResponsePayloadDataID(t *testing.T) {
	payload := &responsePayload{Success: true, Data: []byte(`{"id":12,"name":"x"}`)}
	if got := payload.DataID(); got != "12" {
		t.Fatalf("DataID = %q", got)
	}

	item := &responsePayload{Success: true, Data: []byte(`{"id":"PO-1"}`)}
	if got := item.DataID(); got != "PO-1" {
		t.Fatalf("DataID = %q", got)
	}

	empty := &responsePayload{Success: true}
	if got := empty.DataID(); got != "" {
		t.Fatalf("DataID = %q", got)
	}
}
