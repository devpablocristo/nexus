package errors

import (
	"net/http"
	"testing"

	"nexus-gateway/pkg/types"
)

func TestNormalizeHTTPError(t *testing.T) {
	status, apiErr := Normalize(types.NewHTTPError(http.StatusForbidden, types.ErrCodePolicyDenied, "denied"))
	if status != http.StatusForbidden {
		t.Fatalf("status: %d", status)
	}
	if apiErr.Code != types.ErrCodePolicyDenied {
		t.Fatalf("code: %s", apiErr.Code)
	}
}

func TestNormalizeFallback(t *testing.T) {
	status, apiErr := Normalize(assertErr("x"))
	if status != http.StatusInternalServerError || apiErr.Code != types.ErrCodeInternal {
		t.Fatalf("unexpected: %d %+v", status, apiErr)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
