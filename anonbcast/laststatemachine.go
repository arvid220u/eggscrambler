package anonbcast

import "sync"

// lastStateMachine is a simple way of storing the last version of the state machine.
type lastStateMachine struct {
	sm      StateMachine
	mu      sync.Mutex
	version int
}

func newLastStateMachine() *lastStateMachine {
	var l lastStateMachine
	l.sm.initRound(0)
	l.version = -1
	return &l
}

// set updates the last state machine if and only if version is bigger than the current version
func (l *lastStateMachine) set(sm StateMachine, version int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if version > l.version {
		l.sm = sm
		l.version = version
	}
}

// get returns the last state machine
func (l *lastStateMachine) get() StateMachine {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sm
}
