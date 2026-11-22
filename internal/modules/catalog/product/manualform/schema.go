package manualform

import (
	"errors"
	"net/mail"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

var (
	ErrSchemaInvalid   = errors.New("manual form schema invalid")
	ErrRequiredMissing = errors.New("manual form required missing")
	ErrFieldInvalid    = errors.New("manual form field invalid")
	ErrTypeInvalid     = errors.New("manual form type invalid")
	ErrOptionInvalid   = errors.New("manual form option invalid")
)

const (
	manualFormTypeText     = "text"
	manualFormTypeTextarea = "textarea"
	manualFormTypePhone    = "phone"
	manualFormTypeEmail    = "email"
	manualFormTypeNumber   = "number"
	manualFormTypeSelect   = "select"
	manualFormTypeRadio    = "radio"
	manualFormTypeCheckbox = "checkbox"
)

var phonePattern = regexp.MustCompile(`^\+?[0-9\-()\s]{6,20}$`)
var manualFormFieldKeyPattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

type manualFormField struct {
	Key         string
	Type        string
	Label       jsonmap.JSON
	Placeholder jsonmap.JSON
	Required    bool
	Regex       string
	Min         *float64
	Max         *float64
	MaxLen      *int
	Options     []string
}

type manualFormSchema struct {
	Fields []manualFormField
}

// ValidateAndNormalize 校验表单结构和提交值，并返回可持久化的规范化快照。
func ValidateAndNormalize(schemaJSON jsonmap.JSON, submissionJSON jsonmap.JSON) (jsonmap.JSON, jsonmap.JSON, error) {
	schema, normalizedSchema, err := parseManualFormSchema(schemaJSON)
	if err != nil {
		return nil, nil, err
	}
	normalizedSubmission, err := normalizeManualFormSubmission(schema, submissionJSON)
	if err != nil {
		return nil, nil, err
	}
	return normalizedSchema, normalizedSubmission, nil
}

// NormalizeSchema 只校验并规范化表单结构，供商品创建和更新使用。
func NormalizeSchema(schemaJSON jsonmap.JSON) (jsonmap.JSON, error) {
	_, normalizedSchema, err := parseManualFormSchema(schemaJSON)
	return normalizedSchema, err
}

func parseManualFormSchema(schemaJSON jsonmap.JSON) (*manualFormSchema, jsonmap.JSON, error) {
	if len(schemaJSON) == 0 {
		return &manualFormSchema{Fields: []manualFormField{}}, jsonmap.JSON{"fields": []jsonmap.JSON{}}, nil
	}
	rawFields, ok := schemaJSON["fields"]
	if !ok {
		return nil, nil, ErrSchemaInvalid
	}
	fieldList, ok := rawFields.([]interface{})
	if !ok {
		return nil, nil, ErrSchemaInvalid
	}

	result := &manualFormSchema{Fields: make([]manualFormField, 0, len(fieldList))}
	keys := make(map[string]struct{}, len(fieldList))
	normalizedFields := make([]jsonmap.JSON, 0, len(fieldList))

	for _, rawField := range fieldList {
		fieldMap, ok := rawField.(map[string]interface{})
		if !ok {
			return nil, nil, ErrSchemaInvalid
		}

		key, ok := trimStringField(fieldMap, "key")
		if !ok || key == "" {
			return nil, nil, ErrSchemaInvalid
		}
		if _, exists := keys[key]; exists {
			return nil, nil, ErrSchemaInvalid
		}
		if !manualFormFieldKeyPattern.MatchString(key) {
			return nil, nil, ErrSchemaInvalid
		}
		keys[key] = struct{}{}

		typeValue, ok := trimStringField(fieldMap, "type")
		if !ok || !isSupportedManualFormType(typeValue) {
			return nil, nil, ErrSchemaInvalid
		}

		label, err := parseLocaleTextMapStrict(fieldMap, "label")
		if err != nil {
			return nil, nil, err
		}
		placeholder, err := parseLocaleTextMapStrict(fieldMap, "placeholder")
		if err != nil {
			return nil, nil, err
		}
		required, err := parseBoolFieldStrict(fieldMap, "required")
		if err != nil {
			return nil, nil, err
		}
		regex, err := parseOptionalTrimmedStringStrict(fieldMap, "regex")
		if err != nil {
			return nil, nil, err
		}
		if regex != "" {
			if _, err := compileManualFormRegex(regex); err != nil {
				return nil, nil, ErrSchemaInvalid
			}
		}
		min, err := parseOptionalNumberStrict(fieldMap, "min")
		if err != nil {
			return nil, nil, err
		}
		max, err := parseOptionalNumberStrict(fieldMap, "max")
		if err != nil {
			return nil, nil, err
		}
		if min != nil && max != nil && *min > *max {
			return nil, nil, ErrSchemaInvalid
		}

		maxLen, err := parseOptionalIntStrict(fieldMap, "max_len")
		if err != nil {
			return nil, nil, err
		}
		if maxLen != nil && *maxLen <= 0 {
			return nil, nil, ErrSchemaInvalid
		}

		options, err := parseStringOptionsStrict(fieldMap, "options")
		if err != nil {
			return nil, nil, err
		}
		if (typeValue == manualFormTypeSelect || typeValue == manualFormTypeRadio || typeValue == manualFormTypeCheckbox) && len(options) == 0 {
			return nil, nil, ErrSchemaInvalid
		}

		field := manualFormField{
			Key:         key,
			Type:        typeValue,
			Label:       label,
			Placeholder: placeholder,
			Required:    required,
			Regex:       regex,
			Min:         min,
			Max:         max,
			MaxLen:      maxLen,
			Options:     options,
		}
		result.Fields = append(result.Fields, field)

		normalizedField := jsonmap.JSON{
			"key":      key,
			"type":     typeValue,
			"required": required,
		}
		if len(label) > 0 {
			normalizedField["label"] = label
		}
		if len(placeholder) > 0 {
			normalizedField["placeholder"] = placeholder
		}
		if regex != "" {
			normalizedField["regex"] = regex
		}
		if min != nil {
			normalizedField["min"] = *min
		}
		if max != nil {
			normalizedField["max"] = *max
		}
		if maxLen != nil {
			normalizedField["max_len"] = *maxLen
		}
		if len(options) > 0 {
			normalizedOptions := make([]string, len(options))
			copy(normalizedOptions, options)
			normalizedField["options"] = normalizedOptions
		}
		normalizedFields = append(normalizedFields, normalizedField)
	}

	normalizedSchema := jsonmap.JSON{
		"fields": normalizedFields,
	}
	return result, normalizedSchema, nil
}

func normalizeManualFormSubmission(schema *manualFormSchema, submissionJSON jsonmap.JSON) (jsonmap.JSON, error) {
	if schema == nil {
		return nil, ErrSchemaInvalid
	}
	if len(schema.Fields) == 0 {
		if len(submissionJSON) > 0 {
			return nil, ErrFieldInvalid
		}
		return jsonmap.JSON{}, nil
	}
	if len(submissionJSON) == 0 {
		for _, field := range schema.Fields {
			if field.Required {
				return nil, ErrRequiredMissing
			}
		}
		return jsonmap.JSON{}, nil
	}

	normalized := make(jsonmap.JSON, len(schema.Fields))
	allowed := make(map[string]manualFormField, len(schema.Fields))
	for _, field := range schema.Fields {
		allowed[field.Key] = field
	}

	for key := range submissionJSON {
		if _, ok := allowed[key]; !ok {
			return nil, ErrFieldInvalid
		}
	}

	for _, field := range schema.Fields {
		rawValue, exists := submissionJSON[field.Key]
		if !exists {
			if field.Required {
				return nil, ErrRequiredMissing
			}
			continue
		}

		normalizedValue, hasValue, err := normalizeManualFormFieldValue(field, rawValue)
		if err != nil {
			return nil, err
		}
		if !hasValue {
			if field.Required {
				return nil, ErrRequiredMissing
			}
			continue
		}
		normalized[field.Key] = normalizedValue
	}

	return normalized, nil
}

func normalizeManualFormFieldValue(field manualFormField, rawValue interface{}) (interface{}, bool, error) {
	switch field.Type {
	case manualFormTypeText, manualFormTypeTextarea:
		text, ok := rawValue.(string)
		if !ok {
			return nil, false, ErrTypeInvalid
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, false, nil
		}
		if field.MaxLen != nil && utf8.RuneCountInString(text) > *field.MaxLen {
			return nil, false, ErrFieldInvalid
		}
		if field.Regex != "" {
			compiled, err := compileManualFormRegex(field.Regex)
			if err != nil {
				return nil, false, ErrSchemaInvalid
			}
			if !compiled.MatchString(text) {
				return nil, false, ErrFieldInvalid
			}
		}
		return sanitizeManualFormText(text), true, nil
	case manualFormTypePhone:
		text, ok := rawValue.(string)
		if !ok {
			return nil, false, ErrTypeInvalid
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, false, nil
		}
		if field.MaxLen != nil && utf8.RuneCountInString(text) > *field.MaxLen {
			return nil, false, ErrFieldInvalid
		}
		if !phonePattern.MatchString(text) {
			return nil, false, ErrFieldInvalid
		}
		if field.Regex != "" {
			compiled, err := compileManualFormRegex(field.Regex)
			if err != nil {
				return nil, false, ErrSchemaInvalid
			}
			if !compiled.MatchString(text) {
				return nil, false, ErrFieldInvalid
			}
		}
		return sanitizeManualFormText(text), true, nil
	case manualFormTypeEmail:
		text, ok := rawValue.(string)
		if !ok {
			return nil, false, ErrTypeInvalid
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, false, nil
		}
		if field.MaxLen != nil && utf8.RuneCountInString(text) > *field.MaxLen {
			return nil, false, ErrFieldInvalid
		}
		if _, err := mail.ParseAddress(text); err != nil {
			return nil, false, ErrFieldInvalid
		}
		if field.Regex != "" {
			compiled, err := compileManualFormRegex(field.Regex)
			if err != nil {
				return nil, false, ErrSchemaInvalid
			}
			if !compiled.MatchString(text) {
				return nil, false, ErrFieldInvalid
			}
		}
		return sanitizeManualFormText(text), true, nil
	case manualFormTypeNumber:
		number, ok := parseSubmissionNumber(rawValue)
		if !ok {
			return nil, false, ErrTypeInvalid
		}
		if field.Min != nil && number < *field.Min {
			return nil, false, ErrFieldInvalid
		}
		if field.Max != nil && number > *field.Max {
			return nil, false, ErrFieldInvalid
		}
		return number, true, nil
	case manualFormTypeSelect, manualFormTypeRadio:
		value, ok := rawValue.(string)
		if !ok {
			return nil, false, ErrTypeInvalid
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, false, nil
		}
		for _, option := range field.Options {
			if value == option {
				return value, true, nil
			}
		}
		return nil, false, ErrOptionInvalid
	case manualFormTypeCheckbox:
		values, ok := parseCheckboxValues(rawValue)
		if !ok {
			return nil, false, ErrTypeInvalid
		}
		if len(values) == 0 {
			return nil, false, nil
		}
		allow := map[string]struct{}{}
		for _, option := range field.Options {
			allow[option] = struct{}{}
		}
		normalized := make([]string, 0, len(values))
		seen := map[string]struct{}{}
		for _, value := range values {
			if _, exists := allow[value]; !exists {
				return nil, false, ErrOptionInvalid
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			normalized = append(normalized, value)
		}
		sort.Strings(normalized)
		return normalized, true, nil
	default:
		return nil, false, ErrSchemaInvalid
	}
}

func compileManualFormRegex(raw string) (*regexp.Regexp, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrSchemaInvalid
	}

	pattern, flags, isLiteral := parseRegexLiteral(raw)
	if isLiteral {
		goPattern, err := convertRegexLiteralToGo(pattern, flags)
		if err != nil {
			return nil, err
		}
		return regexp.Compile(goPattern)
	}

	return regexp.Compile(raw)
}

func parseRegexLiteral(raw string) (string, string, bool) {
	if len(raw) < 2 || raw[0] != '/' {
		return "", "", false
	}
	idx := lastUnescapedSlash(raw)
	if idx <= 0 {
		return "", "", false
	}
	return raw[1:idx], raw[idx+1:], true
}

func lastUnescapedSlash(raw string) int {
	for i := len(raw) - 1; i > 0; i-- {
		if raw[i] != '/' {
			continue
		}
		backslashes := 0
		for j := i - 1; j >= 0 && raw[j] == '\\'; j-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return i
		}
	}
	return -1
}

