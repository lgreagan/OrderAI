package services

import (
	"sort"
	"strconv"
	"strings"

	"OrderAI/models"
)

// 箱号区间合并。
//
// 背景：报关资料按「报关品类」聚合，同一行会收集多条发货明细的货箱编号。
// 过去是 strings.Join(" ") 直接拼接字符串，于是同一串连号被拼成
//
//	FBA15MHZ6GKKU000013-15 FBA15MHZ6GKKU000016
//
// 报关行/仓库还得自己算一遍才知道是 13~16 箱。这里按数值把连号真正合并：
//
//	13-15 + 16              → FBA15MHZ6GKKU000013-16
//	17-19 + 20-21           → FBA15MHZ6GKKU000017-21
//	20-21 + 26              → FBA15MHZ6GKKU000020-21 FBA15MHZ6GKKU000026
//
// 只在「同前缀」内合并（不同 FBA 货件号互不相干）；无法解析成编号的
// 内容（如手写的 N/A）原样保留在末尾，绝不吞掉。

// cartonEntry 解析出的单个箱号。
type cartonEntry struct {
	prefix string // 箱号前缀，如 FBA15MHZ6GKKU
	num    int    // 箱序号
	digits string // 原始数字串，用于保留前导零宽度（000013 而不是 13）
}

// maxCartonSpan 一个区间最多展开多少箱，防止手误输入 1-999999 时撑爆内存。
const maxCartonSpan = 20000

// 手工填写常见的分隔与区间写法。
var (
	cartonSeparators = strings.NewReplacer(",", " ", "，", " ", ";", " ", "；", " ", "、", " ")
	cartonRangeMarks = strings.NewReplacer("~", "-", "—", "-", "–", "-", "至", "-", "到", "-")
)

// mergeCartonNumbers 把多段「箱号 / 箱号区间」合并成尽可能少的连续区间。
//
// tokens 可以已经是「一段」也可以自带多个箱号（空格、逗号、分号、顿号分隔）。
func mergeCartonNumbers(tokens []string) string {
	order := make([]string, 0) // 前缀首次出现顺序
	groups := make(map[string][]cartonEntry)
	extras := make([]string, 0) // 解析不了的，原样保留

	for _, raw := range tokens {
		for _, tok := range splitCartonTokens(raw) {
			e, end, ok := parseCartonToken(tok)
			if !ok {
				extras = append(extras, tok)
				continue
			}
			// 跨度大得不正常（手误输入 1-999999）：不敢展开，原样保留。
			if end-e.num > maxCartonSpan {
				extras = append(extras, tok)
				continue
			}
			if _, seen := groups[e.prefix]; !seen {
				order = append(order, e.prefix)
			}
			groups[e.prefix] = append(groups[e.prefix], e)

			// 区间展开成具体箱号，才能和相邻段接上。
			for n := e.num + 1; n <= end; n++ {
				groups[e.prefix] = append(groups[e.prefix], cartonEntry{
					prefix: e.prefix,
					num:    n,
					digits: strconv.Itoa(n),
				})
			}
		}
	}

	parts := make([]string, 0, len(order))
	for _, prefix := range order {
		parts = append(parts, mergePrefixGroup(prefix, groups[prefix])...)
	}
	parts = append(parts, extras...)

	return strings.Join(parts, " ")
}

// mergePrefixGroup 合并同一前缀下的箱号，返回形如
// ["FBA15MHZ6GKKU000013-16", "FBA15MHZ6GKKU000020"]。
func mergePrefixGroup(prefix string, list []cartonEntry) []string {
	if len(list) == 0 {
		return nil
	}

	// 同一箱号可能被多条明细重复登记：去重，保留最长的数字写法（前导零更齐）。
	width := make(map[int]string, len(list))
	for _, e := range list {
		cur, seen := width[e.num]
		if !seen || len(e.digits) > len(cur) {
			width[e.num] = e.digits
		}
	}

	nums := make([]int, 0, len(width))
	for n := range width {
		nums = append(nums, n)
	}
	sort.Ints(nums)

	out := make([]string, 0, len(nums))
	for i := 0; i < len(nums); {
		j := i
		for j+1 < len(nums) && nums[j+1] == nums[j]+1 {
			j++
		}
		if i == j {
			out = append(out, prefix+width[nums[i]])
		} else {
			// 结束号只写自然数字（000013-16），与手工填写的历史格式一致。
			out = append(out, prefix+width[nums[i]]+"-"+strconv.Itoa(nums[j]))
		}
		i = j + 1
	}
	return out
}

