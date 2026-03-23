package connectors

import "errors"

var (
	ErrNotFound         = errors.New("connector not found")
	ErrDisabled         = errors.New("connector is disabled")
	ErrUngated          = errors.New("execution requires review approval")
	ErrOperationUnknown = errors.New("unknown operation for connector")
)

// IsNotFound verifica si el error es de conector no encontrado.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsUngated verifica si la ejecución no tiene aprobación de Review.
func IsUngated(err error) bool {
	return errors.Is(err, ErrUngated)
}
