package main

import (
	"encoding/json"
	"strings"
)

// matchFilter checks if a document matches a filter query
// Supports equality and operators: $gt, $lt, $in
func matchFilter(doc map[string]interface{}, filter map[string]interface{}) bool {
	for k, v := range filter {
		// case toán tử (vd: {"rating": {"$gt": 5}})
		if fv, ok := v.(map[string]interface{}); ok {
			for op, cond := range fv {
				switch strings.ToLower(op) {
				case "$gt":
					if num, ok := toFloat(doc[k]); ok {
						if num <= toFloatMust(cond) {
							return false
						}
					} else {
						return false
					}
				case "$lt":
					if num, ok := toFloat(doc[k]); ok {
						if num >= toFloatMust(cond) {
							return false
						}
					} else {
						return false
					}
				case "$in":
					if arr, ok := cond.([]interface{}); ok {
						found := false
						for _, av := range arr {
							if equals(doc[k], av) {
								found = true
								break
							}
						}
						if !found {
							return false
						}
					} else {
						return false
					}
				default:
					// chưa hỗ trợ toán tử này
					return false
				}
			}
		} else {
			// case: so sánh trực tiếp
			if !equals(doc[k], v) {
				return false
			}
		}
	}
	return true
}

// equals handles basic equality for string/number/json.Number
func equals(a, b interface{}) bool {
	switch va := a.(type) {
	case string:
		if vb, ok := b.(string); ok {
			return va == vb
		}
	case float64:
		if vb, ok := b.(float64); ok {
			return va == vb
		}
	case json.Number:
		if vb, ok := b.(json.Number); ok {
			return va.String() == vb.String()
		}
	}
	// fallback: direct comparison
	return a == b
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

func toFloatMust(v interface{}) float64 {
	if f, ok := toFloat(v); ok {
		return f
	}
	return 0
}
