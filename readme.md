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
    - Each server is a container
    - Build a fake random failure on server(s)
    - Script everything to automate the simulation
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
    - If two entries in different logs have the same index and term, then they store the same command
    - If two entries in different logs have the same index and term, then the logs are identical in all preceding entries
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

Leader election:
- Server startup --> follower
- Remain in follower when receive valid RPCs from leader or candidate
- Leader sends end periodic heartbeats (AppendEntries RPCs that carry no log entries) to maintain authority
- When follower's election timeout (receive no communication) --> begin election
- To begin election, follower increase current term, become candidate
- Votes for itself and issues RequestVote RPCs in parallel to each of the other server
- Remain candidate until 1 of 3:
    - Wins election --> leader: Receive majority of votes --> Server vote for at most 1 candidate, first come first serve --> send heartbeat to others
    - Another leader --> While candidte waiting for vote, receive AppendEntries RPC from another server claim to be leader, if leader's term is >= candidate current --> trust, become follower, else reject, continue candidate
    - No winner --> measure split vote --> candidate timeout, start new election
- Vote restriction (5.4.1): prevent a candidate from winning an election unless its log contains all committed entries
    - Candidate must contact a majority of the cluster in order to be elected --> candidate can only become leader if its log contain all commmited entries
    - Voter reject RequestVote RPC if its log is more up to date
    - Up to date: compare index + term of last entry in log, if different term, larger term wins, if same term, longer logs wins
- Election timeouts are chosen randomly from a fixed interval (e.g., 150–300ms)
- Start up, follower, election timer begins
- Start election, restart election timer

Log replication:
- Leader serves client request: new command, when to response to client?
- Leader append new entry to logs, issue AppendEntries RPCs parallel
- Log entry: command + term when leader receive that command
- Leader decides when it is safe to commit to state machine: once leader have replicated on a majority of servers --> means all preceding logs are also commited (even created by previous leaders)
- Leader keep track highest index known to be committed, include that index in feature Append RPCs --> other server eventually find out
- Consistency check: when send AppendEntries RPC, include index and term of the entry in its log that immediately precedes the new entry --> follower not find entry in its logs with same index + term, reject
- Conflicting entries in follower logs will be overwritten with entries from the leader’s logs
    - Leader find latest logs entry where 2 logs agree, delete entries after that point, send all leader's entries after that point to follower
    - Leader maintains next index for each follower - index of next log entry going to send to follower
    - First come power, init next index = last index entry + 1
    - Append RPCs fail, retry with next index decrease
- If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry) --> why???