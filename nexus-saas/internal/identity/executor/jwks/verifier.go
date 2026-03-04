package jwks

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Verifier struct {
	jwksURL    string
	httpClient *http.Client
	cacheTTL   time.Duration

	mu         sync.RWMutex
	keysByKID  map[string]*rsa.PublicKey
	cacheUntil time.Time
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func NewVerifier(jwksURL string) *Verifier {
	return &Verifier{
		jwksURL:    jwksURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cacheTTL:   5 * time.Minute,
		keysByKID:  make(map[string]*rsa.PublicKey),
	}
}

func (v *Verifier) VerifyToken(ctx context.Context, token string) (map[string]any, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("empty token")
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid")
		}
		key, err := v.getKey(ctx, kid)
		if err != nil {
			return nil, err
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}))
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("token not valid")
	}
	out := make(map[string]any, len(claims))
	for k, val := range claims {
		out[k] = val
	}
	return out, nil
}

func (v *Verifier) getKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keysByKID[kid]
	validCache := time.Now().Before(v.cacheUntil)
	v.mu.RUnlock()
	if ok && validCache {
		return key, nil
	}
	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok = v.keysByKID[kid]
	if !ok {
		return nil, fmt.Errorf("kid %s not found", kid)
	}
	return key, nil
}

func (v *Verifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("jwks status %d", resp.StatusCode)
	}
	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	next := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if !strings.EqualFold(k.Kty, "RSA") || k.Kid == "" {
			continue
		}
		pub, err := rsaFromModExp(k.N, k.E)
		if err != nil {
			continue
		}
		next[k.Kid] = pub
	}
	if len(next) == 0 {
		return errors.New("no usable jwks keys")
	}
	v.mu.Lock()
	v.keysByKID = next
	v.cacheUntil = time.Now().Add(v.cacheTTL)
	v.mu.Unlock()
	return nil
}

func rsaFromModExp(nEnc, eEnc string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(nEnc)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(eEnc)
	if err != nil {
		return nil, err
	}
	if len(nb) == 0 || len(eb) == 0 {
		return nil, errors.New("invalid rsa params")
	}
	n := new(big.Int).SetBytes(nb)
	e := int(new(big.Int).SetBytes(eb).Int64())
	if e <= 1 {
		return nil, errors.New("invalid exponent")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}
