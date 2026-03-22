package tasks

import "errors"

// ErrInvalidTaskState indica que la tarea no admite la operación en su estado actual.
var ErrInvalidTaskState = errors.New("invalid task state")
