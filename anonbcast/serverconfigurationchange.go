package anonbcast

import (
	"github.com/arvid220u/eggscrambler/libraft"
)

type AddRemoveArgs struct {
	Server int
	IsAdd  bool
}

type AddRemoveReply struct {
	Submitted bool
	Error     libraft.AddRemoveServerError
}

type AddProvisionalArgs struct {
	Server int
}

type AddProvisionalReply struct {
	Error libraft.AddProvisionalError
}

type InConfigurationArgs struct {
	Server           int
	IsProvisionalReq bool
}

type InConfigurationReply struct {
	InConfiguration bool
	IsLeader        bool
}

// Returns whether or not this Raft is a leader and whether or not the
// given server is in this raft's configuration.
func (s *Server) InConfiguration(args *InConfigurationArgs, reply *InConfigurationReply) {
	var isLeader bool
	var conf map[int]bool
	if args.IsProvisionalReq {
		isLeader, conf = s.rf.GetProvisionalConfiguration()
	} else {
		isLeader, conf = s.rf.GetCurrentConfiguration()
	}

	_, ok := conf[args.Server]
	reply.InConfiguration = ok
	reply.IsLeader = isLeader
}

// Essentially a pass through for the AddProvisional raft RPC
func (s *Server) AddProvisional(args *AddProvisionalArgs, reply *AddProvisionalReply) {
	_, err := s.rf.AddProvisional(args.Server)
	reply.Error = err
}

// Essentially a pass through for the AddRemove raft RPC
func (s *Server) AddRemove(args *AddRemoveArgs, reply *AddRemoveReply) {
	var err libraft.AddRemoveServerError
	if args.IsAdd {
		_, err = s.rf.AddServer(args.Server)
	} else {
		_, err = s.rf.RemoveServer(args.Server)
	}

	if err == libraft.AR_OK || err == libraft.AR_ALREADY_COMPLETE {
		reply.Submitted = true
		reply.Error = libraft.AR_OK
	} else {
		reply.Submitted = false
		reply.Error = err
	}
}