// splitCartonTokens 把一个字段拆成多个箱号（空格、逗号、分号、顿号都可能被手工用作分隔）。
func splitCartonTokens(raw string) []string {
	s := cartonSeparators.Replace(raw)
	fields := strings.Fields(s)
	// 去掉重复出现的同一个箱号（同箱多 SKU 时很常见）。
	seen := make(map[string]bool, len(fields))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// parseCartonToken 解析一个箱号 / 箱号区间。
//
//	FBA15MHZ6GKKU000016                     → prefix=FBA15MHZ6GKKU num=16  end=16
//	FBA15MHZ6GKKU000013-15                  → prefix=FBA15MHZ6GKKU num=13  end=15
//	FBA15MHZ6GKKU000013-000015              → 同上（结束号也带前导零）
//	FBA15MHZ6GKKU000013-FBA15MHZ6GKKU000015 → 同上（结束号重复写前缀）
func parseCartonToken(tok string) (cartonEntry, int, bool) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return cartonEntry{}, 0, false
	}

	// 统一区间分隔符（~ — – 至 到 都可能是手工写法），再取最右一个 -
	s := cartonRangeMarks.Replace(tok)

	left, right := s, ""
	if i := strings.LastIndex(s, "-"); i > 0 && i < len(s)-1 {
		left, right = s[:i], s[i+1:]
	}

	prefix, digits, ok := splitTrailingDigits(left)
	if !ok {
		return cartonEntry{}, 0, false
	}
	start, err := strconv.Atoi(digits)
	if err != nil {
		return cartonEntry{}, 0, false
	}
	entry := cartonEntry{prefix: strings.TrimSpace(prefix), digits: digits, num: start}
	end := start

	if right != "" {
		rPrefix, rDigits, ok := splitTrailingDigits(right)
		if !ok {
			// 右段不是数字（例如 "A-1号箱"）：整串当单号处理，不猜。
			return cartonEntry{}, 0, false
		}
		rPrefix = strings.TrimSpace(rPrefix)
		// 右段若另写了前缀，必须与左段一致，否则是两个不同箱号，不合并。
		if rPrefix != "" && rPrefix != entry.prefix {
			return cartonEntry{}, 0, false
		}
		if v, err := strconv.Atoi(rDigits); err == nil {
			end = v
		}
	}

	if end < start {
		return cartonEntry{}, 0, false
	}
	return entry, end, true
}

// splitTrailingDigits 拆出尾部的连续数字："FBA15MHZ6GKKU000013" → "FBA15MHZ6GKKU" + "000013"。
func splitTrailingDigits(s string) (prefix, digits string, ok bool) {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	if i == len(s) {
		return "", "", false
	}
	return s[:i], s[i:], true
}

// countUniqueCartons 统计一批箱号里实际有多少个箱子（并集，重复只算一次）。
// 返回 0 表示没有可解析的箱号。
func countUniqueCartons(tokens []string) float64 {
	seen := make(map[string]bool)
	for _, raw := range tokens {
		for _, tok := range splitCartonTokens(raw) {
			e, end, ok := parseCartonToken(tok)
			if !ok || end-e.num > maxCartonSpan {
				continue
			}
			for n := e.num; n <= end; n++ {
				seen[e.prefix+"/"+strconv.Itoa(n)] = true
			}
		}
	}
	return float64(len(seen))
}

// cartonSumOfLines 算报关资料的总箱数。
//
// 不能简单把各品类行的箱数相加：套装（如「6厘米两件套」）的两个组成单品分属
// 不同报关品类，各自带同一个箱号 16，累加会把这箱算两次 —— 于是出现
// 表头写 89 箱、箱号却只到 86 的矛盾。所以只要能解析到箱号，就按箱号并集算；
// 存在没填箱号的行时才退回按行累加（此时无从知道箱子是否共用）。
func cartonSumOfLines(lines []models.DeclarationLine) float64 {
	total := 0.0
	tokens := make([]string, 0, len(lines))
	for _, l := range lines {
		total += l.CartonCount
		if strings.TrimSpace(l.CartonNo) != "" {
			tokens = append(tokens, l.CartonNo)
		}
	}
	if n := countUniqueCartons(tokens); n > 0 && len(tokens) == len(lines) {
		return n
	}
	return total
}
