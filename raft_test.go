package main

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func newUnitServer(t *testing.T, role state, term int, votedFor id, entries []logEntry) *server {
	t.Helper()

	return &server{
		me:               "S1",
		peers:            []id{"S1", "S2", "S3"},
		state:            role,
		currentTerm:      term,
		votedFor:         votedFor,
		log:              append([]logEntry(nil), entries...),
		nextIndex:        make(map[id]int),
		matchIndex:       make(map[id]int),
		electionDeadline: time.Now().Add(time.Hour),
		electionResetCh:  make(chan struct{}, 1),
		replicateCh:      make(map[id]chan struct{}),
		stopCh:           make(chan struct{}),
	}
}

func TestRequestVote(t *testing.T) {
	tests := []struct {
		name         string
		role         state
		term         int
		votedFor     id
		log          []logEntry
		args         requestVoteArgs
		wantReply    requestVoteReply
		wantRole     state
		wantTerm     int
		wantVotedFor id
	}{
		{
			name:      "reject stale term",
			role:      follower,
			term:      3,
			args:      requestVoteArgs{Term: 2, CandidateID: "S2", LastLogTerm: -1},
			wantReply: requestVoteReply{Term: 3, VoteGranted: false},
			wantRole:  follower,
			wantTerm:  3,
		},
		{
			name:         "grant vote in newer term",
			role:         candidate,
			term:         2,
			votedFor:     "S3",
			args:         requestVoteArgs{Term: 3, CandidateID: "S2", LastLogTerm: -1},
			wantReply:    requestVoteReply{Term: 3, VoteGranted: true},
			wantRole:     follower,
			wantTerm:     3,
			wantVotedFor: "S2",
		},
		{
			name:         "grant repeated request from same candidate",
			role:         follower,
			term:         3,
			votedFor:     "S2",
			args:         requestVoteArgs{Term: 3, CandidateID: "S2", LastLogTerm: -1},
			wantReply:    requestVoteReply{Term: 3, VoteGranted: true},
			wantRole:     follower,
			wantTerm:     3,
			wantVotedFor: "S2",
		},
		{
			name:         "reject another candidate in same term",
			role:         follower,
			term:         3,
			votedFor:     "S3",
			args:         requestVoteArgs{Term: 3, CandidateID: "S2", LastLogTerm: -1},
			wantReply:    requestVoteReply{Term: 3, VoteGranted: false},
			wantRole:     follower,
			wantTerm:     3,
			wantVotedFor: "S3",
		},
		{
			name: "reject candidate with older log",
			role: follower,
			term: 3,
			log: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "y=2", Term: 3},
			},
			args: requestVoteArgs{
				Term:         3,
				CandidateID:  "S2",
				LastLogIndex: 10,
				LastLogTerm:  2,
			},
			wantReply: requestVoteReply{Term: 3, VoteGranted: false},
			wantRole:  follower,
			wantTerm:  3,
		},
		{
			name: "reject shorter log with same last term",
			role: follower,
			term: 3,
			log: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "y=2", Term: 3},
			},
			args: requestVoteArgs{
				Term:         3,
				CandidateID:  "S2",
				LastLogIndex: 1,
				LastLogTerm:  3,
			},
			wantReply: requestVoteReply{Term: 3, VoteGranted: false},
			wantRole:  follower,
			wantTerm:  3,
		},
		{
			name: "grant vote for equally up-to-date log",
			role: follower,
			term: 3,
			log: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "y=2", Term: 3},
			},
			args: requestVoteArgs{
				Term:         3,
				CandidateID:  "S2",
				LastLogIndex: 2,
				LastLogTerm:  3,
			},
			wantReply:    requestVoteReply{Term: 3, VoteGranted: true},
			wantRole:     follower,
			wantTerm:     3,
			wantVotedFor: "S2",
		},
		{
			name: "grant vote for higher last log term despite shorter log",
			role: follower,
			term: 3,
			log: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "y=2", Term: 2},
				{Command: "z=3", Term: 2},
			},
			args: requestVoteArgs{
				Term:         3,
				CandidateID:  "S2",
				LastLogIndex: 1,
				LastLogTerm:  3,
			},
			wantReply:    requestVoteReply{Term: 3, VoteGranted: true},
			wantRole:     follower,
			wantTerm:     3,
			wantVotedFor: "S2",
		},
		{
			name:     "adopt newer term before rejecting stale log",
			role:     candidate,
			term:     3,
			votedFor: "S3",
			log: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "y=2", Term: 3},
			},
			args: requestVoteArgs{
				Term:         4,
				CandidateID:  "S2",
				LastLogIndex: 10,
				LastLogTerm:  2,
			},
			wantReply: requestVoteReply{Term: 4, VoteGranted: false},
			wantRole:  follower,
			wantTerm:  4,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newUnitServer(t, test.role, test.term, test.votedFor, test.log)
			reply, err := s.RequestVote(context.Background(), test.args)
			if err != nil {
				t.Fatalf("RequestVote returned error: %v", err)
			}
			if reply != test.wantReply {
				t.Errorf("reply = %+v, want %+v", reply, test.wantReply)
			}
			if s.state != test.wantRole {
				t.Errorf("state = %s, want %s", s.state, test.wantRole)
			}
			if s.currentTerm != test.wantTerm {
				t.Errorf("currentTerm = %d, want %d", s.currentTerm, test.wantTerm)
			}
			if s.votedFor != test.wantVotedFor {
				t.Errorf("votedFor = %q, want %q", s.votedFor, test.wantVotedFor)
			}
			if !reflect.DeepEqual(s.log, test.log) {
				t.Errorf("log changed to %+v, want %+v", s.log, test.log)
			}
		})
	}
}

