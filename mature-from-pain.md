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

# Second-pass review: mutex-safe, but workflow ownership is unclear

- Snapshot: commit `dd79cbcac8eac97c109392d8972fbb34724eb70b`
- Context: the first pass fixed the obvious deadlocks, snapshots outbound RPC
  arguments, validates terms after replies, and adds heartbeat replication.
- Remaining problem: several goroutines can start the same logical workflow.
  The mutex protects individual field accesses, but no component owns the
  complete lifecycle of an election, a timer, or replication to one follower.

## Starting overlapping replication rounds for the same follower

```go
func (s *server) run() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-s.timer.C:
			s.handleTimeout()
		case <-ticker.C:
			s.mu.RLock()
			isLeader := s.state == leader
			term := s.currentTerm
			s.mu.RUnlock()

			if isLeader {
				s.replicateLogs(term)
			}
		}
	}
}

func (s *server) Start(command string) (int, int, bool) {
	s.mu.Lock()
	if s.state != leader {
		s.mu.Unlock()
		return -1, -1, false
	}
	term := s.currentTerm
	s.log = append(s.log, logEntry{Term: term, Command: command})
	s.mu.Unlock()

	s.replicateLogs(term)
	return len(s.log), term, true
}
```

- **Problem:** `replicateLogs` can be started by leader election, every
  heartbeat tick, every accepted client command, and its own retry path.
- **Effect:** one follower can have several AppendEntries RPCs in flight at the
  same time, each based on a different `nextIndex`, `commitIndex`, and log
  snapshot.
- **Why the mutex is insufficient:** it serializes individual updates, but it
  does not serialize the meaning of whole replication rounds.
- **Lesson:** give each follower one replication worker, or an equivalent
  per-follower serialization rule. New work should wake or coalesce with that
  worker rather than starting another independent loop.

## Protecting successful replies but not stale failure replies

```go
if reply.Success {
	lastReplicatedIndex := request.PrevLogIndex + len(request.Entries)

	// A later success cannot make progress go backward.
	s.matchIndex[peer] = max(s.matchIndex[peer], lastReplicatedIndex)
	s.nextIndex[peer] = max(s.nextIndex[peer], lastReplicatedIndex+1)
} else {
	if s.nextIndex[peer] > 1 {
		s.nextIndex[peer]--
	}

	s.mu.Unlock()
	sendAppendEntries(peer)
	return
}
```

```text
request A: nextIndex = 3, sent first
request B: retry uses a newer nextIndex and succeeds
           nextIndex becomes 6
request A: old failure reply arrives late
           nextIndex is decremented to 5
```

- **Problem:** `max` makes success monotonic, but a stale failure can still
  move `nextIndex` backward.
- **Effect:** replication loses proven progress or sends unnecessary old
  entries. Under more complex interleavings, this becomes very hard to reason
  about.
- **Lesson:** a reply needs to be associated with the request that produced it.
  The simplest approach is to allow only one active replication attempt per
  follower. A request sequence number is another valid approach.

## Retrying by recursive function calls

```go
reply, err := s.appendEntries(context.Background(), peer, request)
if err != nil {
	// A disconnected peer can make this recurse forever.
	sendAppendEntries(peer)
	return
}

if !reply.Success {
	if s.nextIndex[peer] > 1 {
		s.nextIndex[peer]--
	}
	s.mu.Unlock()

	// At nextIndex == 1, a repeated false reply also recurses forever.
	sendAppendEntries(peer)
	return
}
```

- **Problem:** retries consume stack frames instead of iterating.
- **Effect:** a permanently unreachable peer can cause stack growth, a leaked
  goroutine, and unbounded retry pressure. Heartbeats can start even more such
  goroutines.
- **Lesson:** retries should be an explicit loop with cancellation and a
  waiting policy. In this project, a replication worker can retry when woken
  by a heartbeat or a new command; no recursion is necessary.

## Stopping the run loop without stopping server work

