// Package event defines business-specific events for the application.
//
// It only contains business-related event definitions: event name constants,
// payload types and publish helper functions (e.g. application lifecycle
// events). The underlying event bus mechanism (EventBus interface, Event,
// EventHandler, subscription options, implementation, tests and examples)
// lives in the reusable pkg/event package.
//
// To add a new business event, follow this pattern in a domain file:
//
//   - define an event name constant (e.g. EventUserCreated = "user.created")
//   - define a payload type (e.g. UserCreatedPayload)
//   - define a publish helper that accepts a pkgevent.EventBus
package event
