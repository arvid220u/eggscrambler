package anonbcast

// Server implements the shared state machine on top of Raft, that is used
// for performing the anonymous broadcasting protocol. It exposes RPCs that
// can be called by a Client on a different machine, and exposes a channel
// that can be consumed by a local Client for reading the latest state.
type Server interface {
}

type server struct {
}

func NewServer() Server {
	return &server{}
}