func convertRegexLiteralToGo(pattern string, flags string) (string, error) {
	hasI := false
	hasM := false
	hasS := false

	for _, flag := range flags {
		switch flag {
		case 'i':
			hasI = true
		case 'm':
			hasM = true
		case 's':
			hasS = true
		case 'g', 'u', 'y':
			// Go 不支持 g/u/y 修饰符，校验场景可忽略其行为
		default:
			return "", ErrSchemaInvalid
		}
	}

	prefix := ""
	if hasI {
		prefix += "(?i)"
	}
	if hasM {
		prefix += "(?m)"
	}
	if hasS {
		prefix += "(?s)"
	}
	return prefix + pattern, nil
}

func isSupportedManualFormType(value string) bool {
	switch value {
	case manualFormTypeText,
		manualFormTypeTextarea,
		manualFormTypePhone,
		manualFormTypeEmail,
		manualFormTypeNumber,
		manualFormTypeSelect,
		manualFormTypeRadio,
		manualFormTypeCheckbox:
		return true
	default:
		return false
	}
}

func trimStringField(fieldMap map[string]interface{}, key string) (string, bool) {
	raw, ok := fieldMap[key]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func parseOptionalTrimmedStringStrict(fieldMap map[string]interface{}, key string) (string, error) {
	raw, ok := fieldMap[key]
	if !ok {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", ErrSchemaInvalid
	}
	return strings.TrimSpace(value), nil
}

func parseLocaleTextMapStrict(fieldMap map[string]interface{}, key string) (jsonmap.JSON, error) {
	raw, ok := fieldMap[key]
	if !ok {
		return jsonmap.JSON{}, nil
	}
	mapValue, ok := raw.(map[string]interface{})
	if !ok {
		return nil, ErrSchemaInvalid
	}
	result := jsonmap.JSON{}
	for locale, localeRaw := range mapValue {
		text, ok := localeRaw.(string)
		if !ok {
			return nil, ErrSchemaInvalid
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		result[locale] = text
	}
	return result, nil
}

func parseBoolFieldStrict(fieldMap map[string]interface{}, key string) (bool, error) {
	raw, ok := fieldMap[key]
	if !ok {
		return false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, ErrSchemaInvalid
	}
	return value, nil
}

func parseOptionalNumberStrict(fieldMap map[string]interface{}, key string) (*float64, error) {
	raw, ok := fieldMap[key]
	if !ok {
		return nil, nil
	}
	value, ok := parseSubmissionNumber(raw)
	if !ok {
		return nil, ErrSchemaInvalid
	}
	return &value, nil
}

func parseOptionalIntStrict(fieldMap map[string]interface{}, key string) (*int, error) {
	raw, ok := fieldMap[key]
	if !ok {
		return nil, nil
	}
	number, ok := parseSubmissionNumber(raw)
	if !ok {
		return nil, ErrSchemaInvalid
	}
	value := int(number)
	if float64(value) != number {
		return nil, ErrSchemaInvalid
	}
	return &value, nil
}

func parseStringOptionsStrict(fieldMap map[string]interface{}, key string) ([]string, error) {
	raw, ok := fieldMap[key]
	if !ok {
		return nil, nil
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil, ErrSchemaInvalid
	}
	options := make([]string, 0, len(list))
	seen := map[string]struct{}{}
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, ErrSchemaInvalid
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, ErrSchemaInvalid
		}
		if _, exists := seen[text]; exists {
			continue
		}
		seen[text] = struct{}{}
		options = append(options, text)
	}
	sort.Strings(options)
	return options, nil
}

func parseSubmissionNumber(raw interface{}) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int8:
		return float64(value), true
	case int16:
		return float64(value), true
	case int32:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint:
		return float64(value), true
	case uint8:
		return float64(value), true
	case uint16:
		return float64(value), true
	case uint32:
		return float64(value), true
	case uint64:
		return float64(value), true
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func parseCheckboxValues(raw interface{}) ([]string, bool) {
	switch value := raw.(type) {
	case []string:
		result := make([]string, 0, len(value))
		for _, item := range value {
			text := strings.TrimSpace(item)
			if text == "" {
				continue
			}
			result = append(result, text)
		}
		return result, true
	case []interface{}:
		result := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			result = append(result, text)
		}
		return result, true
	default:
		return nil, false
	}
}

// sanitizeManualFormText 只做首尾空白裁剪，不做 HTML 转义。
// 提交值按原文存储，展示端（Vue 文本插值）负责输出转义；写入时转义会让
// 后台、用户端和上游对接拿到 &amp; / &#34; 这类实体字面量。
func sanitizeManualFormText(raw string) string {
	return strings.TrimSpace(raw)
}
