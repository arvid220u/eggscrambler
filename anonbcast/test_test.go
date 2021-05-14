package anonbcast

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/arvid220u/6.824-project/labrpc"
	"github.com/arvid220u/6.824-project/network"

	"github.com/arvid220u/6.824-project/mockraft"
	"github.com/arvid220u/6.824-project/raft"
)

func TestServerMockraftNoFailures(t *testing.T) {
	applyCh := make(chan raft.ApplyMsg)
	rf := mockraft.New(applyCh)
	s := NewServer(0, rf)

	updCh := s.GetUpdCh()

	// assert initial state is correct
	upd := <-updCh
	if !upd.StateMachineValid {
		t.Errorf("Expected StateMachine update but got: %v", upd)
	}

	st := upd.StateMachine
	ri, err := st.GetRoundInfo(0)
	if err != nil {
		t.Fatal(err)
	}
	if ri.Phase != PreparePhase {
		t.Errorf("initial phase is %v", ri.Phase)
	}
}

type MessageGenerator struct {
	id string
	mu *sync.Mutex
}

func (mg MessageGenerator) Message(round int) []byte {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	return []byte(fmt.Sprintf("message in round %d from %s", round, mg.id))
}

func TestServerClientSingleMachineNoFailures(t *testing.T) {
	applyCh := make(chan raft.ApplyMsg)
	rf := mockraft.New(applyCh)
	s := NewServer(0, rf)

	net := labrpc.MakeNetwork()
	defer net.Cleanup()
	end1 := net.MakeEnd("client1")
	end2 := net.MakeEnd("client2")
	end3 := net.MakeEnd("client3")
	svc := labrpc.MakeService(s)
	srv := labrpc.MakeServer()
	srv.AddService(svc)
	net.AddServer("server", srv)
	net.Connect("client1", "server")
	net.Enable("client1", true)
	net.Connect("client2", "server")
	net.Enable("client2", true)
	net.Connect("client3", "server")
	net.Enable("client3", true)

	cp1 := network.New([]*labrpc.ClientEnd{end1})
	cp2 := network.New([]*labrpc.ClientEnd{end2})
	cp3 := network.New([]*labrpc.ClientEnd{end3})

	var mu sync.Mutex
	mg1 := MessageGenerator{
		id: "1",
		mu: &mu,
	}
	mg2 := MessageGenerator{
		id: "2",
		mu: &mu,
	}
	mg3 := MessageGenerator{
		id: "3",
		mu: &mu,
	}

	c1 := NewClient(s, mg1, cp1)
	results := c1.GetResCh()
	orderedResults := make(chan RoundResult)
	ro := &resultOrderer{}
	go ro.order(results, orderedResults, 0)
	c2 := NewClient(s, mg2, cp2)
	c3 := NewClient(s, mg3, cp3)

	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			// race to the start
			err := c1.Start(i)
			for err != nil {
				time.Sleep(time.Millisecond * 1)
				err = c1.Start(i)
			}
		} else {
			// wait for everyone to submit :)
			sm1 := c1.GetLastStateMachine()
			for sm1.Round != i || len(sm1.CurrentRoundInfo().Participants) < 3 {
				time.Sleep(time.Millisecond * 5)
				sm1 = c1.GetLastStateMachine()
			}
			err := c1.Start(i)
			if err != nil {
				t.Fatalf("problem occurred in start should never happen: %v", err)
			}
		}
		r := <-orderedResults
		if r.Round != i {
			t.Fatalf("Round %d not equal to i %d", r.Round, i)
		}
		if !r.Succeeded {
			t.Fatalf("Round %d failed which should not happen", r.Round)
		}
		var expectedMessages [][]byte
		if i%2 == 0 {
			expectedMessages = [][]byte{mg1.Message(r.Round)}
			sm2 := c2.GetLastStateMachine()
			for sm2.Round != r.Round+1 {
				time.Sleep(time.Millisecond * 5)
				sm2 = c2.GetLastStateMachine()
			}
			ri, err := sm2.GetRoundInfo(r.Round)
			if err != nil {
				t.Fatalf("error getting round info: %v", err)
			}
			for _, p := range ri.Participants {
				if p == c2.Id {
					expectedMessages = append(expectedMessages, mg2.Message(r.Round))
				}
				if p == c3.Id {
					expectedMessages = append(expectedMessages, mg3.Message(r.Round))
				}
			}
		} else {
			expectedMessages = [][]byte{mg1.Message(r.Round), mg2.Message(r.Round), mg3.Message(r.Round)}
		}
		if !equalContentsBytes(expectedMessages, r.Messages) {
			t.Fatalf("Wrong messages for this round. Expected %v, got %v", expectedMessages, r.Messages)
		}
	}

	ro.kill()
	c1.Kill()
	c2.Kill()
	c3.Kill()

	time.Sleep(100 * time.Millisecond)

	s.Kill()
}

func TestReliableNetBasic(t *testing.T) {
	cfg := make_config(t, 3, false, -1)
	defer cfg.cleanup()
	cfg.begin("Basic broadcast over reliable network")
	time.Sleep(3 * time.Second) // generous amount of time for leader election

	round := 0
	for i := 0; i < 10; i++ {
		fmt.Printf("Attempting broadcast round %d\n", i)
		round = cfg.oneRound(round) + 1
		time.Sleep(1 * time.Second) // probably could be much shorter
	}

	cfg.end()
}

func TestUnreliableNetBasic(t *testing.T) {
	cfg := make_config(t, 3, true, -1)
	defer cfg.cleanup()
	cfg.begin("Basic broadcast over unreliable network")
	time.Sleep(3 * time.Second) // generous amount of time for leader election

	round := 0
	for i := 0; i < 10; i++ {
		fmt.Printf("Attempting broadcast round %d\n", i)
		round = cfg.oneRound(round) + 1
		time.Sleep(1 * time.Second) // probably could be much shorter
	}

	cfg.end()
}

func TestParticipantsAllInInitialConfiguration(t *testing.T) {
	servers := 3
	expectedConfiguration := make(map[int]bool)
	for i := 0; i < servers; i++ {
		expectedConfiguration[i] = true
	}

	cfg := make_config(t, 3, false, -1)
	defer cfg.cleanup()
	cfg.begin("Checking all participants present when all servers in configuration")
	time.Sleep(3 * time.Second)

	cfg.checkConfigurationMatchesParticipants(expectedConfiguration)
}
