package services

import (
	"regexp"
	"strings"
	"unicode"
)

// treatDispenserBallRe 匹配 PI 产品名里需要剔除的 "Treat Dispenser Ball"。
// 例：order_items.product_name="V1 Treat Dispenser Ball"，specification="Light Blue 6 cm"
// → 拼接后为 "V1 Treat Dispenser Ball Light Blue 6 cm" → 剔除后 "V1 Light Blue 6 cm"。
var treatDispenserBallRe = regexp.MustCompile(`(?i)treat\s+dispenser\s+ball`)

var whitespaceRunRe = regexp.MustCompile(`[ \t\f\v\r\n]+`)

// fullWidthReplacer 把常见全角标点转成半角，保证 PI 只出现英文符号。
var fullWidthReplacer = strings.NewReplacer(
	"，", ", ",
	"。", ". ",
	"、", ", ",
	"；", "; ",
	"：", ": ",
	"（", "(",
	"）", ")",
	"【", "[",
	"】", "]",
	"“", `"`,
	"”", `"`,
	"‘", "'",
	"’", "'",
	"！", "!",
	"？", "?",
	"－", "-",
	"＋", "+",
	"×", "x",
	"　", " ",
)

// hasCJK 判断文本是否含中日韩文字（PI 中不允许出现）。
func hasCJK(s string) bool {
	for _, r := range s {
		if isCJKRune(r) {
			return true
		}
	}
	return false
}

func isCJKRune(r rune) bool {
	switch {
	case unicode.Is(unicode.Han, r),
		unicode.Is(unicode.Hiragana, r),
		unicode.Is(unicode.Katakana, r),
		unicode.Is(unicode.Hangul, r),
		unicode.Is(unicode.Bopomofo, r):
		return true
	case r >= 0x2E80 && r <= 0x2FFF, // CJK 部首
		r >= 0x3000 && r <= 0x303F, // CJK 符号与标点（。，、等）
		r >= 0xFE30 && r <= 0xFE4F, // CJK 兼容形式
		r >= 0xFF00 && r <= 0xFFEF: // 半角/全角形式
		return true
	default:
		return false
	}
}

// sanitizePIText 把任意文本整理成不含中文的单行文本：
// 转换常见全角标点、剔除中日韩相关字符、压缩多余空白。
// 其它字符（重音字母、符号等）原样保留。
func sanitizePIText(s string) string {
	if s == "" {
		return ""
	}
	s = fullWidthReplacer.Replace(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isCJKRune(r) {
			continue
		}
		b.WriteRune(r)
	}
	return collapseSpaces(b.String())
}

// sanitizePILines 与 sanitizePIText 类似，但保留换行，用于 PI 底部的多行文本块。
func sanitizePILines(s string) string {
	if s == "" {
		return ""
	}
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = sanitizePIText(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func collapseSpaces(s string) string {
	return strings.TrimSpace(whitespaceRunRe.ReplaceAllString(s, " "))
}

// dedupeAdjacentWords 去掉相邻的重复词：V2 V2 Orange 6 cm → V2 Orange 6 cm。
func dedupeAdjacentWords(s string) string {
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if n := len(out); n > 0 && strings.EqualFold(out[n-1], f) {
			continue
		}
		out = append(out, f)
	}
	return strings.Join(out, " ")
}

// buildPIProductName 由 order_items 的 product_name + specification 生成 PI 产品名：
//   - 拼接两个字段（空格分隔）
//   - 去掉 "Treat Dispenser Ball"
//   - 去掉相邻重复词（如 V2 V2 → V2）
//   - 剔除中文
func buildPIProductName(productName, specification string) string {
	productName = strings.TrimSpace(productName)
	specification = strings.TrimSpace(specification)

	combined := productName
	if specification != "" {
		if combined != "" {
			combined += " "
		}
		combined += specification
	}

	combined = treatDispenserBallRe.ReplaceAllString(combined, " ")
	combined = sanitizePIText(combined)
	return dedupeAdjacentWords(combined)
}
