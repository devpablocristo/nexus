// Package leaseauth signs and verifies ephemeral execution lease tokens.
package leaseauth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	OrgID          string `json:"org_id"`
	LeaseID        string `json:"lease_id"`
	IntentID       string `json:"intent_id"`
	ToolName       string `json:"tool_name"`
	RiskClass      string `json:"risk_class"`
	CredentialMode string `json:"credential_mode"`
	Provider       string `json:"provider,omitempty"`
	Scope          string `json:"scope,omitempty"`
	TargetEnv      string `json:"target_env,omitempty"`
	jwt.RegisteredClaims
}

func SignToken(secret string, claims Claims) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errors.New("leaseauth: signing secret required")
	}
	if strings.TrimSpace(claims.LeaseID) == "" {
		return "", errors.New("leaseauth: lease_id required")
	}
	if strings.TrimSpace(claims.IntentID) == "" {
		return "", errors.New("leaseauth: intent_id required")
	}
	if strings.TrimSpace(claims.OrgID) == "" {
		return "", errors.New("leaseauth: org_id required")
	}
	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.IsZero() {
		return "", errors.New("leaseauth: exp required")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func VerifyToken(secret, tokenString, expectedIssuer string, now time.Time) (Claims, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return Claims{}, errors.New("leaseauth: signing secret required")
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(strings.TrimSpace(expectedIssuer)),
		jwt.WithTimeFunc(func() time.Time {
			if now.IsZero() {
				return time.Now().UTC()
			}
			return now.UTC()
		}),
	)
	claims := Claims{}
	token, err := parser.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("leaseauth: unexpected signing method %s", token.Method.Alg())
		}
		return []byte(secret), nil
	})
	if err != nil {
		return Claims{}, err
	}
	if !token.Valid {
		return Claims{}, errors.New("leaseauth: invalid token")
	}
	return claims, nil
}
