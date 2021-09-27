package gosvcd

import (
	"fmt"
	"time"
)

// Example implementation of the ServiceDaemon and services.

//
// Event
//
type ExampleEvent struct {
	svc       Service
	eventType EventType
	data      interface{}
}

func (ev *ExampleEvent) ServiceId() ServiceId {
	return ev.svc.ID()
}

func (ev *ExampleEvent) EventType() EventType {
	return ev.eventType
}

func (ev *ExampleEvent) Data() interface{} {
	return ev.data
}

//
// Handle
//

type ExampleServiceHandle struct {
	Service
	evs chan *ExampleEvent
}

func (h *ExampleServiceHandle) EmitEvent(eventType EventType, data interface{}) {
	h.evs <- &ExampleEvent{h, eventType, data}
}

func (h *ExampleServiceHandle) Unregister() {
	panic("unimplemented")
}

//
// Builder
//

type ExampleServiceDaemonBuilder struct {
	handles map[ServiceId]*ExampleServiceHandle
	evs     chan *ExampleEvent
}

func NewBuilder() *ExampleServiceDaemonBuilder {
	return &ExampleServiceDaemonBuilder{
		handles: make(map[ServiceId]*ExampleServiceHandle),
		evs:     make(chan *ExampleEvent, 128),
	}
}

func (b *ExampleServiceDaemonBuilder) Register(svc Service) {
	h := &ExampleServiceHandle{svc, b.evs}
	b.handles[svc.ID()] = h
}

func (b *ExampleServiceDaemonBuilder) Start() ServiceDaemon {
	svcs, subs := toposortServices(b.handles)
	s := &ExampleServiceDaemon{
		handles:  b.handles,
		services: svcs,
		subs:     subs,
		evs:      b.evs,
	}
	fmt.Print("Services in dependency order: ")
	for _, svc := range svcs {
		fmt.Printf("%d ", svc.ID())
	}
	fmt.Println("")

	go s.run()

	return s
}

func popFirst(ids []ServiceId) (bool, ServiceId, []ServiceId) {
	if len(ids) == 0 {
		return false, -1, nil
	}
	id := ids[0]
	ids[0] = ids[len(ids)-1]
	return true, id, ids[0 : len(ids)-1]
}

func removeEdge(edge ServiceId, edges []ServiceId) []ServiceId {
	for i, e := range edges {
		if e == edge {
			edges[i] = edges[len(edges)-1]
			return edges[0 : len(edges)-1]
		}
	}
	panic(fmt.Sprintf("edge %v not found from %v", edge, edges))
}

func toposortServices(svcs map[ServiceId]*ExampleServiceHandle) ([]Service, map[EventType][]Service) {
	if len(svcs) == 0 {
		return nil, nil
	}

	sorted := []Service{}
	s := []ServiceId{}

	// Kahn's algorithm for topological sort
	in := make(map[ServiceId][]ServiceId)
	out := make(map[ServiceId][]ServiceId)
	edgesRemaining := 0
	for id, svc := range svcs {
		edges := make([]ServiceId, len(svc.Dependencies()))
		copy(edges, svc.Dependencies())

		in[id] = edges
		for _, depId := range edges {
			out[depId] = append(out[depId], id)
			edgesRemaining = edgesRemaining + 1
		}
		if len(edges) == 0 {
			s = append(s, id)
		}
	}

	fmt.Printf("in: %v\n", in)
	fmt.Printf("out: %v\n", out)

	if len(s) == 0 {
		panic("toposortServices: Services don't form a DAG!")
	}

	var (
		ok bool
		id ServiceId
	)
	for {
		ok, id, s = popFirst(s)
		if !ok {
			break
		}
		n := svcs[id]
		sorted = append(sorted, n)

		for _, depId := range out[id] {
			in[depId] = removeEdge(id, in[depId])
			edgesRemaining = edgesRemaining - 1
			if len(in[depId]) == 0 {
				s = append(s, depId)
			}
		}
		out[id] = []ServiceId{}
	}

	if edgesRemaining > 0 {
		panic("Service dependency graph is cyclic!")
	}

	servicesByEventType := make(map[EventType][]Service)

	for _, svc := range sorted {
		for _, evt := range svc.Subscriptions() {
			servicesByEventType[evt] = append(servicesByEventType[evt], svc)
		}
	}
	return sorted, servicesByEventType
}

//
// Daemon
//

type ExampleServiceDaemon struct {
	handles map[ServiceId]*ExampleServiceHandle

	// Services in dependency order, roots first.
	services []Service

	// Subscriptions, in topologically sorted order.
	subs map[EventType][]Service

	// Event channel
	evs chan *ExampleEvent
}

func (d *ExampleServiceDaemon) run() {

	// Initialize the services (in dependency order)
	for _, s := range d.services {
		s.Init(d.handles[s.ID()])
	}

	// Dispatch events to services
	chans := make(map[EventType]chan *ExampleEvent)

	for typ, svcs := range d.subs {
		ch := make(chan *ExampleEvent, 128)
		chans[typ] = ch
		go func() {
			for ev := range ch {
				for _, svc := range svcs {
					svc.HandleEvent(ev)
				}
			}
		}()
	}

	for ev := range d.evs {
		chans[ev.eventType] <- ev
	}

	for _, ch := range chans {
		close(ch)
	}
}

func (d *ExampleServiceDaemon) Shutdown() {
	// Shut down the services in reverse dependency order, starting
	// from the leafs.
	for i := len(d.services) - 1; i >= 0; i-- {
		d.services[i].Shutdown()
	}

	close(d.evs)
}

//
// Example event
//

var ExSomeEvent_Type = EventType("ExSomeEvent")

type ExSomeEvent struct {
	N int
}

//
// Example service
//

type ExService struct {
	id          ServiceId
	deps        []ServiceId
	eventSource bool
}

func (s *ExService) ID() ServiceId { return s.id }
func (s *ExService) Name() string  { return fmt.Sprintf("ExService%d", s.id) }
func (s *ExService) Dependencies() []ServiceId {
	return s.deps
}
func (s *ExService) Subscriptions() []EventType {
	return []EventType{ExSomeEvent_Type}
}
func (s *ExService) Init(handle ServiceHandle) {
	fmt.Println(s.Name() + ".Init")

	if s.eventSource {
		go func() {
			for i := 0; i < 10; i++ {
				time.Sleep(100 * time.Millisecond)
				handle.EmitEvent(ExSomeEvent_Type, &ExSomeEvent{i})
			}

		}()
	}
}
func (s *ExService) HandleEvent(event Event) {
	fmt.Printf("%s.HandleEvent [from %d, type %s]:  %v\n",
		s.Name(),
		event.ServiceId(),
		event.EventType(),
		event.Data())
}

func (s *ExService) Shutdown() {
	fmt.Println(s.Name() + ".Shutdown")
}

//
// Entrypoint
//

func RunExample() {

	builder := NewBuilder()

	builder.Register(&ExService{2, []ServiceId{0, 1}, false})
	builder.Register(&ExService{3, []ServiceId{2}, true})
	builder.Register(&ExService{0, []ServiceId{}, false})
	builder.Register(&ExService{1, []ServiceId{0}, false})

	daemon := builder.Start()

	time.Sleep(time.Second * 2)

	daemon.Shutdown()
}