func TestAppendEntries(t *testing.T) {
	tests := []struct {
		name         string
		role         state
		term         int
		votedFor     id
		log          []logEntry
		commitIndex  int
		args         appendEntriesArgs
		wantReply    appendEntriesReply
		wantRole     state
		wantTerm     int
		wantVotedFor id
		wantLog      []logEntry
		wantCommit   int
	}{
		{
			name:      "reject stale term",
			role:      follower,
			term:      3,
			log:       []logEntry{{Command: "x=1", Term: 1}},
			args:      appendEntriesArgs{Term: 2, LeaderID: "S2"},
			wantReply: appendEntriesReply{Term: 3, Success: false},
			wantRole:  follower,
			wantTerm:  3,
			wantLog:   []logEntry{{Command: "x=1", Term: 1}},
		},
		{
			name: "accept heartbeat with matching previous entry",
			role: candidate,
			term: 1,
			log:  []logEntry{{Command: "x=1", Term: 1}},
			args: appendEntriesArgs{
				Term:         2,
				LeaderID:     "S2",
				PrevLogIndex: 1,
				PrevLogTerm:  1,
			},
			wantReply: appendEntriesReply{Term: 2, Success: true},
			wantRole:  follower,
			wantTerm:  2,
			wantLog:   []logEntry{{Command: "x=1", Term: 1}},
		},
		{
			name: "append and commit new entry",
			role: follower,
			term: 2,
			log:  []logEntry{{Command: "x=1", Term: 1}},
			args: appendEntriesArgs{
				Term:         2,
				LeaderID:     "S2",
				PrevLogIndex: 1,
				PrevLogTerm:  1,
				Entries:      []logEntry{{Command: "y=2", Term: 2}},
				LeaderCommit: 2,
			},
			wantReply:  appendEntriesReply{Term: 2, Success: true},
			wantRole:   follower,
			wantTerm:   2,
			wantLog:    []logEntry{{Command: "x=1", Term: 1}, {Command: "y=2", Term: 2}},
			wantCommit: 2,
		},
		{
			name: "replace conflicting suffix",
			role: follower,
			term: 3,
			log: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "old=1", Term: 2},
				{Command: "old=2", Term: 2},
			},
			args: appendEntriesArgs{
				Term:         3,
				LeaderID:     "S2",
				PrevLogIndex: 1,
				PrevLogTerm:  1,
				Entries:      []logEntry{{Command: "new=1", Term: 3}},
			},
			wantReply: appendEntriesReply{Term: 3, Success: true},
			wantRole:  follower,
			wantTerm:  3,
			wantLog: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "new=1", Term: 3},
			},
		},
		{
			name:      "reject missing previous entry",
			role:      follower,
			term:      3,
			log:       []logEntry{{Command: "x=1", Term: 1}},
			args:      appendEntriesArgs{Term: 3, LeaderID: "S2", PrevLogIndex: 2, PrevLogTerm: 1},
			wantReply: appendEntriesReply{Term: 3, Success: false},
			wantRole:  follower,
			wantTerm:  3,
			wantLog:   []logEntry{{Command: "x=1", Term: 1}},
		},
		{
			name:      "accept heartbeat for empty log",
			role:      follower,
			term:      1,
			args:      appendEntriesArgs{Term: 1, LeaderID: "S2", PrevLogIndex: 0, PrevLogTerm: -1},
			wantReply: appendEntriesReply{Term: 1, Success: true},
			wantRole:  follower,
			wantTerm:  1,
			wantLog:   nil,
		},
		{
			name: "reject previous term mismatch",
			role: follower,
			term: 3,
			log: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "y=2", Term: 2},
			},
			commitIndex: 1,
			args: appendEntriesArgs{
				Term:         3,
				LeaderID:     "S2",
				PrevLogIndex: 2,
				PrevLogTerm:  3,
			},
			wantReply:  appendEntriesReply{Term: 3, Success: false},
			wantRole:   follower,
			wantTerm:   3,
			wantLog:    []logEntry{{Command: "x=1", Term: 1}, {Command: "y=2", Term: 2}},
			wantCommit: 1,
		},
		{
			name: "step down to follower for leader in same term",
			role: candidate,
			term: 3,
			log:  []logEntry{{Command: "x=1", Term: 1}},
			args: appendEntriesArgs{
				Term:         3,
				LeaderID:     "S2",
				PrevLogIndex: 1,
				PrevLogTerm:  1,
			},
			wantReply: appendEntriesReply{Term: 3, Success: true},
			wantRole:  follower,
			wantTerm:  3,
			wantLog:   []logEntry{{Command: "x=1", Term: 1}},
		},
		{
			name:     "adopt newer term before rejecting missing previous entry",
			role:     candidate,
			term:     2,
			votedFor: "S3",
			log:      []logEntry{{Command: "x=1", Term: 1}},
			args: appendEntriesArgs{
				Term:         3,
				LeaderID:     "S2",
				PrevLogIndex: 2,
				PrevLogTerm:  1,
			},
			wantReply: appendEntriesReply{Term: 3, Success: false},
			wantRole:  follower,
			wantTerm:  3,
			wantLog:   []logEntry{{Command: "x=1", Term: 1}},
		},
		{
			name: "append only entries after matching overlap",
			role: follower,
			term: 3,
			log: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "y=2", Term: 2},
				{Command: "old=3", Term: 2},
			},
			args: appendEntriesArgs{
				Term:         3,
				LeaderID:     "S2",
				PrevLogIndex: 1,
				PrevLogTerm:  1,
				Entries: []logEntry{
					{Command: "y=2", Term: 2},
					{Command: "new=3", Term: 3},
					{Command: "new=4", Term: 3},
				},
			},
			wantReply: appendEntriesReply{Term: 3, Success: true},
			wantRole:  follower,
			wantTerm:  3,
			wantLog: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "y=2", Term: 2},
				{Command: "new=3", Term: 3},
				{Command: "new=4", Term: 3},
			},
		},
		{
			name: "clamp leader commit to last matching entry",
			role: follower,
			term: 3,
			log: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "y=2", Term: 2},
				{Command: "z=3", Term: 3},
			},
			commitIndex: 1,
			args: appendEntriesArgs{
				Term:         3,
				LeaderID:     "S2",
				PrevLogIndex: 3,
				PrevLogTerm:  3,
				LeaderCommit: 10,
			},
			wantReply:  appendEntriesReply{Term: 3, Success: true},
			wantRole:   follower,
			wantTerm:   3,
			wantLog:    []logEntry{{Command: "x=1", Term: 1}, {Command: "y=2", Term: 2}, {Command: "z=3", Term: 3}},
			wantCommit: 3,
		},
		{
			name: "do not decrease commit index",
			role: follower,
			term: 3,
			log: []logEntry{
				{Command: "x=1", Term: 1},
				{Command: "y=2", Term: 2},
			},
			commitIndex: 2,
			args: appendEntriesArgs{
				Term:         3,
				LeaderID:     "S2",
				PrevLogIndex: 2,
				PrevLogTerm:  2,
				LeaderCommit: 1,
			},
			wantReply:  appendEntriesReply{Term: 3, Success: true},
			wantRole:   follower,
			wantTerm:   3,
			wantLog:    []logEntry{{Command: "x=1", Term: 1}, {Command: "y=2", Term: 2}},
			wantCommit: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newUnitServer(t, test.role, test.term, test.votedFor, test.log)
			s.commitIndex = test.commitIndex

			reply, err := callAppendEntries(t, s, test.args)
			if err != nil {
				t.Fatalf("AppendEntries returned error: %v", err)
			}
			if reply != test.wantReply {
				t.Errorf("reply = %+v, want %+v", reply, test.wantReply)
			}
			if s.state != test.wantRole {
				t.Errorf("state = %s, want %s", s.state, test.wantRole)
			}
			if s.currentTerm != test.wantTerm {
				t.Errorf("currentTerm = %d, want %d", s.currentTerm, test.wantTerm)
			}
			if s.votedFor != test.wantVotedFor {
				t.Errorf("votedFor = %q, want %q", s.votedFor, test.wantVotedFor)
			}
			if !reflect.DeepEqual(s.log, test.wantLog) {
				t.Errorf("log = %+v, want %+v", s.log, test.wantLog)
			}
			if s.commitIndex != test.wantCommit {
				t.Errorf("commitIndex = %d, want %d", s.commitIndex, test.wantCommit)
			}
		})
	}
}

