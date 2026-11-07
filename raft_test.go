package main

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func newUnitServer(t *testing.T, role state, term int, votedFor id, entries []logEntry) *server {
	t.Helper()

	timer := time.NewTimer(time.Hour)
	t.Cleanup(func() { timer.Stop() })
	return &server{
		me:          "S1",
		peers:       []id{"S1", "S2", "S3"},
		state:       role,
		currentTerm: term,
		votedFor:    votedFor,
		log:         append([]logEntry(nil), entries...),
		nextIndex:   make(map[id]int),
		matchIndex:  make(map[id]int),
		timer:       timer,
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
			name: "step down for leader in same term",
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
