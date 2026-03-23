package watchers

import "errors"

// ErrNotFound indica que el watcher o proposal no existe.
var ErrNotFound = errors.New("watcher not found")

// ErrWatcherDisabled indica que el watcher está deshabilitado.
var ErrWatcherDisabled = errors.New("watcher is disabled")
