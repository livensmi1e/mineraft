# Concurrency Lessons From the First Raft Run Loop

- Snapshot: commit `edf0872d2eeec0b458c36f9592f18f9ce0da0102`
- Scope: synchronization and goroutine coordination
- Out of scope: whether the Raft algorithm itself is correct
- Snippets: simplified excerpts that preserve the relevant execution flow

## Holding the Raft lock across network calls

```go
func (s *server) doElection() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.doCandidate()
}

func (s *server) doCandidate() {
	s.currentTerm++
	s.state = candidate
	s.votedFor = s.me

	var wg sync.WaitGroup
	for _, peer := range s.peers {
		wg.Go(func() {
			_, _ = s.requestVote(context.Background(), peer, requestVoteArgs{
				Term:        s.currentTerm,
				CandidateID: s.me,
			})
		})
	}

	// doElection still owns s.mu while waiting for network calls.
	wg.Wait()
}
```

- **Problem:** `s.mu` remains locked during RPC calls and `WaitGroup.Wait`.
- **Effect:** incoming `AppendEntries` and `RequestVote` handlers cannot update
  the server while the election is running.
- **Lesson:** use the lock only to update or snapshot state. Release it before
  network calls, then lock again when processing a result.

## Creating a distributed deadlock

```text
Server A                      Server B
--------                      --------
doElection                    doElection
  Lock(A.mu)                    Lock(B.mu)
  send RequestVote ──────────>  RequestVote
                                  waits for B.mu

  RequestVote  <──────────────  send RequestVote
    waits for A.mu               wait for reply from A

  wait for reply from B

A owns A.mu and waits for B.
B owns B.mu and waits for A.
Neither RequestVote handler can acquire its local server mutex.
```

- **Problem:** each candidate owns its local mutex while calling the other.
- **Effect:** both RPC handlers wait for locks held by callers waiting for those
  same handlers.
- **Lesson:** never hold the Raft mutex while calling another node or waiting for
  work that may enter an RPC handler.

## Preventing an incoming leader from taking effect

```go
func (s *server) doElection() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// This does not return until all vote RPCs return.
	s.doCandidate()
}

func (s *server) AppendEntries(
	ctx context.Context,
	args appendEntriesArgs,
) (appendEntriesReply, error) {
	// Blocked here while doElection is waiting for votes.
	s.mu.Lock()
	defer s.mu.Unlock()

	if args.Term < s.currentTerm {
		return appendEntriesReply{Term: s.currentTerm}, nil
	}

	if s.state == candidate {
		s.state = follower
	}

	// The transition is correct in intention, but may run too late.
	return appendEntriesReply{Term: s.currentTerm, Success: true}, nil
}
```

- **Problem:** the transition is implemented, but `AppendEntries` cannot acquire
  `s.mu` while the election owns it.
- **Effect:** a valid leader cannot make the candidate step down promptly.
- **Lesson:** a candidate must remain able to receive RPCs while vote requests
  are in flight.

## Writing to a map from multiple goroutines

```go
replies := make(map[id]requestVoteReply)
var wg sync.WaitGroup

for _, peer := range s.peers {
	if peer == s.me {
		continue
	}

	wg.Go(func() {
		reply, _ := s.requestVote(context.Background(), peer, args)

		// Every RPC goroutine writes to the same map.
		replies[peer] = reply
	})
}

// Waiting here does not make the writes above sequential.
wg.Wait()

for peer, reply := range replies {
	_ = peer
	_ = reply
}
```

- **Problem:** multiple goroutines write to the same map.
- **Important:** `WaitGroup` waits; it does not serialize map access.
- **Effect:** data race or `fatal error: concurrent map writes`.
- **Lesson:** send RPC results through a channel and let one goroutine consume
  them. A map mutex fixes memory safety but not stale-result handling.

## Processing replies only after every RPC completes

```go
for _, peer := range s.peers {
	wg.Go(func() {
		reply, _ := s.requestVote(context.Background(), peer, args)
		replies[peer] = reply
	})
}

// No result is inspected before the slowest RPC returns.
wg.Wait()

votes := 1
for _, reply := range replies {
	if reply.Term > s.currentTerm {
		s.currentTerm = reply.Term
		s.state = follower
		return
	}
	if reply.VoteGranted {
		votes++
	}
}

if majority(votes, len(s.peers)) {
	s.doLeader()
}
```

- **Problem:** all replies are collected before any reply is interpreted.
- **Effect:** an available majority is delayed; a higher term is also ignored
  until the slowest RPC completes.
- **Lesson:** process replies as they arrive. A slow peer must not block a
  decision that can already be made.

## Applying stale election replies

```go
func (s *server) doCandidate() {
	s.currentTerm++
	s.state = candidate
	s.votedFor = s.me

	electionTerm := s.currentTerm

	// Requests are now in flight for electionTerm.
	sendRequestVotes(electionTerm)

	// While they are in flight, AppendEntries may change both fields:
	// s.state = follower
	// s.currentTerm = electionTerm + 1

	reply := receiveVoteReply()

	// Incorrect: the reply is counted without checking whether this
	// election is still the active one.
	if reply.VoteGranted {
		votes++
	}

	if majority(votes, len(s.peers)) {
		s.doLeader()
	}
}
```

