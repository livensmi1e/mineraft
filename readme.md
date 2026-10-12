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
    - Greater or equal N / 2 + 1
- What happens when leader receive request from client, crash before completing?
    - When leader receive request from client, it has to do 3 things: append logs, replicate and commit log and apply logs
    - Before append logs --> lost
    - After append, but before replicate --> lost
    - After replicate, but before commit --> lost
    - After commit, but before apply --> safety
    - After apply, but before response --> safety
- How client know it is safe to retry request?
    - System neees to implement idempotency, for example requestID?
- When server reject AppendLogs RPC, leader decrease nextIndex, but when it success, what does server do with that nextIndex?
    - 

Raft deals with 2 kind of failures
- Network
- Server crash
- Not byzantine

Property:
- Election Safety 
    - At most 1 leader elected in a term
- Leader Append-Only
- Log Matching
    - If two entries in different logs have the same index and term, then they store the same command
    - If two entries in different logs have the same index and term, then the logs are identical in all preceding entries
- Leader Completeness
    - If a log entry is committed in a given term, then that entry will be present in the logs of the leaders for all higher-numbered terms
- State Machine Safety
    - If a server has applied a log entry at a given index to its state machine, no other server will ever apply a different log entry for the same index

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
- Vote restriction (5.4.1): prevent a candidate from winning an election unless its log contains all committed entries --> ensure leader for any given term con-
tains all of the entries committed in previous terms (contains all commited entries)
    - Candidate must contact a majority of the cluster in order to be elected --> candidate can only become leader if its log contain all commmited entries
    - Voter reject RequestVote RPC if its log is more up to date
    - Up to date: compare index + term of last entry in log, if different term, larger term wins, if same term, longer logs wins
- Election timeouts are chosen randomly from a fixed interval (e.g., 150–300ms)
- Start up, follower, election timer begins
- Start election, restart election timer

Log replication:
- Leader serves client request: new command, when to response to client? --> after apply entry to sate machine
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

Safty (commit)
- lastApplied: actual index of highest log entry applied to state machine (initialized to 0, increases monotonically)
- lastCommited: index of highest log entry known to be committed (initialized to 0, increases monotonically) --> safe to apply to state machine <--> has been replicated on logs of majority of server --> never lost or overwritten in the feature --> uarantees that committed entries are durable and will eventually be executed by all of the available state machines --> this is safty rule
- Leader knows that an entry from its *current term* is committed once that entry is stored on a majority of the servers
- Leader cannot immediately conclude that an entry from a *previous term* is committed once it is stored on a majority of servers?
- Only log entries from the leader’s current term are committed by counting replicas
- Once an entry from the current term has been committed in this way, then all prior entries are committed indirectly 
- Not understand proof yet but understand intention: in order to save entries from previous term, leader force to commit entry from its current term, therefore all preceding entries is known to be commited (log matching property)

Follower/Candidate crash
- Follower/Candidate crash before --> future RequestVote and AppendEntries RPCs sent to it will fail --> retry indefinitely
- Follower/Candidate crash after complete RPC before response: receive the same RPC when restart
- Raft RPCs are idempotent

Timing
- Safety must not depend on timing
- Availabity (the ability of the system to respond to clients in a timely manner) inevitably depend on timing
- Without steady leader, cannot progresss --> maintain steady leader as long as broadcastTime << electionTimeout << MTBF


## Implementation

What
- Not a library, this is a lab simulating Raft cluster

Steps (focus on Raft logic first, then cluster and simulation logic later)
- Define server state
- Implement election timer
- Implement RequestVote RPC request & response
- Implement AppendEntries RPC request & response
- Manage persistent state & inject to server when reboot
- Define cluster
- Define RPC as channel

## Test

Questions
- How to test?
- Level? Unit? System? Write unit test for a behavior / invariant in Raft rather than every function
- How to simulate?
    - Each server is a process --> hard to test, hard to manage --> each server is a Go object that runs in each Go routine
    - RPC is Go channel
- How to make a server fail and restart?


## Read
- https://thesquareplanet.com/blog/students-guide-to-raft/
- https://pdos.csail.mit.edu/6.5840/index.html