func callAppendEntries(t *testing.T, s *server, args appendEntriesArgs) (appendEntriesReply, error) {
	t.Helper()

	type result struct {
		reply      appendEntriesReply
		err        error
		panicValue any
	}
	done := make(chan result, 1)
	go func() {
		call := result{}
		defer func() {
			call.panicValue = recover()
			done <- call
		}()
		call.reply, call.err = s.AppendEntries(context.Background(), args)
	}()

	select {
	case call := <-done:
		if call.panicValue != nil {
			t.Fatalf("AppendEntries panicked: %v", call.panicValue)
		}
		return call.reply, call.err
	case <-time.After(500 * time.Millisecond):
		t.Fatal("AppendEntries blocked; check for nested or unreleased mutex locks")
		return appendEntriesReply{}, nil
	}
}

func TestAppendEntriesAppliesCommittedEntries(t *testing.T) {
	s := newUnitServer(t, follower, 3, "", []logEntry{
		{Command: "x=1", Term: 1},
		{Command: "y=2", Term: 3},
	})

	_, err := s.AppendEntries(context.Background(), appendEntriesArgs{
		Term:         3,
		LeaderID:     "S2",
		PrevLogIndex: 2,
		PrevLogTerm:  3,
		LeaderCommit: 2,
	})
	if err != nil {
		t.Fatalf("AppendEntries returned error: %v", err)
	}

	if s.commitIndex != 2 {
		t.Errorf("commitIndex = %d, want 2", s.commitIndex)
	}
	if s.lastApplied != 2 {
		t.Errorf("lastApplied = %d, want 2", s.lastApplied)
	}
	wantDB := []string{"x=1", "y=2"}
	if !reflect.DeepEqual(s.db, wantDB) {
		t.Errorf("db = %v, want %v", s.db, wantDB)
	}
}

