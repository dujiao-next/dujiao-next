package application

import (
	"testing"

	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/shopspring/decimal"
)

func TestCalculateMarkedUpPrice(t *testing.T) {
	tests := []struct {
		name     string
		upstream string
		markup   string
		rounding string
		expected string
	}{
		{"zero markup returns original", "10.00", "0", "none", "10.00"},
		{"50% markup", "10.00", "50", "none", "15.00"},
		{"100% markup doubles price", "10.00", "100", "none", "20.00"},
		{"200% markup triples price", "10.00", "200", "none", "30.00"},
		{"small price with markup", "0.01", "100", "none", "0.02"},
		{"fractional result rounds to 2 decimals", "10.00", "33", "none", "13.30"},
		{"negative markup floors at zero", "5.00", "-200", "none", "0"},
		{"ceil_int rounds up", "10.00", "23.4", "ceil_int", "13.00"},
		{"ceil_int exact integer stays", "10.00", "100", "ceil_int", "20"},
		{"ceil_tenth rounds up to 0.1", "10.00", "23.4", "ceil_tenth", "12.40"},
		{"ceil_tenth exact tenth stays", "10.00", "50", "ceil_tenth", "15.00"},
		{"zero price stays zero", "0.00", "100", "none", "0.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := decimal.RequireFromString(tt.upstream)
			markup := decimal.RequireFromString(tt.markup)
			expected := decimal.RequireFromString(tt.expected)

			result := CalculateMarkedUpPrice(upstream, markup, tt.rounding)
			if !result.Equal(expected) {
				t.Errorf("CalculateMarkedUpPrice(%s, %s%%, %s) = %s, want %s",
					tt.upstream, tt.markup, tt.rounding, result.String(), tt.expected)
			}
		})
	}
}

func TestCalculateLocalPrice(t *testing.T) {
	tests := []struct {
		name     string
		upstream string
		rate     string
		markup   string
		rounding string
		expected string
	}{
		{"USD to CNY with markup", "1.00", "7.2", "50", "none", "10.80"},
		{"rate 1 same as no conversion", "10.00", "1", "50", "none", "15.00"},
		{"rate 0 treated as 1", "10.00", "0", "50", "none", "15.00"},
		{"negative rate treated as 1", "10.00", "-1", "50", "none", "15.00"},
		{"rate with no markup", "1.00", "7.2", "0", "none", "7.20"},
		{"rate with ceil_int", "1.00", "7.2", "50", "ceil_int", "11.00"},
		{"rate with ceil_tenth", "1.00", "7.35", "0", "ceil_tenth", "7.35"},
		{"rate with ceil_tenth fractional", "1.00", "7.34", "1", "ceil_tenth", "7.50"},
		{"high precision rate", "100.00", "0.138462", "0", "none", "13.85"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := decimal.RequireFromString(tt.upstream)
			rate := decimal.RequireFromString(tt.rate)
			markup := decimal.RequireFromString(tt.markup)
			expected := decimal.RequireFromString(tt.expected)

			result := CalculateLocalPrice(upstream, rate, markup, tt.rounding)
			if !result.Equal(expected) {
				t.Errorf("CalculateLocalPrice(%s, rate=%s, %s%%, %s) = %s, want %s",
					tt.upstream, tt.rate, tt.markup, tt.rounding, result.String(), tt.expected)
			}
		})
	}
}

func TestResolveCategoryMarkup(t *testing.T) {
	def := decimal.RequireFromString("10")
	markups := jsonmap.JSON{
		"5": float64(50), // 父分类
		"7": "20",        // 字符串也要能解析
		"8": float64(0),  // 显式 0：该分类不加价，不能回落到默认
		"9": float64(-5), // 负数视为未配置
		"x": float64(99), // 非法键被忽略
	}
	parents := map[uint]uint{6: 5, 7: 5, 8: 5, 9: 5, 10: 6, 20: 21, 21: 20} // 20/21 成环

	tests := []struct {
		name       string
		markups    jsonmap.JSON
		categoryID uint
		parents    map[uint]uint
		want       string
	}{
		{"no config uses default", nil, 5, parents, "10"},
		{"uncategorized uses default", markups, 0, parents, "10"},
		{"own config wins", markups, 5, parents, "50"},
		{"string value parsed", markups, 7, parents, "20"},
		{"explicit zero is respected", markups, 8, parents, "0"},
		{"child inherits parent", markups, 6, parents, "50"},
		{"grandchild inherits ancestor", markups, 10, parents, "50"},
		{"negative falls through to parent", markups, 9, parents, "50"},
		{"unknown category uses default", markups, 99, parents, "10"},
		{"nil parents does not inherit", markups, 6, nil, "10"},
		{"parent cycle terminates with default", markups, 20, parents, "10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveCategoryMarkup(def, tt.markups, tt.categoryID, tt.parents)
			if !got.Equal(decimal.RequireFromString(tt.want)) {
				t.Errorf("ResolveCategoryMarkup(category=%d) = %s, want %s", tt.categoryID, got, tt.want)
			}
		})
	}
}
