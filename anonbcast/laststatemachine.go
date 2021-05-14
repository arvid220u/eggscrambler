package anonbcast

import (
	"sync"
	"time"
)

// lastStateMachine is a simple way of storing the last version of the state machine.
type lastStateMachine struct {
	sm      StateMachine
	mu      sync.Mutex
	version int
	// timestamp stores the time at which this update was received
	timestamp time.Time
	// phaseTimestamp stores the time at which the phase or round was last updated
	phaseTimestamp time.Time
}

func newLastStateMachine() *lastStateMachine {
	var l lastStateMachine
	l.sm.initRound(0)
	l.version = -1
	l.timestamp = time.Now()
	l.phaseTimestamp = time.Now()
	return &l
}

// set updates the last state machine if and only if version is bigger than the current version
func (l *lastStateMachine) set(sm StateMachine, version int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if version > l.version {
		l.version = version
		l.timestamp = time.Now()
		if sm.Round != l.sm.Round || sm.CurrentRoundInfo().Phase != l.sm.CurrentRoundInfo().Phase {
			l.phaseTimestamp = l.timestamp
		}
		l.sm = sm
	}
}

// get returns the last state machine
func (l *lastStateMachine) get() StateMachine {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sm
}

// deepCopy returns a snapshot of the provided lastStateMachine,
// making it possible to read the values atomically
func (l *lastStateMachine) deepCopy() *lastStateMachine {
	l.mu.Lock()
	defer l.mu.Unlock()
	var l2 lastStateMachine
	l2.sm = l.sm.DeepCopy()
	l2.version = l.version
	l2.timestamp = l.timestamp
	l2.phaseTimestamp = l.phaseTimestamp
	return &l2
}
