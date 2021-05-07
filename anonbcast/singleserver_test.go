package anonbcast

import (
	"testing"

	"github.com/arvid220u/6.824-project/mockraft"
	"github.com/arvid220u/6.824-project/raft"
)

func TestServerMockraftNoFailures(t *testing.T) {
	SetDebug(true)
	SetDump(true)

	applyCh := make(chan raft.ApplyMsg)
	rf := mockraft.New(applyCh)
	s := NewServer(rf)

	updCh := s.GetUpdCh()

	// assert initial state is correct
	st := <-updCh
	ri, err := st.GetRoundInfo(0)
	if err != nil {
		t.Fatal(err)
	}
	if ri.Phase != PreparePhase {
		t.Errorf("initial phase is %v", ri.Phase)
	}
}
