package dlp

import "testing"

func TestSummarize(t *testing.T) {
	d := NewDetector()
	s := d.Summarize(
		map[string]any{"email": "john@example.com", "card": "4111111111111111", "jwt": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature"},
		map[string]any{"phone": "+1 202-555-0180", "nid": "123456789"},
	)
	if s["email"].(map[string]any)["count"].(int) == 0 {
		t.Fatalf("expected email > 0")
	}
	if s["credit_card"].(map[string]any)["count"].(int) == 0 {
		t.Fatalf("expected credit_card > 0")
	}
	if s["phone"].(map[string]any)["count"].(int) == 0 {
		t.Fatalf("expected phone > 0")
	}
}