func TestAppendEntriesDoesNotApplyUncommittedEntry(t *testing.T) {
	s := newUnitServer(t, follower, 3, "", nil)

	_, err := s.AppendEntries(context.Background(), appendEntriesArgs{
		Term:     3,
		LeaderID: "S2",
		Entries:  []logEntry{{Command: "x=1", Term: 3}},
	})
	if err != nil {
		t.Fatalf("AppendEntries returned error: %v", err)
	}

	if s.commitIndex != 0 || s.lastApplied != 0 {
		t.Errorf("commitIndex, lastApplied = %d, %d; want 0, 0", s.commitIndex, s.lastApplied)
	}
	if len(s.db) != 0 {
		t.Errorf("db = %v, want no applied commands", s.db)
	}
}

func TestAppendEntriesPreventsStaleTimeoutElection(t *testing.T) {
	s := newUnitServer(t, candidate, 3, "S1", nil)
	s.electionDeadline = time.Now().Add(-time.Second)

	_, err := s.AppendEntries(context.Background(), appendEntriesArgs{
		Term:     3,
		LeaderID: "S2",
	})
	if err != nil {
		t.Fatalf("AppendEntries returned error: %v", err)
	}

	// Simulate run observing an old timer event immediately afterwards. The
	// updated deadline must make that event harmless.
	s.handleElectionTimeout()

	if s.state != follower {
		t.Errorf("state = %s, want follower", s.state)
	}
	if s.currentTerm != 3 {
		t.Errorf("currentTerm = %d, want 3", s.currentTerm)
	}
}