- **Problem:** a reply is used without proving that its election is still
  active.
- **Possible interleaving:** send votes in term 5, step down in term 6, then
  receive a granted vote from term 5.
- **Required check:** still `candidate` and `currentTerm == electionTerm`.
- **Exception:** always process a reply containing a higher term.
- **Lesson:** every asynchronous result needs a generation; in Raft, the term is
  usually that generation.

## Reading shared state without the mutex

```go
func (s *server) run() {
	for {
		select {
		case <-s.stopCh:
			return
		case <-s.timer.C:
			s.handleTimeout()
		}
	}
}

func (s *server) handleTimeout() {
	// This read is not protected by s.mu.
	if s.state == leader {
		return
	}

	// s.state may also change between the check and doElection.
	s.doElection()
}

func (s *server) AppendEntries(/* ... */) (appendEntriesReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// A different goroutine writes the same field under the mutex.
	s.state = follower
	return appendEntriesReply{}, nil
}
```

- **Problem:** RPC handlers write `s.state` under `s.mu`, but `handleTimeout`
  reads it without the mutex.
- **Effect:** Go data race and a check-then-act race.
- **Lesson:** protect reads and writes with the same mutex. Validate the state in
  the same critical section that performs the transition.

## Reading live state inside outbound RPC goroutines

```go
func (s *server) doCandidate() {
	s.currentTerm++
	s.state = candidate
	s.votedFor = s.me

	for _, peer := range s.peers {
		wg.Go(func() {
			// These values are read later, inside another goroutine.
			lastLogIndex, lastLogTerm := s.lastLog()

			args := requestVoteArgs{
				Term:         s.currentTerm,
				CandidateID:  s.me,
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}

			_, _ = s.requestVote(context.Background(), peer, args)
		})
	}

	wg.Wait()
}
```

- **Current situation:** the outer lock happens to exclude writers.
- **Hidden problem:** removing that oversized lock would make these reads race
  with incoming RPC handlers.
- **Effect:** one request could contain values taken from different Raft states.
- **Lesson:** build an immutable request snapshot under the lock, then give that
  snapshot to the RPC goroutine.

## Treating a reset timer as if it cancels an observed timeout

```go
func (s *server) run() {
	for {
		select {
		case <-s.stopCh:
			return
		case <-s.timer.C:
			// The timer event has already been observed here.
			s.handleTimeout()
		}
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

func (s *server) AppendEntries(/* ... */) (appendEntriesReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// This reset cannot cancel a timeout already selected by run.
	s.resetElectionTimer()
	return appendEntriesReply{Success: true}, nil
}
```

- **Problem:** `run` may already have received the timeout before another
  goroutine resets the timer.
- **Important:** resetting a timer cannot retract an event already observed.
- **Effect:** the server may start an election immediately after a valid
  heartbeat.
- **Lesson:** give timer ownership to one goroutine and treat timeout events as
  potentially stale. Recheck an election deadline or generation.

## Using uncancellable RPCs for role-specific work

```go
func (s *server) doCandidate() {
	var wg sync.WaitGroup

	for _, peer := range s.peers {
		wg.Go(func() {
			// This context is never cancelled when the election ends,
			// the term changes, or the server is asked to stop.
			reply, err := s.requestVote(
				context.Background(),
				peer,
				args,
			)
			_ = reply
			_ = err
		})
	}

	// doCandidate cannot return until every RPC returns.
	wg.Wait()
}

func (s *server) stop() {
	close(s.stopCh)

	// run may still be blocked inside doCandidate -> wg.Wait.
	<-s.done
}
```

- **Problem:** election RPCs have no lifecycle or election-specific
  cancellation.
- **Effect:** old work can outlive its election and can delay shutdown.
- **Lesson:** use a context tied to the server or election lifecycle.
- **Important:** cancellation improves cleanup; it does not replace term and
  state validation.

## Letting old leader replies mutate new leader state

```text
Leader in term T
    |
    | send AppendEntries(peer, nextIndex = 10)
    v
RPC is in flight
    |
    | receive a higher term from another server
    v
Become follower in term T+1
    |
    | old AppendEntries reply from term T arrives
    v
Incorrectly update nextIndex or matchIndex for the old leadership

Another possible ordering for one follower:

request A sent with nextIndex = 10
request B sent with nextIndex = 12
request B succeeds first
request A fails later
old failure moves nextIndex backward after the newer success
```

- **Future risk:** replication replies can arrive after the leader has stepped
  down or entered a newer term.
- **Effect:** an old reply may corrupt `nextIndex`, `matchIndex`, or a commit
  decision.
- **Second risk:** two replies from the same follower may arrive out of order.
- **Lesson:** verify `state == leader` and the sending term before applying a
  reply. Serializing replication per follower avoids many ordering problems.

## The mental model

```text
lock   -> make one atomic state transition and take a snapshot
unlock -> perform RPC or other blocking work
lock   -> verify role + term, then apply the result if it is still relevant
unlock
```

- **Memory safety:** consistently protect shared fields.
- **Freshness:** never let an old event mutate a newer term or role.
- **Liveness:** never let network work block local state transitions.
- **Key idea:** a mutex protects memory; it does not prove that an asynchronous
  event is still logically valid.

