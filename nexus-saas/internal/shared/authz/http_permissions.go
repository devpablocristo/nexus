package authz

// CanAccess enforces scoped access:
// - admin/secops role: always allowed
// - all other requests: required scope must be present
func CanAccess(scopes []string, role *string, required string) bool {
	if IsRole(role, "admin", "secops") {
		return true
	}
	return HasScope(scopes, required)
}
