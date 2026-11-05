package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var (
	errNetworkClosed = errors.New("network closed")
	errUnreachable   = errors.New("peer unreachable")
	errDropped       = errors.New("rpc dropped")
)

type rpcHandler interface {
	RequestVote(context.Context, requestVoteArgs) (requestVoteReply, error)
	AppendEntries(context.Context, appendEntriesArgs) (appendEntriesReply, error)
}

type networkMode string

const (
	reliableNetwork   networkMode = "reliable"
	unreliableNetwork networkMode = "unreliable"
)

type link struct {
	From id `json:"from"`
	To   id `json:"to"`
}

type rpcEvent struct {
	At       time.Time `json:"at"`
	From     id        `json:"from"`
	To       id        `json:"to"`
	Method   string    `json:"method"`
	Stage    string    `json:"stage"`
	Term     int       `json:"term"`
	Error    string    `json:"error,omitempty"`
}

type network struct {
	mu sync.RWMutex

	handlers map[id]rpcHandler
	nodes    map[id]bool
	links    map[link]bool
	mode     networkMode
	closed   bool

	randomMu sync.Mutex
	random   *rand.Rand

	events *eventHub
}

func newNetwork(seed int64, events *eventHub) *network {
	return &network{
		handlers: make(map[id]rpcHandler),
		nodes:    make(map[id]bool),
		links:    make(map[link]bool),
		mode:     reliableNetwork,
		random:   rand.New(rand.NewSource(seed)),
		events:   events,
	}
}

func (n *network) register(nodeID id, handler rpcHandler) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return errNetworkClosed
	}
	if _, exists := n.handlers[nodeID]; exists {
		return fmt.Errorf("register node %q: already registered", nodeID)
	}

	n.handlers[nodeID] = handler
	n.nodes[nodeID] = true
	return nil
}

func (n *network) close() {
	n.mu.Lock()
	n.closed = true
	n.mu.Unlock()
}

func (n *network) requestVote(ctx context.Context, from, to id, args requestVoteArgs) (requestVoteReply, error) {
	var reply requestVoteReply
	err := n.call(ctx, from, to, "RequestVote", args.Term, func(handler rpcHandler) error {
		var err error
		reply, err = handler.RequestVote(ctx, args)
		return err
	})
	return reply, err
}

func (n *network) appendEntries(ctx context.Context, from, to id, args appendEntriesArgs) (appendEntriesReply, error) {
	args.Entries = append([]logEntry(nil), args.Entries...)
	var reply appendEntriesReply
	err := n.call(ctx, from, to, "AppendEntries", args.Term, func(handler rpcHandler) error {
		var err error
		reply, err = handler.AppendEntries(ctx, args)
		return err
	})
	return reply, err
}

func (n *network) call(
	ctx context.Context,
	from, to id,
	method string,
	term int,
	invoke func(rpcHandler) error,
) error {
	n.events.publish(rpcEvent{From: from, To: to, Method: method, Stage: "sent", Term: term})

	handler, mode, err := n.route(from, to)
	if err != nil {
		n.publishFailure(from, to, method, term, "unreachable", err)
		return err
	}

	if err := waitFor(ctx, n.delay(mode)); err != nil {
		n.publishFailure(from, to, method, term, "cancelled", err)
		return err
	}
	if mode == unreliableNetwork && n.chance(0.15) {
		n.publishFailure(from, to, method, term, "dropped", errDropped)
		return errDropped
	}

	n.events.publish(rpcEvent{From: from, To: to, Method: method, Stage: "delivered", Term: term})
	if err := invoke(handler); err != nil {
		n.publishFailure(from, to, method, term, "rejected", err)
		return err
	}

	if err := waitFor(ctx, n.delay(mode)); err != nil {
		n.publishFailure(to, from, method, term, "cancelled", err)
		return err
	}
	if _, _, err := n.route(to, from); err != nil {
		n.publishFailure(to, from, method, term, "unreachable", err)
		return err
	}
	if mode == unreliableNetwork && n.chance(0.15) {
		n.publishFailure(to, from, method, term, "dropped", errDropped)
		return errDropped
	}

	n.events.publish(rpcEvent{From: to, To: from, Method: method, Stage: "replied", Term: term})
	return nil
}

func (n *network) route(from, to id) (rpcHandler, networkMode, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.closed {
		return nil, n.mode, errNetworkClosed
	}
	handler, exists := n.handlers[to]
	if !exists || !n.nodes[from] || !n.nodes[to] || !n.linkEnabledLocked(from, to) {
		return nil, n.mode, errUnreachable
	}
	return handler, n.mode, nil
}

func (n *network) setMode(mode networkMode) error {
	if mode != reliableNetwork && mode != unreliableNetwork {
		return fmt.Errorf("unknown network mode %q", mode)
	}
	n.mu.Lock()
	n.mode = mode
	n.mu.Unlock()
	return nil
}

func (n *network) setNode(nodeID id, enabled bool) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.handlers[nodeID]; !exists {
		return fmt.Errorf("unknown node %q", nodeID)
	}
	n.nodes[nodeID] = enabled
	return nil
}

func (n *network) setLink(from, to id, enabled bool) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.handlers[from]; !exists {
		return fmt.Errorf("unknown node %q", from)
	}
	if _, exists := n.handlers[to]; !exists {
		return fmt.Errorf("unknown node %q", to)
	}
	if from == to {
		return errors.New("cannot configure a node's loopback link")
	}
	n.links[link{From: from, To: to}] = enabled
	return nil
}

func (n *network) linkEnabledLocked(from, to id) bool {
	enabled, configured := n.links[link{From: from, To: to}]
	return !configured || enabled
}

func (n *network) snapshot() networkSnapshot {
	n.mu.RLock()
	defer n.mu.RUnlock()

	nodes := make(map[id]bool, len(n.nodes))
	for nodeID, enabled := range n.nodes {
		nodes[nodeID] = enabled
	}
	disabledLinks := make([]link, 0)
	for configuredLink, enabled := range n.links {
		if !enabled {
			disabledLinks = append(disabledLinks, configuredLink)
		}
	}
	return networkSnapshot{Mode: n.mode, Nodes: nodes, DisabledLinks: disabledLinks}
}

func (n *network) publishFailure(from, to id, method string, term int, stage string, err error) {
	n.events.publish(rpcEvent{
		From:    from,
		To:      to,
		Method:  method,
		Stage:   stage,
		Term:    term,
		Error:   err.Error(),
	})
}

func (n *network) delay(mode networkMode) time.Duration {
	if mode == reliableNetwork {
		return 35 * time.Millisecond
	}
	n.randomMu.Lock()
	delay := 40 + n.random.Intn(181)
	n.randomMu.Unlock()
	return time.Duration(delay) * time.Millisecond
}

func (n *network) chance(probability float64) bool {
	n.randomMu.Lock()
	hit := n.random.Float64() < probability
	n.randomMu.Unlock()
	return hit
}

func waitFor(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
