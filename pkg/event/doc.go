// Package event provides a complete, reusable in-process event bus mechanism.
//
// It is self-contained and does not depend on the application's internal
// packages, so it can be reused across projects. It offers:
//
//   - EventBus interface with a default implementation
//   - Event, EventHandler, HandlerPriority and SubscriptionOptions types
//   - synchronous and asynchronous dispatch
//   - priority-ordered subscribers (zero value is normal priority; stable
//     order for equal priorities)
//   - wildcard subscription (e.g. "user.*" matches "user.created")
//   - per-subscription cancel funcs returned by Subscribe/SubscribeWildcard
//   - panic recovery and error logging per handler
//   - sentinel errors (ErrClosed, ErrMaxSubscribers, ErrInvalidPattern) for
//     errors.Is checks
//
// The bus is safe for concurrent use. Publish holds the lock only to collect
// subscriber snapshots and to register async tasks, so handlers may safely
// subscribe/unsubscribe, and Close waits for all queued async handlers.
//
// Business-specific event definitions (event names, payloads and publish
// helpers) belong in the application's internal/event package, which builds
// on top of this package.
package event
