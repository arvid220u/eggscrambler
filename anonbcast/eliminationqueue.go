package anonbcast

import (
	"sync"
)

// eliminationQueue keeps track of operations that have yet to be known to be committed to the leader's log,
// and that the client wants to send. It enables the elimination of many unnecessary RPC calls by always only
// keeping the most recent op memorized.
type eliminationQueue struct {
	mu           sync.Mutex
	cond         *sync.Cond
	ops          []Op
	lastFinished Op
}

// must be called with the lock HELD
func (p *eliminationQueue) checkRep() {
	if !IsDebug() {
		return
	}
	assertf(len(p.ops) <= 1, "only one operation at a time possible")
}

func newEliminationQueue() *eliminationQueue {
	var p eliminationQueue
	p.cond = sync.NewCond(&p.mu)
	p.lastFinished = AbortOp{R: -1}
	return &p
}

// add adds the op to the queue of pending operations, if it is not already there,
// and if a newer op is not already there. It removes all older operations.
func (p *eliminationQueue) add(op Op) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.checkRep()
	defer p.checkRep()
	// if this operation is not newer than the last finished one, don't bother
	if compare(op, p.lastFinished) != opNewer {
		return
	}
	if len(p.ops) != 0 {
		switch compare(p.ops[0], op) {
		case opNewer:
			return
		case opEqual:
			return
		}
		p.ops[0] = op
	} else {
		p.ops = append(p.ops, op)
	}
	// if we reach this point, we modified, so we want to wake up
	p.cond.Broadcast()
}

// get blocks until there is a new operation, at which point it returns it.
func (p *eliminationQueue) get() Op {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.checkRep()
	defer p.checkRep()
	for len(p.ops) == 0 {
		p.cond.Wait()
	}
	return p.ops[0]
}

// finish removes the operation from the pending queue and stores it in the last finished.
func (p *eliminationQueue) finish(op Op) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.checkRep()
	defer p.checkRep()
	if len(p.ops) > 0 {
		if compare(p.ops[0], op) == opEqual {
			p.ops = nil
		}
	}
	if compare(p.lastFinished, op) == opOlder {
		p.lastFinished = op
	}
}

type opComparison string

const (
	opNewer        opComparison = "opNewer"
	opOlder        opComparison = "opOlder"
	opEqual        opComparison = "opEqual"
	opIncomparable opComparison = "opIncomparable"
)

// opIndex is a helper function to compare
func opIndex(op Op) int {
	switch op.Type() {
	case PublicKeyOpType:
		return 0
	case StartOpType:
		return 1
	case MessageOpType:
		return 2
	case ScrambledOpType:
		return 3
	case DecryptedOpType:
		return 4
	case RevealOpType:
		return 5
	case AbortOpType:
		return 6
	default:
		assertf(false, "should never happen")
		return -1
	}
}

// compare returns statements about a in relation to b. A return value
// of opNewer means that a is considered to be a newer operation than b,
// from the perspective of the client.
func compare(a Op, b Op) opComparison {
	if a.Round() < b.Round() {
		return opOlder
	}
	if a.Round() > b.Round() {
		return opNewer
	}

	// old -> new
	// publicKey, start, message, scrambled, decrypted, reveal, abort

	aIndex := opIndex(a)
	bIndex := opIndex(b)

	if aIndex < bIndex {
		return opOlder
	}
	if aIndex > bIndex {
		return opNewer
	}

	// we only have internal orderings within the scrambled and decrypted phases
	switch as := a.(type) {
	case ScrambledOp:
		bs := b.(ScrambledOp)
		if as.Prev < bs.Prev {
			return opOlder
		}
		if as.Prev > bs.Prev {
			return opNewer
		}
	case DecryptedOp:
		bs := b.(DecryptedOp)
		if as.Prev < bs.Prev {
			return opOlder
		}
		if as.Prev > bs.Prev {
			return opNewer
		}
	}

	// at this point, if the type is the same, they have the same newness!
	if a.Type() == b.Type() {
		return opEqual
	}

	// we should never get here...
	assertf(false, "we should never get here")
	return opIncomparable
}
