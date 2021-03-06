package event

import (
	"fmt"
	"strings"
	"sync"

	"github.com/floeit/floe/config"
	nt "github.com/floeit/floe/config/nodetype"
	"github.com/floeit/floe/log"
)

const sysPrefix = "sys." // all internal events that nodes can not see

// HostedIDRef is any ID unique within the scope of the host that created it.
type HostedIDRef struct {
	HostID string
	ID     int64
}

func (h HostedIDRef) String() string {
	if h.ID == 0 {
		return "na"
	}
	return fmt.Sprintf("%s-%d", h.HostID, h.ID)
}

// Equal returns true if h and g are considered equal
func (h HostedIDRef) Equal(g HostedIDRef) bool {
	return h.HostID == g.HostID && h.ID == g.ID
}

// Equals compares receiver with param rh
func (h HostedIDRef) Equals(rh HostedIDRef) bool {
	return h.HostID == rh.HostID && h.ID == rh.ID
}

// RunRef uniquely identifies and routes a particular run across the whole cluster
type RunRef struct {
	// FlowRef identifies the flow that this reference relates to
	FlowRef config.FlowRef

	// Run identifies the host and id that this run was initiated by.
	// This is a cluster unique reference, which may not refer to the node that is
	// executing the Run (that will be defined by ExecHost)
	Run HostedIDRef

	// ExecHost is the host that is actually executing, or executed this event,
	// use in conjunction with Run to find the active and archived run
	ExecHost string
}

func (r RunRef) String() string {
	return fmt.Sprintf("runref_%s_%s", r.FlowRef, r.Run)
}

// Equal returns true ir r and s are considered to refer to the same thing
func (r RunRef) Equal(s RunRef) bool {
	return r.FlowRef.Equal(s.FlowRef) && r.Run.Equal(s.Run)
}

// Adopted means that this RunRef has been added to a pending list and been assigned a
// unique run ID
func (r RunRef) Adopted() bool {
	if r.Run.ID == 0 {
		return false
	}
	return true
}

// Observer defines the interface for observers.
type Observer interface {
	Notify(e Event)
}

// Event defines a moment in time thing occurring
type Event struct {
	// RunRef if this event is in the scope of a specific run
	// if nil then is a general event that could be routed to triggers
	RunRef RunRef

	// SourceNode is the Ref of the node in the context of a RunRef
	SourceNode config.NodeRef

	// Tag is the label that helps route the event.
	// it will match node.Type for trigger nodes, and node.Listen for task and merge nodes.
	Tag string

	// Good specifically when this is classed as a good event
	Good bool

	// Unique and ordered event ID within a Run. An ID greater than another
	// ID must have happened after it within the context of the RunRef.
	// A flow initiating trigger will have ID 1.
	ID int64

	// Opts - some optional data in the event
	Opts nt.Opts
}

// copy makes a copy without sharing the underlying Opts aps.
// Any pointers in the opts map (there should not be) will share memory
func (e Event) copy() Event {
	newE := e
	// break the common map memory link
	newE.Opts = nt.Opts{}
	for k, v := range e.Opts {
		newE.Opts[k] = v
	}
	return newE
}

// SetGood sets this event as a good event
func (e *Event) SetGood() {
	e.Good = true
	e.Tag = fmt.Sprintf("%s.%s.good", e.SourceNode.Class, e.SourceNode.ID)
}

// IsSystem returns true if the event is a internal system event
func (e *Event) IsSystem() bool {
	if len(e.Tag) < 3 {
		return false
	}
	return strings.HasPrefix(e.Tag, sysPrefix)
}

// Queue is not strictly a queue, it just distributes all events to the observers
type Queue struct {
	sync.RWMutex

	idCounter int64
	// observers are any entities that care about events emitted from the queue
	observers []Observer
}

// Register registers an observer to this q
func (q *Queue) Register(o Observer) {
	q.observers = append(q.observers, o)
}

// Publish sends an event to all the observers
func (q *Queue) Publish(e Event) {
	q.Lock()
	// grab the next event ID
	q.idCounter++
	e.ID = q.idCounter
	if e.Opts == nil {
		e.Opts = nt.Opts{}
	}
	q.Unlock()

	// node updates can be noisy - an event is issued for every line of output
	// if e.Tag != "sys.node.update" {
	// for helpfulness indicate if this event was issued by an already adopted flow
	isTrig := " (trigger)"
	if e.RunRef.Adopted() {
		isTrig = ""
	}
	log.Debugf("<%s-ev:%d> - queue publish type:<%s>%s from: %s", e.RunRef, e.ID, e.Tag, isTrig, e.SourceNode)
	// }

	// and notify all observers - in background goroutines
	for _, o := range q.observers {
		// send separate copies to each observer to avoid any races
		go o.Notify(e.copy())
	}
}
