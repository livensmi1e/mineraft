package main

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"
)

type state string

const (
	follower  state = "follower"
	candidate state = "candidate"
	leader    state = "leader"
)

const (
	minElectionTimeout = 150 * time.Millisecond
	maxElectionTimeout = 300 * time.Millisecond
)

var errRaftNotImplemented = errors.New("raft handler not implemented")

type id string

type logEntry struct {
	Command string `json:"command"`
	Term    int    `json:"term"`
}

// These RPC values mirror Figure 2 of the Raft paper. The transport only moves
// them; deciding how to answer them is deliberately left to the Raft core.
type requestVoteArgs struct {
	Term         int `json:"term"`
	CandidateID  id  `json:"candidateId"`
	LastLogIndex int `json:"lastLogIndex"`
	LastLogTerm  int `json:"lastLogTerm"`
}

type requestVoteReply struct {
	Term        int  `json:"term"`
	VoteGranted bool `json:"voteGranted"`
}

type appendEntriesArgs struct {
	Term         int        `json:"term"`
	LeaderID     id         `json:"leaderId"`
	PrevLogIndex int        `json:"prevLogIndex"`
	PrevLogTerm  int        `json:"prevLogTerm"`
	Entries      []logEntry `json:"entries"`
	LeaderCommit int        `json:"leaderCommit"`
}

type appendEntriesReply struct {
	Term    int  `json:"term"`
	Success bool `json:"success"`
}

type server struct {
	mu sync.RWMutex

	me        id
	peers     []id
	state     state
	transport *network

	// Volatile state on all servers.
	commitIndex int
	lastApplied int

	// Volatile state on leaders.
	nextIndex  map[id]int
	matchIndex map[id]int

	// Persistent state on all servers.
	currentTerm int
	votedFor    id
	log         []logEntry

	timer  *time.Timer
	stopCh chan struct{}
	done   chan struct{}
}

func newServer(me id, peers []id, transport *network, currentTerm int, votedFor id, log []logEntry) *server {
	s := &server{
		me:          me,
		peers:       append([]id(nil), peers...),
		state:       follower,
		transport:   transport,
		nextIndex:   make(map[id]int),
		matchIndex:  make(map[id]int),
		currentTerm: currentTerm,
		votedFor:    votedFor,
		log:         append([]logEntry(nil), log...),
		timer:       time.NewTimer(randomElectionTimeout()),
		stopCh:      make(chan struct{}),
		done:        make(chan struct{}),
	}

	go s.run()
	return s
}

func (s *server) lastLogIndex() int {
	return len(s.log)
}

// run is the starting point for election timers, heartbeats, and applying
// committed entries. Infrastructure shutdown already works; Raft behavior is
// intentionally yours to implement here.
func (s *server) run() {
	defer close(s.done)
	<-s.stopCh
}

func (s *server) stop() {
	select {
	case <-s.stopCh:
		return
	default:
		close(s.stopCh)
	}
	<-s.done
	if !s.timer.Stop() {
		select {
		case <-s.timer.C:
		default:
		}
	}
}

// RequestVote is called concurrently by the simulated network. Protect Raft
// state with s.mu while implementing the rules from Figure 2.
func (s *server) RequestVote(_ context.Context, _ requestVoteArgs) (requestVoteReply, error) {
	return requestVoteReply{}, errRaftNotImplemented
}

// AppendEntries handles both heartbeats and log replication.
func (s *server) AppendEntries(_ context.Context, _ appendEntriesArgs) (appendEntriesReply, error) {
	return appendEntriesReply{}, errRaftNotImplemented
}

// requestVote and appendEntries are the only outbound RPC calls the Raft core
// needs. Context cancellation is how callers stop waiting for failed peers.
func (s *server) requestVote(ctx context.Context, peer id, args requestVoteArgs) (requestVoteReply, error) {
	return s.transport.requestVote(ctx, s.me, peer, args)
}

func (s *server) appendEntries(ctx context.Context, peer id, args appendEntriesArgs) (appendEntriesReply, error) {
	return s.transport.appendEntries(ctx, s.me, peer, args)
}

func (s *server) snapshot() nodeSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return nodeSnapshot{
		ID:           s.me,
		State:        s.state,
		CurrentTerm:  s.currentTerm,
		VotedFor:     s.votedFor,
		CommitIndex:  s.commitIndex,
		LastApplied:  s.lastApplied,
		Log:          append([]logEntry{}, s.log...),
	}
}

func randomElectionTimeout() time.Duration {
	return minElectionTimeout + time.Duration(rand.Int63n(int64(maxElectionTimeout-minElectionTimeout)))
}
