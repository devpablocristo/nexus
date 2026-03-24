package watchers

import (
	"errors"

	"github.com/devpablocristo/core/backend/go/domainerr"
)

// ErrNotFound indica que el watcher o proposal no existe.
var ErrNotFound = domainerr.NotFound("not found")

// ErrWatcherDisabled indica que el watcher está deshabilitado.
var ErrWatcherDisabled = errors.New("watcher is disabled")
