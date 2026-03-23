package memory

import "errors"

var (
	ErrNotFound        = errors.New("memory entry not found")
	ErrVersionConflict = errors.New("memory version conflict")
)

// IsNotFound verifica si el error es de entrada no encontrada.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsVersionConflict verifica si el error es de conflicto de versión.
func IsVersionConflict(err error) bool {
	return errors.Is(err, ErrVersionConflict)
}
