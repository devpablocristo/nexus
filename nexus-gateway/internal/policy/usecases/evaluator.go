package usecases

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type ToolAttributes struct {
	Name       string
	Kind       string
	Method     string
	URL        string
	ActionType string
	RiskLevel  int
}

type Evaluator struct{}

func NewEvaluator() *Evaluator { return &Evaluator{} }

func (e *Evaluator) Matches(conditionsJSON []byte, input, context map[string]any, tool ToolAttributes) (bool, error) {
	if len(conditionsJSON) == 0 || string(conditionsJSON) == "{}" {
		return true, nil
	}
	var cond any
	if err := json.Unmarshal(conditionsJSON, &cond); err != nil {
		return false, fmt.Errorf("conditions json invalid: %w", err)
	}
	return evalCond(cond, input, context, tool)
}

func evalCond(cond any, input, context map[string]any, tool ToolAttributes) (bool, error) {
	m, ok := cond.(map[string]any)
	if !ok {
		return false, nil
	}

	if allV, ok := m["all"]; ok {
		arr, ok := allV.([]any)
		if !ok {
			return false, nil
		}
		for _, c := range arr {
			b, err := evalCond(c, input, context, tool)
			if err != nil {
				return false, err
			}
			if !b {
				return false, nil
			}
		}
		return true, nil
	}
	if anyV, ok := m["any"]; ok {
		arr, ok := anyV.([]any)
		if !ok {
			return false, nil
		}
		for _, c := range arr {
			b, err := evalCond(c, input, context, tool)
			if err != nil {
				return false, err
			}
			if b {
				return true, nil
			}
		}
		return false, nil
	}
	if notV, ok := m["not"]; ok {
		b, err := evalCond(notV, input, context, tool)
		if err != nil {
			return false, err
		}
		return !b, nil
	}

	path, _ := m["path"].(string)
	op, _ := m["op"].(string)
	value := m["value"]
	target, exists := getPathValue(path, input, context, tool)

	switch op {
	case "exists":
		return exists && target != nil, nil
	case "not_exists":
		return !exists || target == nil, nil
	case "eq":
		if !exists {
			return false, nil
		}
		return equalsJSON(target, value), nil
	case "neq":
		if !exists {
			return false, nil
		}
		return !equalsJSON(target, value), nil
	case "lt", "lte", "gt", "gte":
		if !exists {
			return false, nil
		}
		return compareNumeric(op, target, value), nil
	case "in":
		if !exists {
			return false, nil
		}
		arr, ok := value.([]any)
		if !ok {
			return false, nil
		}
		for _, v := range arr {
			if equalsJSON(target, v) {
				return true, nil
			}
		}
		return false, nil
	case "contains":
		if !exists {
			return false, nil
		}
		switch t := target.(type) {
		case string:
			s, ok := value.(string)
			return ok && strings.Contains(t, s), nil
		case []any:
			for _, v := range t {
				if equalsJSON(v, value) {
					return true, nil
				}
			}
			return false, nil
		default:
			return false, nil
		}
	case "regex":
		if !exists {
			return false, nil
		}
		pat, ok := value.(string)
		if !ok {
			return false, nil
		}
		s, ok := target.(string)
		if !ok {
			return false, nil
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return false, err
		}
		return re.MatchString(s), nil
	default:
		return false, nil
	}
}

func getPathValue(path string, input, context map[string]any, tool ToolAttributes) (any, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, false
	}
	ns := parts[0]
	rest := parts[1:]

	switch ns {
	case "input":
		return traverse(input, rest)
	case "context":
		return traverse(context, rest)
	case "tool":
		toolMap := map[string]any{
			"name":        tool.Name,
			"kind":        tool.Kind,
			"method":      tool.Method,
			"url":         tool.URL,
			"action_type": tool.ActionType,
			"risk_level":  float64(tool.RiskLevel),
		}
		return traverse(toolMap, rest)
	default:
		return nil, false
	}
}

func traverse(root map[string]any, parts []string) (any, bool) {
	var cur any = root
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[p]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func equalsJSON(a, b any) bool {
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if aok && bok {
		return af == bf
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func compareNumeric(op string, a, b any) bool {
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if !aok || !bok {
		return false
	}
	switch op {
	case "lt":
		return af < bf
	case "lte":
		return af <= bf
	case "gt":
		return af > bf
	case "gte":
		return af >= bf
	default:
		return false
	}
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
