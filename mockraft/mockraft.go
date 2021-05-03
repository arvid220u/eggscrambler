package mockraft

import "github.com/arvid220u/6.824-project/raft"

// Mockraft is a single-server mock of Raft, that always commits every single entry, immediately.
type Mockraft struct {
	log     []interface{}
	applyCh chan raft.ApplyMsg
}

func (m *Mockraft) Start(command interface{}) (int, int, bool) {
	m.log = append(m.log, command)
	go func(m *Mockraft, command interface{}) {
		m.applyCh <- raft.ApplyMsg{
			Command:      command,
			CommandTerm:  1,
			CommandIndex: len(m.log),
			CommandValid: true,
		}
	}(m, command)
	return len(m.log), 1, true
}

func (m *Mockraft) Kill() {
}

func (m *Mockraft) Snapshot(index int, snapshot []byte) {
}

func (m *Mockraft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {
	return true
}

func (m *Mockraft) GetApplyCh() <-chan raft.ApplyMsg {
	return m.applyCh
}

func New(applyCh chan raft.ApplyMsg) *Mockraft {
	return &Mockraft{applyCh: applyCh}
}
