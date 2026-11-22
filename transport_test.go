package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testRPCHandler struct {
	voteReply requestVoteReply
	voteCalls int
}

func (h *testRPCHandler) RequestVote(_ context.Context, _ requestVoteArgs) (requestVoteReply, error) {
	h.voteCalls++
	return h.voteReply, nil
}

func (h *testRPCHandler) AppendEntries(_ context.Context, _ appendEntriesArgs) (appendEntriesReply, error) {
	return appendEntriesReply{Success: true}, nil
}

func TestNetworkDeliversRequestAndReply(t *testing.T) {
	events := newEventHub()
	network := newNetwork(1, events)
	from := &testRPCHandler{}
	to := &testRPCHandler{voteReply: requestVoteReply{Term: 3, VoteGranted: true}}
	if err := network.register("S1", from); err != nil {
		t.Fatal(err)
	}
	if err := network.register("S2", to); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reply, err := network.requestVote(ctx, "S1", "S2", requestVoteArgs{Term: 3})
	if err != nil {
		t.Fatalf("request vote: %v", err)
	}
	if !reply.VoteGranted || reply.Term != 3 {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	if to.voteCalls != 1 {
		t.Fatalf("RequestVote calls = %d, want 1", to.voteCalls)
	}
}

func TestNetworkTreatsReplyAsDirectionalTraffic(t *testing.T) {
	events := newEventHub()
	network := newNetwork(1, events)
	from := &testRPCHandler{}
	to := &testRPCHandler{voteReply: requestVoteReply{VoteGranted: true}}
	if err := network.register("S1", from); err != nil {
		t.Fatal(err)
	}
	if err := network.register("S2", to); err != nil {
		t.Fatal(err)
	}
	if err := network.setLink("S2", "S1", false); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := network.requestVote(ctx, "S1", "S2", requestVoteArgs{Term: 1})
	if !errors.Is(err, errUnreachable) {
		t.Fatalf("request vote error = %v, want %v", err, errUnreachable)
	}
	if to.voteCalls != 1 {
		t.Fatalf("handler should receive the request before its reply is lost; calls = %d", to.voteCalls)
	}
}

func TestNetworkRejectsTrafficFromIsolatedNode(t *testing.T) {
	events := newEventHub()
	network := newNetwork(1, events)
	if err := network.register("S1", &testRPCHandler{}); err != nil {
		t.Fatal(err)
	}
	if err := network.register("S2", &testRPCHandler{}); err != nil {
		t.Fatal(err)
	}
	if err := network.setNode("S1", false); err != nil {
		t.Fatal(err)
	}

	_, err := network.appendEntries(context.Background(), "S1", "S2", appendEntriesArgs{})
	if !errors.Is(err, errUnreachable) {
		t.Fatalf("append entries error = %v, want %v", err, errUnreachable)
	}
}

func TestNetworkMarksEmptyAppendEntriesAsHeartbeat(t *testing.T) {
	events := newEventHub()
	network := newNetwork(1, events)
	if err := network.register("S1", &testRPCHandler{}); err != nil {
		t.Fatal(err)
	}
	if err := network.register("S2", &testRPCHandler{}); err != nil {
		t.Fatal(err)
	}

	stream, unsubscribe := events.subscribe()
	defer unsubscribe()
	args := appendEntriesArgs{Term: 4, LeaderID: "S1", PrevLogIndex: 2, PrevLogTerm: 3, LeaderCommit: 1}
	errCh := make(chan error, 1)
	go func() {
		_, err := network.appendEntries(context.Background(), "S1", "S2", args)
		errCh <- err
	}()

	select {
	case event := <-stream:
		if event.Stage != "sent" || !event.Heartbeat {
			t.Fatalf("first event = %+v, want sent heartbeat", event)
		}
		eventArgs, ok := event.Args.(appendEntriesArgs)
		if !ok || eventArgs.Term != args.Term || eventArgs.LeaderID != args.LeaderID ||
			eventArgs.PrevLogIndex != args.PrevLogIndex || eventArgs.PrevLogTerm != args.PrevLogTerm ||
			eventArgs.LeaderCommit != args.LeaderCommit || len(eventArgs.Entries) != 0 {
			t.Fatalf("event args = %#v, want %#v", event.Args, args)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for heartbeat event")
	}
	if err := <-errCh; err != nil {
		t.Fatalf("append entries: %v", err)
	}
}

func TestEventHubReplaysNotableEvents(t *testing.T) {
	events := newEventHub()
	events.publish(rpcEvent{Method: "AppendEntries", Stage: "sent", Heartbeat: true})
	events.publish(rpcEvent{From: "S1", To: "S2", Method: "RequestVote", Stage: "sent", Term: 3})

	if len(events.history) != 1 {
		t.Fatalf("history length = %d, want 1", len(events.history))
	}

	stream, unsubscribe := events.subscribe()
	defer unsubscribe()
	select {
	case event := <-stream:
		if !event.Replay || event.From != "S1" || event.To != "S2" || event.Method != "RequestVote" || event.Stage != "sent" {
			t.Fatalf("replayed event = %+v, want RequestVote sent from S1 to S2", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for replayed event")
	}
}

func TestNetworkDoesNotDeliverRPCStartedBeforeClose(t *testing.T) {
	events := newEventHub()
	network := newNetwork(1, events)
	from := &testRPCHandler{}
	to := &testRPCHandler{voteReply: requestVoteReply{VoteGranted: true}}
	if err := network.register("S1", from); err != nil {
		t.Fatal(err)
	}
	if err := network.register("S2", to); err != nil {
		t.Fatal(err)
	}

	stream, unsubscribe := events.subscribe()
	defer unsubscribe()

	errCh := make(chan error, 1)
	go func() {
		_, err := network.requestVote(context.Background(), "S1", "S2", requestVoteArgs{Term: 1})
		errCh <- err
	}()

	select {
	case event := <-stream:
		if event.Stage != "sent" {
			t.Fatalf("first event stage = %q, want sent", event.Stage)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sent event")
	}
	network.close()

	select {
	case err := <-errCh:
		if !errors.Is(err, errNetworkClosed) {
			t.Fatalf("request vote error = %v, want %v", err, errNetworkClosed)
		}
	case <-time.After(time.Second):
		t.Fatal("request vote did not return after network close")
	}
	if to.voteCalls != 0 {
		t.Fatalf("RequestVote calls = %d, want 0 after network close", to.voteCalls)
	}
}
