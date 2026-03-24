package connectors

import (
	"errors"

	"github.com/devpablocristo/core/backend/go/domainerr"
)

var (
	ErrNotFound = domainerr.NotFound("not found")
	ErrDisabled         = errors.New("connector is disabled")
	ErrUngated          = errors.New("execution requires review approval")
	ErrOperationUnknown = errors.New("unknown operation for connector")
)

// IsNotFound verifica si el error es de conector no encontrado.
func IsNotFound(err error) bool {
	return domainerr.IsNotFound(err)
}

// IsUngated verifica si la ejecución no tiene aprobación de Review.
func IsUngated(err error) bool {
	return errors.Is(err, ErrUngated)
}
