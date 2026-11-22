package main

import (
	"context"
	"math/rand"
	"slices"
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
	heartbeatInterval  = 50 * time.Millisecond // why 50ms
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

	db []string

	// Election timing is owned by run. Other goroutines update the deadline
	// while holding mu, then notify run to reprogram its local timer.
	electionDeadline time.Time
	electionResetCh  chan struct{}

	// Each follower has one worker. Signals are deliberately coalesced: a
	// worker always builds its request from the latest Raft state.
	replicateCh map[id]chan struct{}

	// electionVotes is meaningful only while state == candidate.
	electionVotes int

	// stopCh owns the server's background-work lifetime. It is intentionally
	// separate from context.Context, which is scoped to individual RPCs.
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func newServer(me id, peers []id, transport *network, currentTerm int, votedFor id, log []logEntry) *server {
	s := &server{
		me:               me,
		peers:            append([]id(nil), peers...),
		state:            follower,
		transport:        transport,
		nextIndex:        make(map[id]int), // 1 <= nextIndex <= len(log)+1
		matchIndex:       make(map[id]int),
		currentTerm:      currentTerm,
		votedFor:         votedFor,
		log:              append([]logEntry(nil), log...),
		db:               make([]string, 0),
		electionDeadline: time.Now().Add(randomElectionTimeout()),
		electionResetCh:  make(chan struct{}, 1),
		replicateCh:      make(map[id]chan struct{}),
		stopCh:           make(chan struct{}),
	}

	for _, peer := range s.peers {
		if peer == s.me {
			continue
		}
		work := make(chan struct{}, 1)
		s.replicateCh[peer] = work
		s.wg.Add(1)
		go s.replicationWorker(peer, work)
	}

	s.wg.Add(1)
	go s.run()
	return s
}

// lastLogLocked returns the index and term of the local last entry. The caller
// must hold s.mu.
func (s *server) lastLogLocked() (index, term int) {
	index = len(s.log)
	if index == 0 {
		return index, -1
	}
	return index, s.log[index-1].Term
}

// triggerReplication wakes every follower worker. The buffer size of one
// intentionally coalesces heartbeats and client commands while a worker is
// already replicating.
func (s *server) triggerReplication() {
	for _, peer := range s.peers {
		if peer == s.me {
			continue
		}
		select {
		case s.replicateCh[peer] <- struct{}{}:
		default:
		}
	}
}

// replicationWorker is the sole sender of AppendEntries to peer. Serializing
// one peer's requests makes a failure reply unambiguously refer to the latest
// nextIndex used by this worker.
func (s *server) replicationWorker(peer id, work <-chan struct{}) {
	defer s.wg.Done()
	for {
		select {
		case <-s.stopCh:
			return
		case <-work:
			s.replicatePeer(peer)
		}
	}
}

// replicatePeer catches peer up for one trigger. Network failures return to
// the worker; a later heartbeat or client command retries without recursion.
func (s *server) replicatePeer(peer id) {
	for {
		s.mu.Lock()
		if s.state != leader {
			s.mu.Unlock()
			return
		}
		request := s.appendEntriesRequestLocked(peer)
		s.mu.Unlock()

		reply, err := s.appendEntries(context.Background(), peer, request)
		if err != nil {
			return
		}

		s.mu.Lock()
		if reply.Term > s.currentTerm {
			s.becomeFollowerLocked(reply.Term)
			s.resetElectionDeadlineLocked()
			s.mu.Unlock()
			s.notifyElectionTimer()
			return
		}
		if s.state != leader || s.currentTerm != request.Term {
			s.mu.Unlock()
			return
		}

		if reply.Success {
			lastReplicatedIndex := request.PrevLogIndex + len(request.Entries)
			s.matchIndex[peer] = lastReplicatedIndex
			s.nextIndex[peer] = lastReplicatedIndex + 1
			commitAdvanced := s.advanceCommitIndexLocked()
			s.mu.Unlock()
			if commitAdvanced {
				// Promptly carry the new LeaderCommit to every follower.
				s.triggerReplication()
			}
			return
		}

		if s.nextIndex[peer] == 1 {
			s.mu.Unlock()
			return
		}
		s.nextIndex[peer]--
		s.mu.Unlock()
	}
}

// appendEntriesRequestLocked snapshots the latest state for one follower.
func (s *server) appendEntriesRequestLocked(peer id) appendEntriesArgs {
	nextIndex := s.nextIndex[peer]
	prevLogIndex, prevLogTerm := nextIndex-1, -1
	if prevLogIndex >= 1 {
		prevLogTerm = s.log[prevLogIndex-1].Term
	}

	request := appendEntriesArgs{
		Term:         s.currentTerm,
		LeaderID:     s.me,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		LeaderCommit: s.commitIndex,
	}
	if nextIndex <= len(s.log) {
		request.Entries = slices.Clone(s.log[nextIndex-1:])
	}
	return request
}

func (s *server) advanceCommitIndexLocked() bool {
	for index := len(s.log); index >= s.commitIndex+1; index-- {
		if s.log[index-1].Term != s.currentTerm {
			continue
		}
		count := 1 // The leader stores every entry in its own log.
		for _, matchIndex := range s.matchIndex {
			if matchIndex >= index {
				count++
			}
		}
		if majority(count, len(s.peers)) {
			s.commitIndex = index
			s.applyCommittedLocked()
			return true
		}
	}
	return false
}

func (s *server) applyCommittedLocked() {
	for s.lastApplied < s.commitIndex {
		s.lastApplied++
		s.db = append(s.db, s.log[s.lastApplied-1].Command)
	}
}

// candidateDoElection starts a new election immediately. Timer-driven
// elections use handleElectionTimeout, which first verifies the deadline.
func (s *server) candidateDoElection() {
	s.mu.Lock()
	electionTerm, args, peers := s.beginElectionLocked()
	s.mu.Unlock()

	s.notifyElectionTimer()
	s.sendRequestVotes(electionTerm, args, peers)
}

func (s *server) beginElectionLocked() (int, requestVoteArgs, []id) {
	s.state = candidate
	s.currentTerm++
	s.votedFor = s.me
	s.electionVotes = 1
	s.resetElectionDeadlineLocked()

	lastLogIndex, lastLogTerm := s.lastLogLocked()
	args := requestVoteArgs{
		Term:         s.currentTerm,
		CandidateID:  s.me,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}
	return s.currentTerm, args, append([]id(nil), s.peers...)
}

func (s *server) sendRequestVotes(electionTerm int, args requestVoteArgs, peers []id) {
	// handleRequestVoteReply accepts a reply only while this election is still
	// active, so a late reply cannot revive an older term.
	for _, peer := range peers {
		if peer == s.me {
			continue
		}
		s.wg.Add(1)
		go func(peer id) {
			defer s.wg.Done()
			reply, err := s.requestVote(context.Background(), peer, args)
			if err != nil {
				return
			}
			s.handleRequestVoteReply(electionTerm, reply)
		}(peer)
	}
}

func (s *server) handleRequestVoteReply(electionTerm int, reply requestVoteReply) {
	var notifyTimer, startReplication bool

	s.mu.Lock()
	if reply.Term > s.currentTerm {
		s.becomeFollowerLocked(reply.Term)
		s.resetElectionDeadlineLocked()
		notifyTimer = true
	} else if s.state == candidate && s.currentTerm == electionTerm && reply.VoteGranted {
		s.electionVotes++
		if majority(s.electionVotes, len(s.peers)) {
			s.becomeLeaderLocked()
			notifyTimer = true
			startReplication = true
		}
	}
	s.mu.Unlock()

	if notifyTimer {
		s.notifyElectionTimer()
	}
	if startReplication {
		s.triggerReplication()
	}
}

func (s *server) becomeLeaderLocked() {
	s.state = leader
	s.electionVotes = 0
	s.electionDeadline = time.Time{}
	lastLogIndex, _ := s.lastLogLocked()
	for _, peer := range s.peers {
		if peer == s.me {
			continue
		}
		s.nextIndex[peer] = lastLogIndex + 1
		s.matchIndex[peer] = 0
	}
}

// becomeFollowerLocked updates term-dependent state but deliberately leaves
// election timing to its caller. A granted vote or a valid AppendEntries resets
// the deadline; merely receiving RequestVote does not.
func (s *server) becomeFollowerLocked(term int) {
	if term > s.currentTerm {
		s.currentTerm = term
		s.votedFor = ""
	}
	s.state = follower
	s.electionVotes = 0
}

func (s *server) handleElectionTimeout() {
	s.mu.Lock()
	if s.state == leader || time.Now().Before(s.electionDeadline) {
		s.mu.Unlock()
		return
	}
	electionTerm, args, peers := s.beginElectionLocked()
	s.mu.Unlock()

	s.notifyElectionTimer()
	s.sendRequestVotes(electionTerm, args, peers)
}

// run is the sole owner of the election timer and heartbeat ticker.
func (s *server) run() {
	defer s.wg.Done()

	heartbeats := time.NewTicker(heartbeatInterval)
	defer heartbeats.Stop()
	electionTimer := time.NewTimer(s.electionDelay())
	defer electionTimer.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-s.electionResetCh:
			resetTimer(electionTimer, s.electionDelay())
		case <-electionTimer.C:
			s.handleElectionTimeout()
			resetTimer(electionTimer, s.electionDelay())
		case <-heartbeats.C:
			s.mu.RLock()
			isLeader := s.state == leader
			s.mu.RUnlock()
			if isLeader {
				s.triggerReplication()
			}
		}
	}
}