func TestStart(t *testing.T) {
	t.Run("leader appends but does not apply before commit", func(t *testing.T) {
		s := newUnitServer(t, leader, 4, "S1", []logEntry{{Command: "x=1", Term: 3}})
		s.peers = []id{"S1"}

		index, term, isLeader := s.Start("y=2")

		if !isLeader {
			t.Fatal("Start reported isLeader = false, want true")
		}
		if index != 2 || term != 4 {
			t.Errorf("Start() = (%d, %d, true), want (2, 4, true)", index, term)
		}
		wantLog := []logEntry{{Command: "x=1", Term: 3}, {Command: "y=2", Term: 4}}
		if !reflect.DeepEqual(s.log, wantLog) {
			t.Errorf("log = %+v, want %+v", s.log, wantLog)
		}
		if s.commitIndex != 0 || s.lastApplied != 0 || len(s.db) != 0 {
			t.Errorf("uncommitted command changed state machine: commitIndex=%d lastApplied=%d db=%v", s.commitIndex, s.lastApplied, s.db)
		}
	})

	t.Run("non-leader rejects command without changing log", func(t *testing.T) {
		s := newUnitServer(t, follower, 4, "", []logEntry{{Command: "x=1", Term: 3}})

		index, term, isLeader := s.Start("y=2")

		if isLeader || index != -1 || term != -1 {
			t.Errorf("Start() = (%d, %d, %t), want (-1, -1, false)", index, term, isLeader)
		}
		wantLog := []logEntry{{Command: "x=1", Term: 3}}
		if !reflect.DeepEqual(s.log, wantLog) {
			t.Errorf("log = %+v, want %+v", s.log, wantLog)
		}
	})
}

func TestCandidateElectionBecomesLeaderAndInitializesReplicationState(t *testing.T) {
	leaderServer, followers := newRaftTestCluster(t, 0, nil)

	leaderServer.candidateDoElection()

	waitForCondition(t, "candidate to become leader", func() bool {
		leaderServer.mu.RLock()
		defer leaderServer.mu.RUnlock()
		return leaderServer.state == leader
	})

	leaderServer.mu.RLock()
	if leaderServer.currentTerm != 1 {
		t.Errorf("currentTerm = %d, want 1", leaderServer.currentTerm)
	}
	if leaderServer.votedFor != leaderServer.me {
		t.Errorf("votedFor = %q, want %q", leaderServer.votedFor, leaderServer.me)
	}
	for _, follower := range followers {
		if leaderServer.nextIndex[follower.me] != 1 {
			t.Errorf("nextIndex[%s] = %d, want 1", follower.me, leaderServer.nextIndex[follower.me])
		}
		if leaderServer.matchIndex[follower.me] != 0 {
			t.Errorf("matchIndex[%s] = %d, want 0", follower.me, leaderServer.matchIndex[follower.me])
		}
	}
	leaderServer.mu.RUnlock()
}

func TestReplicationCommitsAppliesAndPropagatesCommit(t *testing.T) {
	entries := []logEntry{
		{Command: "old=1", Term: 2},
		{Command: "x=1", Term: 3},
	}
	leaderServer, followers := newRaftTestCluster(t, 3, entries)

	leaderServer.triggerReplication()

	waitForCondition(t, "leader to commit and apply entry", func() bool {
		leaderServer.mu.RLock()
		defer leaderServer.mu.RUnlock()
		return leaderServer.commitIndex == 2 && leaderServer.lastApplied == 2
	})

	leaderServer.mu.RLock()
	if !reflect.DeepEqual(leaderServer.db, []string{"old=1", "x=1"}) {
		t.Errorf("leader db = %v, want [old=1 x=1]", leaderServer.db)
	}
	leaderServer.mu.RUnlock()

	// A commit advancement wakes workers again so they carry the new
	// LeaderCommit. Triggering once more is harmless and keeps this phase of the
	// test explicit.
	leaderServer.triggerReplication()

	waitForCondition(t, "followers to apply committed entry", func() bool {
		for _, follower := range followers {
			follower.mu.RLock()
			ready := follower.commitIndex == 2 && follower.lastApplied == 2
			follower.mu.RUnlock()
			if !ready {
				return false
			}
		}
		return true
	})

	for _, follower := range followers {
		follower.mu.RLock()
		if !reflect.DeepEqual(follower.log, entries) {
			t.Errorf("%s log = %+v, want %+v", follower.me, follower.log, entries)
		}
		if !reflect.DeepEqual(follower.db, []string{"old=1", "x=1"}) {
			t.Errorf("%s db = %v, want [old=1 x=1]", follower.me, follower.db)
		}
		follower.mu.RUnlock()
	}
}

