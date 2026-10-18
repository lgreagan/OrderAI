package services

import (
	"math"
	"strings"
)

// round2 按金额惯例保留两位小数。
func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

// carryover 计算历史欠款 = 历史 PI 合计 − 历史收款。
// 收款多于开票（客户预收/多收）时按 0 计，避免出现负欠款冲抵本期金额。
func carryover(billed, paid float64) float64 {
	if diff := billed - paid; diff > 0 {
		return round2(diff)
	}
	return 0
}

// firstNonEmpty 返回第一个非空（去空格后）的字符串。
func firstNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