func (s *server) stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

// Start is called by client
func (s *server) Start(command string) (index int, term int, isLeader bool) {
	s.mu.Lock()
	if s.state != leader {
		s.mu.Unlock()
		return -1, -1, false
	}
	term = s.currentTerm
	s.log = append(s.log, logEntry{Term: term, Command: command})
	index = len(s.log)
	s.mu.Unlock()
	s.triggerReplication()
	return index, term, true
}

// RequestVote is called concurrently by the simulated network.
func (s *server) RequestVote(ctx context.Context, args requestVoteArgs) (requestVoteReply, error) {
	s.mu.Lock()
	if args.Term < s.currentTerm {
		reply := requestVoteReply{Term: s.currentTerm, VoteGranted: false}
		s.mu.Unlock()
		return reply, nil
	}
	notifyTimer := false
	if args.Term > s.currentTerm {
		s.becomeFollowerLocked(args.Term)
		notifyTimer = true
	}

	lastLogIndex, lastLogTerm := s.lastLogLocked()
	canVote := s.votedFor == "" || s.votedFor == args.CandidateID
	logIsUpToDate := (args.LastLogTerm > lastLogTerm) || (args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex)

	if canVote && logIsUpToDate {
		s.votedFor = args.CandidateID
		s.resetElectionDeadlineLocked()
		notifyTimer = true
		reply := requestVoteReply{Term: s.currentTerm, VoteGranted: true}
		s.mu.Unlock()
		if notifyTimer {
			s.notifyElectionTimer()
		}
		return reply, nil
	}

	reply := requestVoteReply{Term: s.currentTerm, VoteGranted: false}
	s.mu.Unlock()
	if notifyTimer {
		s.notifyElectionTimer()
	}
	return reply, nil
}

