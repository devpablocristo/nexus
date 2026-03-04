package authz

func HasScope(scopes []string, required string) bool {
	for _, s := range scopes {
		if s == required {
			return true
		}
	}
	return false
}

func HasAnyScope(scopes []string, required ...string) bool {
	for _, req := range required {
		if HasScope(scopes, req) {
			return true
		}
	}
	return false
}

func IsRole(role *string, accepted ...string) bool {
	if role == nil {
		return false
	}
	for _, r := range accepted {
		if *role == r {
			return true
		}
	}
	return false
}
