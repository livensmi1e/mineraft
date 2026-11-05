package main

import (
	"errors"
	"sort"
	"sync"
	"time"
)

type nodeSnapshot struct {
	ID           id         `json:"id"`
	State        state      `json:"state"`
	CurrentTerm  int        `json:"currentTerm"`
	VotedFor     id         `json:"votedFor,omitempty"`
	CommitIndex  int        `json:"commitIndex"`
	LastApplied  int        `json:"lastApplied"`
	Log          []logEntry `json:"log"`
	NetworkReady bool       `json:"networkReady"`
}

type networkSnapshot struct {
	Mode          networkMode `json:"mode"`
	Nodes         map[id]bool `json:"nodes"`
	DisabledLinks []link      `json:"disabledLinks"`
}

type clusterSnapshot struct {
	Nodes   []nodeSnapshot  `json:"nodes"`
	Network networkSnapshot `json:"network"`
}

type cluster struct {
	servers map[id]*server
	network *network
	events  *eventHub
}

func newCluster(nodeIDs []id) (*cluster, error) {
	if len(nodeIDs) == 0 {
		return nil, errors.New("cluster needs at least one node")
	}

	events := newEventHub()
	network := newNetwork(time.Now().UnixNano(), events)
	c := &cluster{
		servers: make(map[id]*server, len(nodeIDs)),
		network: network,
		events:  events,
	}

	for _, nodeID := range nodeIDs {
		if _, exists := c.servers[nodeID]; exists {
			c.close()
			return nil, errors.New("node IDs must be unique")
		}
		s := newServer(nodeID, nodeIDs, network, 0, "", nil)
		if err := network.register(nodeID, s); err != nil {
			s.stop()
			c.close()
			return nil, err
		}
		c.servers[nodeID] = s
	}
	return c, nil
}

func (c *cluster) close() {
	c.network.close()
	for _, s := range c.servers {
		s.stop()
	}
	c.events.close()
}

func (c *cluster) snapshot() clusterSnapshot {
	network := c.network.snapshot()
	nodes := make([]nodeSnapshot, 0, len(c.servers))
	for _, s := range c.servers {
		node := s.snapshot()
		node.NetworkReady = network.Nodes[node.ID]
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return clusterSnapshot{Nodes: nodes, Network: network}
}

type eventHub struct {
	mu          sync.Mutex
	subscribers map[chan rpcEvent]struct{}
	closed      bool
}

func newEventHub() *eventHub {
	return &eventHub{subscribers: make(map[chan rpcEvent]struct{})}
}

func (h *eventHub) publish(event rpcEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	event.At = time.Now()
	for subscriber := range h.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (h *eventHub) subscribe() (<-chan rpcEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	stream := make(chan rpcEvent, 64)
	if !h.closed {
		h.subscribers[stream] = struct{}{}
	}
	return stream, func() {
		h.mu.Lock()
		delete(h.subscribers, stream)
		h.mu.Unlock()
	}
}

func (h *eventHub) close() {
	h.mu.Lock()
	h.closed = true
	h.subscribers = make(map[chan rpcEvent]struct{})
	h.mu.Unlock()
}