// AppendEntries handles both heartbeats and log replication.
func (s *server) AppendEntries(ctx context.Context, args appendEntriesArgs) (appendEntriesReply, error) {
	s.mu.Lock()

	if args.Term < s.currentTerm {
		reply := appendEntriesReply{Term: s.currentTerm, Success: false}
		s.mu.Unlock()
		return reply, nil
	}

	// Any non-stale AppendEntries is a valid leader contact for timing.
	s.becomeFollowerLocked(args.Term)
	s.resetElectionDeadlineLocked()
	defer func() {
		s.mu.Unlock()
		s.notifyElectionTimer()
	}()

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
		s.applyCommittedLocked()
	}

	return appendEntriesReply{Term: s.currentTerm, Success: true}, nil
}

// requestVote and appendEntries are the only outbound RPC calls the Raft core
// needs. ctx belongs to one RPC attempt, never to the server's lifetime.
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

// resetElectionDeadlineLocked changes Raft state only. run owns the actual
// time.Timer and is notified after the caller releases mu.
func (s *server) resetElectionDeadlineLocked() {
	s.electionDeadline = time.Now().Add(randomElectionTimeout())
}

func (s *server) notifyElectionTimer() {
	select {
	case s.electionResetCh <- struct{}{}:
	default:
	}
}

func (s *server) electionDelay() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.state == leader {
		return time.Hour
	}
	delay := time.Until(s.electionDeadline)
	if delay < 0 {
		return 0
	}
	return delay
}

// resetTimer is called only by run, the sole owner of the time.Timer.
func resetTimer(timer *time.Timer, delay time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(delay)
}

func randomElectionTimeout() time.Duration {
	return minElectionTimeout + time.Duration(rand.Int63n(int64(maxElectionTimeout-minElectionTimeout)))
}

func majority(count, total int) bool {
	return count >= total/2+1
}
