package hub

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/floeit/floe/config"
	nt "github.com/floeit/floe/config/nodetype"
	"github.com/floeit/floe/event"
	"github.com/floeit/floe/exe/git"
	"github.com/floeit/floe/store"
)

type timerTrigger func(*event.Queue, *timer)

type timer struct {
	flow    config.FlowRef
	nodeID  string
	period  int          // time between triggers in seconds
	next    time.Time    // computed next time to run
	trigger timerTrigger // the function to fire
	opts    nt.Opts
}

type timers struct {
	mu   sync.RWMutex
	list map[string]*timer
}

func newTimers(q *event.Queue) *timers {
	t := &timers{
		list: map[string]*timer{},
	}

	go func() {
		for now := range time.Tick(time.Second) {
			t.mu.RLock()
			for name, tim := range t.list {
				if !now.After(tim.next) {
					continue
				}
				tim.next = now.Add(time.Duration(tim.period) * time.Second)
				log.Debugf("<%s> - timer trigger", name)
				tim.trigger(q, tim)
			}
			t.mu.RUnlock()
		}
	}()
	return t
}

func (t *timers) register(flow config.FlowRef, nodeID string, opts nt.Opts, trigger timerTrigger) {
	period, ok := opts["period"].(int)
	if !ok {
		period = 10
	}
	t.mu.Lock()
	t.list[flow.String()+"-"+nodeID] = &timer{
		flow:    flow,
		nodeID:  nodeID,
		period:  period,
		next:    time.Now().UTC().Add(time.Duration(period) * time.Second),
		trigger: trigger,
		opts:    opts,
	}
	t.mu.Unlock()
}

func sendTriggerEvent(q *event.Queue, flowRef config.FlowRef, nodeID, typ string, opts nt.Opts) {
	log.Debugf("<%s> - from %s trigger <%s> added to pending", flowRef, typ, nodeID)
	q.Publish(event.Event{
		RunRef: event.RunRef{
			FlowRef: flowRef,
		},
		Tag: "inbound." + typ,
		SourceNode: config.NodeRef{
			Class: "trigger",
			ID:    nodeID,
		},
		Opts: opts,
	})
}

func startFlowTrigger(q *event.Queue, tim *timer) {
	sendTriggerEvent(q, tim.flow, tim.nodeID, "timer", tim.opts)
}

const pollStoreRoot = "refs"

type repoPoller struct {
	store   store.Store
	nodeID  string
	url     string
	refs    string
	exclude string
	gitKey  string
}

func newRepoPoller(store store.Store, nodeID, gitKey string, opts nt.Opts) *repoPoller {
	rp := &repoPoller{
		store:  store,
		nodeID: nodeID,
		gitKey: gitKey,
	}

	rp.url, _ = opts["url"].(string)
	rp.refs, _ = opts["refs"].(string)
	rp.exclude, _ = opts["exclude"].(string)

	if rp.url == "" {
		return nil
	}
	if rp.refs == "" {
		rp.refs = "refs/*"
	}

	return rp
}

func (r *repoPoller) timer(q *event.Queue, tim *timer) {
	prev, err := r.loadRefs(tim.flow.ID)
	if err != nil {
		log.Errorf("<%s> - could not load previous refs: %s", tim.flow, err)
	}

	log.Debugf("<%s> - checking repo <%s> for changes matching <%s>", tim.flow, r.url, r.refs)
	new, output, ok := git.Ls(r.url, r.refs, r.exclude, r.gitKey)
	if !ok {
		log.Errorf("<%s> - could not get new refs: %s", tim.flow, err)
		log.Errorf(strings.Join(output, "\n"))
		return
	}

	err = r.saveRefs(tim.flow.ID, *new)
	if err != nil {
		log.Errorf("<%s> - could not save refs: %s", tim.flow, err)
	}

	changes := changedRefs(prev, *new)

	// start a pending flow for each changed hash
	for _, ref := range changes.Hashes {
		opts := nt.Opts{
			"url":    r.url,
			"branch": ref.Name,
			"hash":   ref.Hash,
		}
		log.Debugf("<%s> - found changed branch: <%s>", tim.flow, ref.Name)
		sendTriggerEvent(q, tim.flow, r.nodeID, "poll-git", opts)
	}
}

func changedRefs(old, new git.Hashes) (changed git.Hashes) {
	if old.RepoURL != new.RepoURL {
		return changed
	}
	changed = git.Hashes{
		RepoURL: old.RepoURL,
		Hashes:  map[string]git.Ref{},
	}
	for key, n := range new.Hashes {
		o, ok := old.Hashes[key]
		if !ok {
			changed.Hashes[key] = n
			continue
		}
		if n.Hash != o.Hash {
			changed.Hashes[key] = n
		}
	}
	return changed
}

func (r *repoPoller) loadRefs(flowID string) (git.Hashes, error) {
	h := git.Hashes{}
	err := r.store.Load(repoKey(flowID, r.nodeID), &h)
	return h, err
}

func (r *repoPoller) saveRefs(flowID string, h git.Hashes) error {
	return r.store.Save(repoKey(flowID, r.nodeID), h)
}

func repoKey(flowID, nodeID string) string {
	return filepath.Join(pollStoreRoot, flowID, nodeID)
}
