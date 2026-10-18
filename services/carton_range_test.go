package services

import (
	"testing"

	"OrderAI/models"
)

// 总箱数按箱号并集算：套装两个组成单品分属不同品类、带同一个箱号，不能算两次。
func TestCartonSumOfLines(t *testing.T) {
	const p = "FBA15MHZ6GKKU"

	// 真实发货单的 17 个报关行：行箱数累加 89，而箱号只到 86
	// （套装的两个组成单品分属不同品类，16 / 20 / 21 号箱被算了两遍）。
	lines := []models.DeclarationLine{
		{CartonCount: 12, CartonNo: p + "00001-12"},
		{CartonCount: 4, CartonNo: p + "000013-15 " + p + "000016"},
		{CartonCount: 1, CartonNo: p + "000016"},
		{CartonCount: 5, CartonNo: p + "000017-19 " + p + "000020-21"},
		{CartonCount: 3, CartonNo: p + "000020-21 " + p + "000026"},
		{CartonCount: 4, CartonNo: p + "000022-25"},
		{CartonCount: 1, CartonNo: p + "000027"},
		{CartonCount: 15, CartonNo: p + "000028-42"},
		{CartonCount: 19, CartonNo: p + "000043-61"},
		{CartonCount: 1, CartonNo: p + "000062"},
		{CartonCount: 2, CartonNo: p + "000063-64"},
		{CartonCount: 2, CartonNo: p + "000065-66"},
		{CartonCount: 3, CartonNo: p + "000067-69"},
		{CartonCount: 15, CartonNo: p + "000070-84"},
		{CartonCount: 1, CartonNo: p + "000085"},
		{CartonCount: 1, CartonNo: p + "000086"},
	}
	sum := 0.0
	for _, l := range lines {
		sum += l.CartonCount
	}
	if sum != 89 {
		t.Fatalf("用例前提：行箱数累加应为 89，实际 %v", sum)
	}
	if got := cartonSumOfLines(lines); got != 86 {
		t.Fatalf("总箱数应按箱号并集为 86，实际 %v", got)
	}

	// 最直接的重叠：同一个箱号在两行都登记，只算一次。
	dup := []models.DeclarationLine{
		{CartonCount: 2, CartonNo: p + "000020-21"},
		{CartonCount: 2, CartonNo: p + "000020-21"},
		{CartonCount: 1, CartonNo: p + "000026"},
	}
	if got := cartonSumOfLines(dup); got != 3 {
		t.Fatalf("20-21 重复登记只应算 2 箱 + 26 一箱 = 3，实际 %v", got)
	}

	// 有行没填箱号时退回按行累加（无从判断是否共用箱子）。
	mixed := []models.DeclarationLine{
		{CartonCount: 2, CartonNo: p + "000020-21"},
		{CartonCount: 3},
	}
	if got := cartonSumOfLines(mixed); got != 5 {
		t.Fatalf("缺箱号时应按行累加 2+3=5，实际 %v", got)
	}
}

func TestMergeCartonNumbers(t *testing.T) {
	const p = "FBA15MHZ6GKKU"

	cases := []struct {
		name string
		in   []string
		want string
	}{
		{
			name: "真实问题：13-15 与 16 应合并成 13-16",
			in:   []string{p + "000013-15", p + "000016"},
			want: p + "000013-16",
		},
		{
			name: "真实问题：17-19 与 20-21 应合并成 17-21",
			in:   []string{p + "000017-19", p + "000020-21"},
			want: p + "000017-21",
		},
		{
			name: "20-21 与 26 不连续，保持两段",
			in:   []string{p + "000020-21", p + "000026"},
			want: p + "000020-21 " + p + "000026",
		},
		{
			name: "同箱多 SKU 重复登记只出现一次",
			in:   []string{p + "000020-21", p + "000020-21"},
			want: p + "000020-21",
		},
		{
			name: "乱序输入也按箱号升序输出",
			in:   []string{p + "000016", p + "000013-15"},
			want: p + "000013-16",
		},
		{
			name: "三段首尾相接合并成一整段",
			in:   []string{p + "000013-15", p + "000016", p + "000017-19", p + "000020-21"},
			want: p + "000013-21",
		},
		{
			name: "单个箱号原样输出",
			in:   []string{p + "000085"},
			want: p + "000085",
		},
		{
			name: "5 位老格式保持原宽度",
			in:   []string{p + "00001-12"},
			want: p + "00001-12",
		},
		{
			name: "不同 FBA 前缀各自成段，互不合并",
			in:   []string{"FBA15MHZ6GKKU000001-3", "FBA15MHZ7AAAA000001-2"},
			want: "FBA15MHZ6GKKU000001-3 FBA15MHZ7AAAA000001-2",
		},
		{
			name: "一个字段里写了多个箱号（逗号分隔）",
			in:   []string{p + "000013-15, " + p + "000016"},
			want: p + "000013-16",
		},
		{
			name: "结束号重复写前缀也能合并",
			in:   []string{p + "000013-" + p + "000016"},
			want: p + "000013-16",
		},
		{
			name: "波浪号写法",
			in:   []string{p + "000013~15", p + "000016"},
			want: p + "000013-16",
		},
		{
			name: "无前缀纯数字",
			in:   []string{"15-17", "18"},
			want: "15-18",
		},
		{
			name: "解析不了的内容原样保留在末尾",
			in:   []string{"N/A", p + "000026"},
			want: p + "000026 N/A",
		},
		{
			name: "空输入输出空串",
			in:   nil,
			want: "",
		},
		{
			name: "空白与空串被忽略",
			in:   []string{"", "  ", p + "000086"},
			want: p + "000086",
		},
		{
			name: "异常大区间不展开，原样保留",
			in:   []string{p + "000001-999999"},
			want: p + "000001-999999",
		},
		{
			name: "报错格式：结束号小于起始号，原样保留",
			in:   []string{p + "000020-15"},
			want: p + "000020-15",
		},
	}

	for _, c := range cases {
		got := mergeCartonNumbers(c.in)
		if got != c.want {
			t.Errorf("%s\n  输入: %v\n  期望: %q\n  实际: %q", c.name, c.in, c.want, got)
		}
	}
}

// 报关行的真实数据：宠物球品类里 13-15/16 与 17-19/20-21 各自成段。
func TestMergeCartonNumbers_RealShipment(t *testing.T) {
	const p = "FBA15MHZ6GKKU"
	got := mergeCartonNumbers([]string{p + "000013-15", p + "000016"})
	if got != p+"000013-16" {
		t.Fatalf("13-15 + 16 应合并为 13-16，实际 %q", got)
	}
	got = mergeCartonNumbers([]string{p + "000017-19", p + "000020-21"})
	if got != p+"000017-21" {
		t.Fatalf("17-19 + 20-21 应合并为 17-21，实际 %q", got)
	}
	// 20-21 已被上一段吃掉；与 26 之间有 22-25 属于别的行，不能连起来。
	got = mergeCartonNumbers([]string{p + "000020-21", p + "000026"})
	if got != p+"000020-21 "+p+"000026" {
		t.Fatalf("20-21 与 26 不连续应保持两段，实际 %q", got)
	}
}
