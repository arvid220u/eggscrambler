package mockraft

import (
	"github.com/arvid220u/eggscrambler/libraft"
	"sync"
)

// Mockraft is a single-server mock of Raft, that always commits every single entry, immediately.
type Mockraft struct {
	log     []interface{}
	applyCh chan libraft.ApplyMsg
	mu      sync.Mutex
}

func (m *Mockraft) Start(command interface{}) (int, int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.log = append(m.log, command)
	go func(m *Mockraft, command interface{}, index int) {
		m.applyCh <- libraft.ApplyMsg{
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

func (m *Mockraft) GetApplyCh() <-chan libraft.ApplyMsg {
	return m.applyCh
}

// Implemented as no-op as it shouldn't be called on mockraft
func (m *Mockraft) GetProvisionalConfiguration() (bool, map[int]bool) {
	return true, make(map[int]bool)
}

// Implemented as no-op as it shouldn't be called on mockraft
func (m *Mockraft) GetCurrentConfiguration() (bool, map[int]bool) {
	mp := make(map[int]bool)
	mp[0] = true
	return true, mp
}

// Implemented as no-op as it shouldn't be called on mockraft
func (m *Mockraft) AddProvisional(peer int) (int, libraft.AddProvisionalError) {
	return 0, libraft.AP_SUCCESS
}

// Implemented as no-op as it shouldn't be called on mockraft
func (m *Mockraft) RemoveServer(peer int) (<-chan bool, libraft.AddRemoveServerError) {
	return nil, libraft.AR_OK
}

// Implemented as no-op as it shouldn't be called on mockraft
func (m *Mockraft) AddServer(peer int) (<-chan bool, libraft.AddRemoveServerError) {
	return nil, libraft.AR_OK
}

func New(applyCh chan libraft.ApplyMsg) *Mockraft {
	return &Mockraft{applyCh: applyCh}
}
