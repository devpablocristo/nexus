package actors

import (
	"errors"
	"net/http"
	"strings"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
)

const (
	HeaderActorType = "X-Nexus-Actor-Type"
	HeaderActorID   = "X-Nexus-Actor-Id"
)

var ErrIncompleteActorHeaders = errors.New("incomplete actor headers")

func FromRequest(r *http.Request) (*sharedaudit.Actor, error) {
	actorType := strings.TrimSpace(r.Header.Get(HeaderActorType))
	actorID := strings.TrimSpace(r.Header.Get(HeaderActorID))
	if actorType == "" && actorID == "" {
		return nil, nil
	}
	if actorType == "" || actorID == "" {
		return nil, ErrIncompleteActorHeaders
	}
	return &sharedaudit.Actor{Type: actorType, ID: actorID}, nil
}
