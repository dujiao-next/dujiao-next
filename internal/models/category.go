package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// JSON 类型定义，用于存储多语言内容
type JSON map[string]interface{}

// Value 实现 driver.Valuer 接口
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// StringArray 字符串数组类型，用于存储tags、images等
type StringArray []string

// Value 实现 driver.Valuer 接口
func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// Scan 实现 sql.Scanner 接口
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = StringArray{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// UintArray 存储无符号整数数组，序列化为 JSON
type UintArray []uint

// Value 实现 driver.Valuer 接口
func (u UintArray) Value() (driver.Value, error) {
	if u == nil {
		return nil, nil
	}
	return json.Marshal(u)
}

// Scan 实现 sql.Scanner 接口
func (u *UintArray) Scan(value interface{}) error {
	if value == nil {
		*u = UintArray{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, u)
}

// Category 分类表
type Category struct {
	ID        uint           `gorm:"primarykey" json:"id"`                         // 主键
	ParentID  uint           `gorm:"not null;default:0;index" json:"parent_id"`    // 父分类ID，0 表示一级分类
	Slug      string         `gorm:"uniqueIndex;not null" json:"slug"`             // 唯一标识
	NameJSON  JSON           `gorm:"type:json;not null" json:"name"`               // 多语言名称
	Icon      string         `gorm:"type:varchar(500)" json:"icon"`                // 分类图标（图片路径）
	SortOrder int            `gorm:"default:0;index" json:"sort_order"`            // 排序权重
	IsActive  bool           `gorm:"not null;default:true;index" json:"is_active"` // 是否启用，停用后前台不展示
	CreatedAt time.Time      `gorm:"index" json:"created_at"`                      // 创建时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`                               // 软删除时间
}

// TableName 指定表名
func (Category) TableName() string {
	return "categories"
}
