package types

type ctxKey string

const (
	CtxKeyRequestID ctxKey = "request_id"
	CtxKeyOrgID     ctxKey = "org_id"
	CtxKeyActor     ctxKey = "actor"
	CtxKeyRole      ctxKey = "role"
	CtxKeyScopes    ctxKey = "scopes"
)
