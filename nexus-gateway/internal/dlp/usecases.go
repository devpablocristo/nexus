package dlp

import (
	"regexp"
	"strings"
)

type Detector struct {
	emailRe    *regexp.Regexp
	phoneRe    *regexp.Regexp
	jwtRe      *regexp.Regexp
	apiKeyRe   *regexp.Regexp
	nationalRe *regexp.Regexp
}

type TypeSummary struct {
	Count int `json:"count"`
}

func NewDetector() *Detector {
	return &Detector{
		emailRe:    regexp.MustCompile(`(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`),
		phoneRe:    regexp.MustCompile(`\+?[0-9][0-9\-\s()]{7,}[0-9]`),
		jwtRe:      regexp.MustCompile(`(?i)\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`),
		apiKeyRe:   regexp.MustCompile(`(?i)\b(sk_live|sk_test|api[_-]?key|xoxb|ghp_)[A-Za-z0-9_-]{8,}\b`),
		nationalRe: regexp.MustCompile(`\b[0-9]{8,12}\b`),
	}
}

func (d *Detector) Summarize(input, context map[string]any) map[string]any {
	counts := map[string]int{
		"email":       0,
		"phone":       0,
		"credit_card": 0,
		"national_id": 0,
		"jwt":         0,
		"api_key":     0,
	}
	walkAny(input, func(s string) { d.matchString(strings.TrimSpace(s), counts) })
	walkAny(context, func(s string) { d.matchString(strings.TrimSpace(s), counts) })

	out := map[string]any{}
	for k, v := range counts {
		out[k] = map[string]any{"count": v}
	}
	return out
}

func (d *Detector) matchString(s string, counts map[string]int) {
	if s == "" {
		return
	}
	if d.emailRe.MatchString(s) {
		counts["email"]++
	}
	if d.phoneRe.MatchString(s) {
		counts["phone"]++
	}
	if d.jwtRe.MatchString(s) {
		counts["jwt"]++
	}
	if d.apiKeyRe.MatchString(s) {
		counts["api_key"]++
	}
	if d.nationalRe.MatchString(s) {
		counts["national_id"]++
	}
	if looksLikeCard(s) {
		counts["credit_card"]++
	}
}

func walkAny(v any, fn func(string)) {
	switch t := v.(type) {
	case map[string]any:
		for _, vv := range t {
			walkAny(vv, fn)
		}
	case []any:
		for _, vv := range t {
			walkAny(vv, fn)
		}
	case string:
		fn(t)
	}
}

func looksLikeCard(s string) bool {
	digits := make([]int, 0, len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := digits[i]
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return sum%10 == 0
}
