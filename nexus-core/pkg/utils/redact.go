package utils

import (
	"strings"
)

var sensitiveKeys = map[string]struct{}{
	"password":      {},
	"token":         {},
	"api_key":       {},
	"apikey":        {},
	"authorization": {},
	"secret":        {},
	"ssn":           {},
	"credit_card":   {},
	"card_number":   {},
	"cvv":           {},
}

func Redact(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, vv := range t {
			if _, ok := sensitiveKeys[strings.ToLower(k)]; ok {
				out[k] = "***"
				continue
			}
			out[k] = Redact(vv)
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, vv := range t {
			out = append(out, Redact(vv))
		}
		return out
	default:
		return v
	}
}
