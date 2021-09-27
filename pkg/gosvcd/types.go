package gosvcd

// ServiceId is a globally unique identifier for the service
// TODO(JM): How to assign these nicely? A compile-time construction
// would be useful. Preferably without having an external tool.
type ServiceId int64

// TODO: Should events be tightly associated with a specific service?
// This would make introspection easier as we could produce the graph
// of services and the events they may emit.
type EventType string

// ServiceDaemon performs service registration, initialization and event
// dispatching.
type ServiceDaemonBuilder interface {
	// Register a service.
	Register(svc Service)

	// Start the daemon.
	Start() ServiceDaemon
}

type ServiceDaemon interface {
	// Shutdown stops all services and the daemon.
	Shutdown()
}

// ServiceHandle contains the set of operations common to all services.
type ServiceHandle interface {
	EmitEvent(eventType EventType, data interface{})
	Unregister()
}

// Service
type Service interface {
	// Id returns the globally unique identifier for the service.
	// The identifier can be used by other services to declare themselves
	// to be dependent on this service.
	ID() ServiceId
	
	// Name is a human readable description of the service.
	Name() string

	// Dependencies returns the upstream dependencies of this service.
	// If a service B depends on service A, then A will be initialized
	// before B, and if both A and B subscribe to event type E, then A
	// will receive the event E before B.
	// 
	// TODO: Do we actually need this in both directions? Sometimes one
	// might need to be initialized after some service, but it may want
	// to handle events before it.
	Dependencies() []ServiceId

	// Subscriptions returns the list of events that this service is subscribed
	// to.
	// TODO: Is it fine that this is static?
	Subscriptions() []EventType

	// Initialize the service. Invoked after all services listed by Dependencies()
	// are initialized
	Init(handle ServiceHandle)

	// HandleEvent is called when an event of a type that is mentioned in Subscriptions()
	// is emitted.
	HandleEvent(event Event)

	// Shutdown the service
	Shutdown()
}

type Event interface {
	// The service that emitted this event.
	ServiceId() ServiceId

	// The event type
	EventType() EventType

	// Data, if any, associated with the event.
	Data() interface{}
}
