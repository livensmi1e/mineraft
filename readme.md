## Process

1. Read paper to understand high level

2. Read again to understand low level

3. Read again until reach wow moment

3. Implement

## Notes

Includes:
- Leader election
- Log replication
- Safty with restriction

Not in scope:
- Cluster membership changes

Questions:
- How to simutate client/server, failures,...?
- What storage to use? after node recover, does storage must recover or can lost all? 
    - For logs, no, this can be in memory because after joining cluster, it will eventually have the same logs as leader?
    - For state machine, no --> it can be lost on node failures, because after node recovers and join the cluster again, its log will reach the common state, then we can apply the logs to the state machine
- What is definition of majority?

Raft deals with 2 kind of failures
- Network
- Server crash
- Not byzantine

Property:
- Election Safety
- Leader Append-Only
- Log Matching
- Leader Completeness
- State Machine Safety

Server state
- Three states: leader, follower, or candidate
- Follower passive: issue no request --> if receive request from client --> redirect to leader
- Leader receive request from client
- Transition

Term:
- Is a logical clock
- Term begins with an election, ends up in no leader if election fails or 1 leader if success --> at most 1
- To detect obsolete information: stale leader
- Each server must store current term number (init with 0), increase mono, exchanged whenever servers communicate
- Server discovers term < other's --> update current term to that larger
- Server receive request with term < its term --> reject
- Leader or candidate discover term < other's --> become follower

RPC:
- RequestVote
- AppendEntries: only from leader, to replicate log, heartbeat
- If not receive response within time, retry
- Issue in parallel