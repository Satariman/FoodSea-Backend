package domain

import "errors"

var (
	// ErrTransient signals a temporary failure that the orchestrator should retry.
	ErrTransient = errors.New("transient error")

	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrManualIntervention is returned when compensation exhausted all retry attempts
	// and the saga is stuck in a partially-compensated state requiring human action.
	ErrManualIntervention = errors.New("manual intervention required")
)
