package libraft

type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandTerm  int
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int

	RaftUpdateValid bool // Indicates that the leader has changed
	IsLeader        bool

	ConfigValid   bool // Indicates server configuration has changed
	Configuration map[string]bool
}

// Raft can manage a shared log.
type Raft interface {
	// Start initiates consensus on the supplied command.
	// If this server isn't the leader, return false. Otherwise, start
	// the agreement and return immediately.
	//
	// The first return value is the index that the command will appear at
	// if it's ever committed. The second return value is the current
	// term. The third return value is true if this server believes it is
	// the leader.
	Start(command interface{}) (int, int, bool)

	// Kill kills all long-running goroutines and releases any memory
	// used by the Raft instance. After calling Kill no other methods
	// may be called.
	Kill()

	// Snapshot should be called after the service using Raft has created
	// a snapshot of its data up to and including index. Raft may then
	// discard a prefix of its log.
	Snapshot(index int, snapshot []byte)

	// CondInstallSnapshot returns true if the service should install a snapshot
	// that was given to it on the applyCh. If false is returned, the service should
	// discard the snapshot.
	CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool

	// GetApplyCh returns a channel that Raft sends updates on. The same channel
	// is always returned.
	GetApplyCh() <-chan ApplyMsg

	// GetState returns currentTerm and whether this server
	// believes it is the leader.
	// Useful for testing.
	GetState() (int, bool)

	// GetProvisionalConfiguration returns whether or not the server thinks it's the leader
	// and a set of servers with provisional status
	GetProvisionalConfiguration() (bool, map[string]bool)

	// GetCurrentConfiguration returns whether or not the server thinks it's the leader
	// and a set of servers with voting status
	GetCurrentConfiguration() (bool, map[string]bool)

	AddProvisional(peer string) (int, AddProvisionalError)

	RemoveServer(peer string) (<-chan bool, AddRemoveServerError)

	AddServer(peer string) (<-chan bool, AddRemoveServerError)
}
