package models

import (
	"strings"
	"time"
)

// 操作动作标识。
const (
	OperationActionCreate  = "create"
	OperationActionUpdate  = "update"
	OperationActionDelete  = "delete"
	OperationActionExecute = "execute"
	OperationActionExport  = "export"
	OperationActionOther   = "other"
)

// OperationActionNames 动作标识 → 中文名。
var OperationActionNames = map[string]string{
	OperationActionCreate:  "新增",
	OperationActionUpdate:  "修改",
	OperationActionDelete:  "删除",
	OperationActionExecute: "执行",
	OperationActionExport:  "导出",
	OperationActionOther:   "操作",
}

// OperationActionName 返回动作中文名，未知动作按「操作」处理。
func OperationActionName(action string) string {
	if name, ok := OperationActionNames[action]; ok {
		return name
	}
	return "操作"
}

// DefaultOperator 当前系统还没有登录体系，操作人先统一记为管理员。
const DefaultOperator = "管理员"

// OperationLog 关键操作日志。
type OperationLog struct {
	ID          uint64    `json:"id"`
	Module      string    `json:"module"`
	ModuleName  string    `json:"module_name"`
	Action      string    `json:"action"`
	ActionName  string    `json:"action_name"`
	Description string    `json:"description"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	URI         string    `json:"uri"`
	TargetID    string    `json:"target_id"`
	Operator    string    `json:"operator"`
	IP          string    `json:"ip"`
	Status      int       `json:"status"`
	Success     bool      `json:"success"`
	ErrorMsg    string    `json:"error_message"`
	DurationMs  int64     `json:"duration_ms"`
	Detail      string    `json:"detail"`
	CreatedAt   time.Time `json:"created_at"`
}

// 操作日志分页默认值与上限。
const (
	OperationLogDefaultPageSize = 20
	OperationLogMaxPageSize     = 200
)

// OperationLogFilter 操作日志查询条件。
type OperationLogFilter struct {
	Module   string
	Action   string
	Keyword  string
	Operator string
	Success  string // "" 全部 / "1" 成功 / "0" 失败
	DateFrom string // yyyy-MM-dd
	DateTo   string // yyyy-MM-dd
	Page     int
	PageSize int
}

// Normalize 补齐分页默认值并限制取值范围。
func (f *OperationLogFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = OperationLogDefaultPageSize
	}
	if f.PageSize > OperationLogMaxPageSize {
		f.PageSize = OperationLogMaxPageSize
	}
	f.Module = strings.TrimSpace(f.Module)
	f.Action = strings.TrimSpace(f.Action)
	f.Keyword = strings.TrimSpace(f.Keyword)
	f.Operator = strings.TrimSpace(f.Operator)
	f.Success = strings.TrimSpace(f.Success)
	f.DateFrom = strings.TrimSpace(f.DateFrom)
	f.DateTo = strings.TrimSpace(f.DateTo)
}

// Offset 返回分页偏移量。
func (f OperationLogFilter) Offset() int {
	return (f.Page - 1) * f.PageSize
}

// OperationLogSummary 日志概览统计。
type OperationLogSummary struct {
	Total       int64 `json:"total"`
	Today       int64 `json:"today"`
	TodayFailed int64 `json:"today_failed"`
	Failed      int64 `json:"failed"`
}

// OperationLogOption 下拉筛选项。
type OperationLogOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int64  `json:"count"`
}
