// Package jsonmap provides the shared JSON object value used at persistence
// and transport boundaries. It is deliberately independent from GORM models
// and business modules so bounded contexts do not need to import a global
// models package merely to exchange arbitrary JSON objects.
package jsonmap

import (
	"database/sql/driver"
	"encoding/json"
)

// JSON is a JSON object with database/sql serialization support.
type JSON map[string]interface{}

// Value implements driver.Valuer.
func (value JSON) Value() (driver.Value, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

// Scan implements sql.Scanner.
//
// SQLite/PostgreSQL 驱动对 JSON 列可能返回 []byte，也可能返回 string
// （例如手工用 sqlite3 写入、或 TEXT 亲和性的列），两种都要认，
// 否则会静默扫成 nil —— 页面区块会“凭空消失”而没有任何报错。
func (value *JSON) Scan(source interface{}) error {
	if source == nil {
		*value = make(JSON)
		return nil
	}
	var raw []byte
	switch typed := source.(type) {
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		return nil
	}
	return json.Unmarshal(raw, value)
}
