package main

import (
	"math/rand"
	"time"
)

type state int

const (
	follower state = iota
	candidate
	leader
)

const (
	minElectionTimeout = 150 * time.Millisecond
	maxElectionTimeout = 300 * time.Millisecond
)

type id string

type logEntry struct {
	cmd  string // represent as "x = 3"
	term int
}

type request struct {
}

type server struct {
	me    id
	state state

	// volatile
	commitIndex int // index of highest log entry known to be committed, increase monotonically
	lastApplied int // index of highest log entry applied to state machine, increase monotonically

	nextIndex  map[id]int // index of the next log entry to send to each server
	matchIndex map[id]int // index of highest log entry known to be replicated on each servers

	// persistent
	currentTerm int // latest term server has seen, increases monotonically
	votedFor    id
	log         []logEntry

	requestCh chan request
	timer     *time.Timer
	done      chan struct{}
}

func newServer(me id, peers []id, currentTerm int, votedFor id, log []logEntry) *server {
	server := &server{
		me:    me,
		state: follower,

		nextIndex:  make(map[id]int),
		matchIndex: make(map[id]int),

		currentTerm: currentTerm,
		votedFor:    votedFor,
		log:         log,

		timer: time.NewTimer(randomElectionTimeout()),
		done:  make(chan struct{}),
	}
	for _, peer := range peers {
		if peer != me {
			server.nextIndex[peer] = server.lastLogIndex()
		}
	}
	go server.run()
	return server
}

func (s server) lastLogIndex() int {
	return len(s.log)
}

func (s server) run() {
	defer close(s.done)
}

func randomElectionTimeout() time.Duration {
	return minElectionTimeout + time.Duration(rand.Int63n(int64(maxElectionTimeout-minElectionTimeout)))
}
