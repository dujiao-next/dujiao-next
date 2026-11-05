package jsonmap

import (
	"reflect"
	"testing"
)

func TestJSONDatabaseRoundTrip(t *testing.T) {
	original := JSON{
		"enabled": true,
		"name":    "storefront",
		"nested":  map[string]interface{}{"count": float64(2)},
	}

	encoded, err := original.Value()
	if err != nil {
		t.Fatalf("encode JSON value: %v", err)
	}

	var decoded JSON
	if err := decoded.Scan(encoded); err != nil {
		t.Fatalf("scan JSON value: %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("JSON round trip mismatch\nwant: %#v\ngot:  %#v", original, decoded)
	}
}

func TestJSONScanAcceptsTextColumn(t *testing.T) {
	// 同一个 JSON 列，驱动可能给出 []byte 也可能给出 string；
	// TEXT 亲和性（手工写入）时必须同样能扫出来。
	var decoded JSON
	if err := decoded.Scan(`{"blocks":[{"id":"b1"}]}`); err != nil {
		t.Fatalf("scan string JSON: %v", err)
	}
	blocks, ok := decoded["blocks"].([]interface{})
	if !ok || len(blocks) != 1 {
		t.Fatalf("scan string JSON lost content: %#v", decoded)
	}
}

func TestJSONScanIgnoresUnexpectedType(t *testing.T) {
	value := JSON{"keep": true}
	if err := value.Scan(123); err != nil {
		t.Fatalf("scan unexpected type should not error: %v", err)
	}
	if !value["keep"].(bool) {
		t.Fatalf("scan unexpected type should keep previous value: %#v", value)
	}
}

func TestJSONScanNilCreatesEmptyObject(t *testing.T) {
	value := JSON{"stale": true}
	if err := value.Scan(nil); err != nil {
		t.Fatalf("scan nil JSON: %v", err)
	}
	if value == nil || len(value) != 0 {
		t.Fatalf("scan nil should create an empty object, got %#v", value)
	}
}

func TestJSONNilDatabaseValueStaysNull(t *testing.T) {
	var value JSON
	encoded, err := value.Value()
	if err != nil {
		t.Fatalf("encode nil JSON: %v", err)
	}
	if encoded != nil {
		t.Fatalf("nil JSON should produce a NULL driver value, got %#v", encoded)
	}
}
