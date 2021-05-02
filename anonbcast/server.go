package anonbcast

// Server implements the shared state machine on top of Raft, that is used
// for performing the anonymous broadcasting protocol. It exposes RPCs that
// can be called by a Client on a different machine, and exposes a channel
// that can be consumed by a local Client for reading the latest state.
type Server struct {
	rf Raft
}

func NewServer(rf Raft) *Server {
	return &Server{rf: rf}
}

type Err string

const (
	OK             = "OK"
	ErrWrongLeader = "ErrWrongLeader"
)

type RpcReply struct {
	Err Err
}

// SubmitOp is an RPC for clients to submit operations to the state machine.
// Returns ErrWrongLeader if not the leader and OK otherwise.
func (s *Server) SubmitOp(args Op, reply *RpcReply) {

}
