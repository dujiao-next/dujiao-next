package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"

	"gorm.io/gorm"
)

var localizedJSONSearchKeys = append([]string(nil), constants.SupportedLocales...)

// dbDialectName 获取数据库方言名称，默认按 sqlite 处理。
func dbDialectName(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		return "sqlite"
	}
	name := strings.ToLower(strings.TrimSpace(db.Dialector.Name()))
	if name == "" {
		return "sqlite"
	}
	return name
}

// jsonTextExpr 构建 JSON 字段文本提取表达式，兼容 sqlite 与 postgres。
func jsonTextExpr(db *gorm.DB, column, key string) string {
	return jsonTextExprByDialect(dbDialectName(db), column, key)
}

// jsonArrayLengthExpr 构建 JSON 数组长度表达式，兼容 sqlite 与 postgres。
func jsonArrayLengthExpr(db *gorm.DB, column string) string {
	return jsonArrayLengthExprByDialect(dbDialectName(db), column)
}

func jsonArrayLengthExprByDialect(dialect, column string) string {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "postgresql":
		return fmt.Sprintf("jsonb_array_length(COALESCE(%s::jsonb, '[]'::jsonb))", column)
	default:
		return fmt.Sprintf("json_array_length(COALESCE(%s, '[]'))", column)
	}
}

// jsonArrayContainsExpr 构建 JSON 数组元素包含查询表达式，兼容 sqlite 与 postgres。
// 返回的表达式含一个 ? 占位符，匹配值需通过 jsonArrayElementParam 获取。
func jsonArrayContainsExpr(db *gorm.DB, column string) string {
	return jsonArrayContainsExprByDialect(dbDialectName(db), column)
}

// jsonArrayElementParam 将匹配值适配为目标数据库的 JSON 数组元素参数值。
// PostgreSQL: 原始字符串（jsonb_array_elements_text 自动解码）。
// SQLite:     原始字符串（json_each 的 value 列在 = 比较时返回解码值）。
func jsonArrayElementParam(db *gorm.DB, value string) interface{} {
	return jsonArrayElementParamByDialect(dbDialectName(db), value)
}

func jsonArrayContainsExprByDialect(dialect, column string) string {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "postgresql":
		return fmt.Sprintf("EXISTS (SELECT 1 FROM jsonb_array_elements_text(%s::jsonb) AS t WHERE t = ?)", column)
	default:
		return fmt.Sprintf("EXISTS (SELECT 1 FROM json_each(%s) WHERE json_each.value = ?)", column)
	}
}

func jsonArrayElementParamByDialect(dialect, value string) interface{} {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "postgresql":
		return value
	default:
		return value
	}
}

func jsonTextExprByDialect(dialect, column, key string) string {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "postgresql":
		// postgres 统一转 jsonb 后再使用 ->> 提取文本
		return fmt.Sprintf("(%s::jsonb ->> '%s')", column, key)
	default:
		// sqlite 使用 json_extract，语言键使用引号避免 - 等特殊字符问题
		return fmt.Sprintf("json_extract(%s, '$.\"%s\"')", column, key)
	}
}

// localizedJSONCoalesceExpr 生成多语言字段回退表达式。
func localizedJSONCoalesceExpr(db *gorm.DB, column string) string {
	parts := make([]string, 0, len(localizedJSONSearchKeys)+1)
	for _, key := range localizedJSONSearchKeys {
		parts = append(parts, jsonTextExpr(db, column, key))
	}
	parts = append(parts, "''")
	return fmt.Sprintf("COALESCE(%s)", strings.Join(parts, ", "))
}

// buildLocalizedLikeCondition 构建普通列 + JSON 多语言列的 LIKE 条件，并返回参数数量。
func buildLocalizedLikeCondition(db *gorm.DB, plainColumns, jsonColumns []string) (string, int) {
	return buildLocalizedLikeConditionByDialect(dbDialectName(db), plainColumns, jsonColumns)
}

func buildLocalizedLikeConditionByDialect(dialect string, plainColumns, jsonColumns []string) (string, int) {
	parts := make([]string, 0, len(plainColumns)+len(jsonColumns)*len(localizedJSONSearchKeys))
	argCount := 0
	operator := likeOperatorByDialect(dialect)

	for _, column := range plainColumns {
		trimmed := strings.TrimSpace(column)
		if trimmed == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s ?", trimmed, operator))
		argCount++
	}

	for _, column := range jsonColumns {
		trimmed := strings.TrimSpace(column)
		if trimmed == "" {
			continue
		}
		for _, key := range localizedJSONSearchKeys {
			parts = append(parts, fmt.Sprintf("%s %s ?", jsonTextExprByDialect(dialect, trimmed, key), operator))
			argCount++
		}
	}

	return strings.Join(parts, " OR "), argCount
}

func likeOperatorByDialect(dialect string) string {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "postgresql":
		return "ILIKE"
	default:
		return "LIKE"
	}
}

// repeatLikeArgs 生成重复的 LIKE 参数列表。
func repeatLikeArgs(like string, count int) []interface{} {
	args := make([]interface{}, 0, count)
	for i := 0; i < count; i++ {
		args = append(args, like)
	}
	return args
}

// dateGroupExpr 构建 SQL 日期分组表达式，将 UTC 时间戳转为目标时区的 YYYY-MM-DD 字符串。
// refTime 用于 SQLite 获取固定 UTC 偏移（SQLite 不支持命名时区）。
func dateGroupExpr(db *gorm.DB, column string, loc *time.Location, refTime time.Time) string {
	if loc == nil {
		loc = time.UTC
	}
	dialect := dbDialectName(db)
	switch dialect {
	case "postgres", "postgresql":
		zoneName := loc.String()
		if zoneName == "" || zoneName == "Local" {
			zoneName = "UTC"
		}
		return fmt.Sprintf("TO_CHAR(%s AT TIME ZONE '%s', 'YYYY-MM-DD')", column, zoneName)
	default: // sqlite
		_, offset := refTime.In(loc).Zone()
		sign := "+"
		if offset < 0 {
			sign = "-"
			offset = -offset
		}
		hours := offset / 3600
		minutes := (offset % 3600) / 60
		if minutes != 0 {
			return fmt.Sprintf("strftime('%%Y-%%m-%%d', %s, '%s%d hours', '%s%d minutes')", column, sign, hours, sign, minutes)
		}
		return fmt.Sprintf("strftime('%%Y-%%m-%%d', %s, '%s%d hours')", column, sign, hours)
	}
}

// quotedStatusList 将状态常量数组拼接为 SQL IN 子句所需的带引号逗号分隔列表。
// 仅用于内部常量值，不可用于用户输入。
func quotedStatusList(statuses []string) string {
	parts := make([]string, len(statuses))
	for i, s := range statuses {
		parts[i] = "'" + s + "'"
	}
	return strings.Join(parts, ",")
}
