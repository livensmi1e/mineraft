package main

type State int

const (
	Follower State = iota
	Candidate
	Leader
)

type ID string

type Log struct {
	cmd  string // represent as "x = 3"
	term int
}

type Server struct {
	me    ID
	state State

	// volatile
	commitIndex int // index of highest log entry known to be committed, increase monotonically
	lastApplied int // index of highest log entry applied to state machine, increase monotonically

	nextIndex  map[ID]int // index of the next log entry to send to each server
	matchIndex map[ID]int // index of highest log entry known to be replicated on each servers

	// persistent
	currentTerm int // latest term server has seen, increases monotonically
	votedFor    ID
	log         []Log
}

func (s Server) lastLogIndex() int {
	return len(s.log)
}

func NewServer(me ID, peers []ID, currentTerm int, votedFor ID, log []Log) *Server {
	server := &Server{
		me:    me,
		state: Follower,

		currentTerm: currentTerm,
		votedFor:    votedFor,
		log:         log,
	}
	for _, peer := range peers {
		server.nextIndex[peer] = server.lastLogIndex()
	}
	return server
}