func TestLeaderDoesNotDirectlyCommitPriorTermEntry(t *testing.T) {
	leaderServer, followers := newRaftTestCluster(t, 3, []logEntry{{Command: "x=1", Term: 2}})

	leaderServer.triggerReplication()

	waitForCondition(t, "followers to replicate prior-term entry", func() bool {
		for _, follower := range followers {
			follower.mu.RLock()
			matched := len(follower.log) == 1
			follower.mu.RUnlock()
			if !matched {
				return false
			}
		}
		return true
	})

	leaderServer.mu.RLock()
	defer leaderServer.mu.RUnlock()
	if leaderServer.commitIndex != 0 {
		t.Errorf("commitIndex = %d, want 0: prior-term entries cannot be directly committed", leaderServer.commitIndex)
	}
	if leaderServer.lastApplied != 0 || len(leaderServer.db) != 0 {
		t.Errorf("prior-term entry was applied: lastApplied=%d db=%v", leaderServer.lastApplied, leaderServer.db)
	}
}

func TestStopStopsServerWorkers(t *testing.T) {
	events := newEventHub()
	network := newNetwork(1, events)
	s := newServer("S1", []id{"S1", "S2"}, network, 0, "", nil)
	t.Cleanup(func() {
		network.close()
		events.close()
	})

	stopped := make(chan struct{})
	go func() {
		s.stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop did not wait for run and replication workers to exit")
	}
	select {
	case <-s.stopCh:
	default:
		t.Fatal("stop did not close the server lifecycle channel")
	}
}

// newRaftTestCluster creates stopped servers so tests can exercise Raft logic
// through the real, reliable in-memory transport without election timers.
func newRaftTestCluster(t *testing.T, term int, entries []logEntry) (*server, []*server) {
	t.Helper()

	ids := []id{"S1", "S2", "S3"}
	events := newEventHub()
	network := newNetwork(1, events)
	t.Cleanup(func() {
		network.close()
		events.close()
	})

	leaderServer := newStoppedTestServer(t, "S1", ids, leader, term, "", entries, network)
	followers := []*server{
		newStoppedTestServer(t, "S2", ids, follower, term, "", nil, network),
		newStoppedTestServer(t, "S3", ids, follower, term, "", nil, network),
	}
	for _, server := range append([]*server{leaderServer}, followers...) {
		if err := network.register(server.me, server); err != nil {
			t.Fatalf("register %s: %v", server.me, err)
		}
	}

	for _, follower := range followers {
		leaderServer.nextIndex[follower.me] = len(entries) + 1
		leaderServer.matchIndex[follower.me] = 0
	}
	return leaderServer, followers
}

func newStoppedTestServer(t *testing.T, me id, peers []id, role state, term int, votedFor id, entries []logEntry, transport *network) *server {
	t.Helper()

	s := &server{
		me:               me,
		peers:            append([]id(nil), peers...),
		state:            role,
		transport:        transport,
		currentTerm:      term,
		votedFor:         votedFor,
		log:              append([]logEntry(nil), entries...),
		nextIndex:        make(map[id]int),
		matchIndex:       make(map[id]int),
		electionDeadline: time.Now().Add(time.Hour),
		electionResetCh:  make(chan struct{}, 1),
		replicateCh:      make(map[id]chan struct{}),
		stopCh:           make(chan struct{}),
	}
	for _, peer := range peers {
		if peer == me {
			continue
		}
		work := make(chan struct{}, 1)
		s.replicateCh[peer] = work
		s.wg.Add(1)
		go s.replicationWorker(peer, work)
	}
	t.Cleanup(func() {
		close(s.stopCh)
		s.wg.Wait()
	})
	return s
}

func waitForCondition(t *testing.T, description string, condition func() bool) {
	t.Helper()

	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for %s", description)
		case <-ticker.C:
		}
	}
}
