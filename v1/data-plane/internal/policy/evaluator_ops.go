package policy

import (
	"fmt"
	"strings"
)

// evalOp aplica un operador (exists, eq, in, regex, etc.) sobre target/value.
func (e *Evaluator) evalOp(op string, target any, value any, exists bool) (bool, error) {
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
		return evalOpIn(target, value, exists)
	case "contains":
		return evalOpContains(target, value, exists)
	case "regex":
		return e.evalOpRegex(target, value, exists)
	default:
		return false, nil
	}
}

func evalOpIn(target any, value any, exists bool) (bool, error) {
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
}

func evalOpContains(target any, value any, exists bool) (bool, error) {
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
}

func (e *Evaluator) evalOpRegex(target any, value any, exists bool) (bool, error) {
	if !exists {
		return false, nil
	}
	pat, ok := value.(string)
	if !ok {
		return false, nil
	}
	if len(pat) > 1024 {
		return false, fmt.Errorf("regex pattern too long (%d chars, max 1024)", len(pat))
	}
	s, ok := target.(string)
	if !ok {
		return false, nil
	}
	re, err := e.compileRegex(pat)
	if err != nil {
		return false, err
	}
	return re.MatchString(s), nil
}
