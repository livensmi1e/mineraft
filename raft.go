package main

import (
	"context"
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

func (s *server) lastLog() (index, term int) {
	index = len(s.log)
	if index == 0 {
		return index, -1
	}
	return index, s.log[index-1].Term
}

func (s *server) beLeader() {
	s.mu.Lock()
	s.state = leader
	lastLogIndex, _ := s.lastLog()
	for _, p := range s.peers {
		if p == s.me {
			continue
		}
		s.nextIndex[p] = lastLogIndex + 1
		s.matchIndex[p] = 0
	}
	s.mu.Unlock()

	var wg sync.WaitGroup
	for _, p := range s.peers {
		if p == s.me {
			continue
		}
		wg.Go(func() {
			lastLogIndex, lastLogTerm := s.lastLog()
			reply, _ := s.appendEntries(context.Background(), p, appendEntriesArgs{
				Term:         s.currentTerm,
				LeaderID:     s.me,
				PrevLogIndex: lastLogIndex,
				PrevLogTerm:  lastLogTerm,
				LeaderCommit: s.commitIndex,
			})

			// todo: process reply
			reply.Term++
		})
	}
	wg.Wait()
}

// Locked:
//
//	become candidate
//	increment term
//	vote for self
//	snapshot term + last log
//
// Unlocked:
//
//	send RequestVote RPCs
//
// Locked for each reply:
//
//	verify reply is still relevant
//	update state
func (s *server) candidateDoElection() {
	// become candidate, start election
	s.mu.Lock()
	s.state = candidate
	s.currentTerm++
	s.votedFor = s.me
	s.resetElectionTimer()

	lastLogIndex, lastLogTerm := s.lastLog()
	electionTerm := s.currentTerm
	count, total := 1, len(s.peers)
	s.mu.Unlock()

	for _, p := range s.peers {
		if p == s.me {
			continue
		}
		go func() {
			reply, _ := s.requestVote(context.Background(), p, requestVoteArgs{
				Term:         electionTerm,
				CandidateID:  s.me, // me never change, safe
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			})

			s.mu.Lock()
			if reply.Term > s.currentTerm {
				s.state = follower
				s.votedFor = ""
				s.currentTerm = reply.Term
				s.resetElectionTimer()
				s.mu.Unlock()
				return
			}
			valid := s.validCandidateInElection(electionTerm)
			if valid {
				if reply.VoteGranted {
					count++
				}
				if majority(count, total) {
					s.mu.Unlock()
					s.beLeader()
				}
			} else {
				s.mu.Unlock()
				return // stale
			}
		}()
	}
}

func (s *server) handleTimeout() {
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()

	if state == leader {
		return
	}
	s.candidateDoElection()
}

// run is the starting point for election timers, heartbeats, and applying
// committed entries.
func (s *server) run() {
	defer close(s.done)
	for {
		select {
		case <-s.stopCh:
			return
		case <-s.timer.C:
			s.handleTimeout()
		}
	}

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

// RequestVote is called concurrently by the simulated network.
func (s *server) RequestVote(ctx context.Context, args requestVoteArgs) (requestVoteReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if args.Term < s.currentTerm {
		return requestVoteReply{Term: s.currentTerm, VoteGranted: false}, nil
	}
	if args.Term > s.currentTerm {
		s.currentTerm = args.Term
		s.votedFor = ""
		s.state = follower
	}

	lastLogIndex, lastLogTerm := s.lastLog()
	canVote := s.votedFor == "" || s.votedFor == args.CandidateID
	logIsUpToDate := (args.LastLogTerm > lastLogTerm) || (args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex)

	if canVote && logIsUpToDate {
		s.votedFor = args.CandidateID
		s.resetElectionTimer()
		return requestVoteReply{Term: s.currentTerm, VoteGranted: true}, nil
	}

	return requestVoteReply{Term: s.currentTerm, VoteGranted: false}, nil
}

// AppendEntries handles both heartbeats and log replication.
func (s *server) AppendEntries(ctx context.Context, args appendEntriesArgs) (appendEntriesReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if args.Term < s.currentTerm {
		return appendEntriesReply{Term: s.currentTerm, Success: false}, nil
	}

	// valid leader request, reset timer
	s.resetElectionTimer()

	if s.state == candidate {
		s.state = follower
	}
	if args.Term > s.currentTerm {
		s.currentTerm = args.Term
		s.votedFor = ""
	}

	if args.PrevLogIndex > len(s.log) {
		return appendEntriesReply{Term: s.currentTerm, Success: false}, nil
	}
	// if prevLogIndex <= 0 {
	// 	leader append all new entries to server's log, so not need to find matching entry
	// }
	if args.PrevLogIndex >= 1 {
		entry := s.log[args.PrevLogIndex-1]
		if entry.Term != args.PrevLogTerm {
			return appendEntriesReply{Term: s.currentTerm, Success: false}, nil
		}
	}

	conflictIndex := -1
	for i, entry := range args.Entries {
		index := args.PrevLogIndex + i + 1
		if index > len(s.log) {
			conflictIndex = len(s.log) + 1
			break
		}
		if entry.Term != s.log[index-1].Term {
			conflictIndex = index
			break
		}
	}
	if conflictIndex != -1 {
		s.log = s.log[0 : conflictIndex-1]
		for _, entry := range args.Entries[conflictIndex-args.PrevLogIndex-1 : len(args.Entries)] {
			s.log = append(s.log, entry)
		}
	}
	if args.LeaderCommit > s.commitIndex {
		lastNewEntryIndex := len(args.Entries) + args.PrevLogIndex
		s.commitIndex = min(args.LeaderCommit, lastNewEntryIndex)
	}

	return appendEntriesReply{Term: s.currentTerm, Success: true}, nil
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
		ID:          s.me,
		State:       s.state,
		CurrentTerm: s.currentTerm,
		VotedFor:    s.votedFor,
		CommitIndex: s.commitIndex,
		LastApplied: s.lastApplied,
		Log:         append([]logEntry{}, s.log...),
	}
}

func (s *server) resetElectionTimer() {
	if !s.timer.Stop() {
		select {
		case <-s.timer.C:
		default:
		}
	}
	s.timer.Reset(randomElectionTimeout())
}

// assume lock is acquired
func (s *server) validCandidateInElection(electionTerm int) bool {
	return s.state == candidate && electionTerm == s.currentTerm
}

func randomElectionTimeout() time.Duration {
	return minElectionTimeout + time.Duration(rand.Int63n(int64(maxElectionTimeout-minElectionTimeout)))
}

func majority(count, total int) bool {
	return count >= total/2+1
}
