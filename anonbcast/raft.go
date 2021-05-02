package anonbcast

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
	GetApplyCh() chan RaftMsg
}

type RaftMsgType int

const (
	RaftMsgCommand RaftMsgType = iota
	RaftMsgSnapshot
	RaftMsgUpdate
)

// RaftMsg represents a message that is sent from Raft to the service using it.
type RaftMsg interface {
	// Type returns the type of raft message, which is a command, a snapshot or an update
	Type() RaftMsgType
	// Term returns the term of the message. precondition: Type is command or snapshot
	Term() int
	// Index returns the index of the message. precondition: Type is command or snapshot
	Index() int
	// Command returns the contents of the command. precondition: Type is command
	Command() interface{}
	// Snapshot returns the contents of the snapshot. precondition: Type is snapshot
	Snapshot() []byte
	// Leader returns whether it is a leader or not. precondition: Type is update
	Leader() bool
}

type mockraftmsg struct {
	command interface{}
	term    int
	index   int
}

func (m mockraftmsg) Type() RaftMsgType {
	return RaftMsgCommand
}

func (m mockraftmsg) Term() int {
	return m.term
}

func (m mockraftmsg) Index() int {
	return m.index
}

func (m mockraftmsg) Command() interface{} {
	return m.command
}

func (m mockraftmsg) Snapshot() []byte {
	return nil
}

func (m mockraftmsg) Leader() bool {
	return true
}

// mockraft is a single-server mock of Raft, that always commits every single entry, immediately.
type mockraft struct {
	log     []interface{}
	applyCh chan RaftMsg
}

func (m *mockraft) Start(command interface{}) (int, int, bool) {
	m.log = append(m.log, command)
	go func(m *mockraft, command interface{}) {
		m.applyCh <- mockraftmsg{
			command: command,
			term:    1,
			index:   len(m.log),
		}
	}(m, command)
	return len(m.log), 1, true
}

func (m *mockraft) Kill() {
}

func (m *mockraft) Snapshot(index int, snapshot []byte) {
}

func (m *mockraft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {
	return true
}

func (m *mockraft) GetApplyCh() chan RaftMsg {
	return m.applyCh
}

func newMockraft(applyCh chan RaftMsg) *mockraft {
	return &mockraft{applyCh: applyCh}
}
