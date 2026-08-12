package event

import "errors"

// Sentinel errors returned by the event bus.
var (
	// ErrClosed is returned when publishing to or subscribing on a closed bus.
	ErrClosed = errors.New("event: bus is closed")

	// ErrMaxSubscribers is returned when a subscribe exceeds the per-event
	// subscriber limit.
	ErrMaxSubscribers = errors.New("event: maximum subscribers reached")

	// ErrInvalidPattern is returned when a wildcard pattern is invalid.
	ErrInvalidPattern = errors.New("event: invalid wildcard pattern")
)