```go
func (s *server) stop() {
	close(s.stopCh)
	<-s.done
}

func (s *server) appendEntries(
	ctx context.Context,
	peer id,
	args appendEntriesArgs,
) (appendEntriesReply, error) {
	return s.transport.appendEntries(ctx, s.me, peer, args)
}

// Callers currently use context.Background().
reply, err := s.appendEntries(context.Background(), peer, request)
```

- **Problem:** `done` means only `run` exited. It does not mean outbound vote
  RPCs, replication RPCs, or retry calls have stopped.
- **Effect:** a server can appear stopped while old goroutines still send
  traffic or mutate state after a delayed reply.
- **Lesson:** create a server-lifetime context in `newServer`, cancel it in
  `stop`, and derive role- or request-specific contexts when useful.
- **Important:** cancellation handles cleanup; term and role checks are still
  required for safety.

## Checking follower state, then starting an election later

```go
func (s *server) handleTimeout() {
	s.mu.RLock()
	state := s.state
	if state == leader {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	// Another goroutine may receive a valid AppendEntries here.
	s.candidateDoElection()
}

func (s *server) AppendEntries(
	ctx context.Context,
	args appendEntriesArgs,
) (appendEntriesReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resetElectionTimer()
	s.state = follower
	return appendEntriesReply{Term: s.currentTerm, Success: true}, nil
}
```

```text
run receives an old timeout event
    ↓
handleTimeout observes follower and unlocks
    ↓
AppendEntries resets the election timer and confirms a leader
    ↓
handleTimeout calls candidateDoElection anyway
```

- **Problem:** the read is mutex-protected, but the decision is not atomic with
  the state transition.
- **Effect:** a valid heartbeat can be followed immediately by an unnecessary
  election.
- **Lesson:** a timeout is an event, not proof that an election is still due.
  Revalidate the timeout when performing the transition, ideally with timer
  ownership and a deadline/generation managed by one goroutine.

## Hiding lock requirements in helper functions

```go
func (s *server) lastLog() (index, term int) {
	index = len(s.log)
	if index == 0 {
		return 0, -1
	}
	return index, s.log[index-1].Term
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

func (s *server) applyCommittedLocked() {
	for s.lastApplied < s.commitIndex {
		s.lastApplied++
		s.db = append(s.db, s.log[s.lastApplied-1].Command)
	}
}
```

- **Good example:** `applyCommittedLocked` documents its lock requirement in
  its name.
- **Problem:** `lastLog` and `resetElectionTimer` also depend on a particular
  synchronization context, but their names do not say so.
- **Effect:** a later caller can innocently introduce a data race, deadlock, or
  timer lifecycle bug.
- **Lesson:** either acquire the lock inside a public helper, or give an
  internal helper a `Locked` suffix and call it only from documented critical
  sections.

## Repeating state-transition rules in several callbacks

```go
// Higher-term AppendEntries reply.
s.state = follower
s.votedFor = ""
s.currentTerm = reply.Term
s.resetElectionTimer()

// Higher-term RequestVote.
s.currentTerm = args.Term
s.votedFor = ""
s.state = follower

// Same-term AppendEntries.
if s.state == candidate || s.state == leader {
	s.state = follower
}
```

- **Problem:** state transitions are distributed across request handlers,
  response handlers, and timeout logic.
- **Effect:** adding a new field or invariant later means every copy must be
  updated correctly. It is easy for one path to forget to clear a vote, stop
  leader work, or reset a timer.
- **Lesson:** a small helper such as `stepDownLocked(term)` is justified when
  it centralizes an actual Raft invariant. This is not abstraction for its own
  sake.

## What was already right in this revision

```text
lock   -> update state and snapshot RPC arguments
unlock -> perform network RPC
lock   -> validate term and role before using the reply
unlock
```

- Outbound RPCs do not hold `s.mu`, preventing the original distributed
  deadlock.
- Election and replication replies verify their term/role before most state
  updates.
- Request arguments are copied before an RPC, so one request is internally
  consistent.
- **Next lesson:** these local rules are necessary, but a complete sync model
  also needs ownership: who starts work, who cancels it, and who is allowed to
  have one active workflow for a peer or timer.

