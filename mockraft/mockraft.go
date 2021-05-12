package mockraft

import (
	"sync"

	"github.com/arvid220u/6.824-project/raft"
)

// Mockraft is a single-server mock of Raft, that always commits every single entry, immediately.
type Mockraft struct {
	log     []interface{}
	applyCh chan raft.ApplyMsg
	mu      sync.Mutex
}

func (m *Mockraft) Start(command interface{}) (int, int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.log = append(m.log, command)
	go func(m *Mockraft, command interface{}, index int) {
		m.applyCh <- raft.ApplyMsg{
			Command:      command,
			CommandTerm:  1,
			CommandIndex: index,
			CommandValid: true,
		}
	}(m, command, len(m.log))
	return len(m.log), 1, true
}

func (m *Mockraft) Kill() {
}

func (m *Mockraft) Snapshot(index int, snapshot []byte) {
}

func (m *Mockraft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {
	return true
}
func (m *Mockraft) GetState() (int, bool) {
	return 1, true
}

func (m *Mockraft) GetApplyCh() <-chan raft.ApplyMsg {
	return m.applyCh
}

// Implemented as no-op as it shouldn't be called on mockraft
func (m *Mockraft) GetProvisionalConfiguration() (bool, map[int]bool) {
	return true, make(map[int]bool)
}

// Implemented as no-op as it shouldn't be called on mockraft
func (m *Mockraft) GetCurrentConfiguration() (bool, map[int]bool) {
	return true, make(map[int]bool)
}

// Implemented as no-op as it shouldn't be called on mockraft
func (m *Mockraft) AddProvisional(peer int) (int, raft.AddProvisionalError) {
	return 0, raft.AP_SUCCESS
}

// Implemented as no-op as it shouldn't be called on mockraft
func (m *Mockraft) RemoveServer(peer int) (<-chan bool, raft.AddRemoveServerError) {
	return nil, raft.AR_OK
}

// Implemented as no-op as it shouldn't be called on mockraft
func (m *Mockraft) AddServer(peer int) (<-chan bool, raft.AddRemoveServerError) {
	return nil, raft.AR_OK
}

func New(applyCh chan raft.ApplyMsg) *Mockraft {
	return &Mockraft{applyCh: applyCh}
}
