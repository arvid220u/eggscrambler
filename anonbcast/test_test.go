package anonbcast

import (
	"fmt"
	"github.com/arvid220u/6.824-project/labrpc"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/arvid220u/6.824-project/mockraft"
	"github.com/arvid220u/6.824-project/raft"
)

func TestServerMockraftNoFailures(t *testing.T) {
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

type MessageGenerator struct {
	id string
	mu *sync.Mutex
}

func (mg MessageGenerator) Message(round int) string {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	return fmt.Sprintf("message in round %d from %s", round, mg.id)
}

func resultOrderer(unorderedResults <-chan RoundResult, orderedResults chan<- RoundResult) {
	var results []RoundResult
	applyIndex := 0
	for {
		r := <-unorderedResults
		for len(results) <= r.Round {
			results = append(results, RoundResult{Round: -1})
		}
		results[r.Round] = r

		for applyIndex < len(results) && results[applyIndex].Round != -1 {
			orderedResults <- results[applyIndex]
			applyIndex++
		}
	}
}

func equalContents(s1 []string, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	s1c := make([]string, len(s1))
	copy(s1c, s1)
	s2c := make([]string, len(s2))
	copy(s2c, s2)
	sort.Strings(s1c)
	sort.Strings(s2c)

	for i, c1 := range s1c {
		if c1 != s2c[i] {
			return false
		}
	}
	return true
}

func TestServerClientSingleMachineNoFailures(t *testing.T) {
	applyCh := make(chan raft.ApplyMsg)
	rf := mockraft.New(applyCh)
	s := NewServer(rf)

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

	c1 := NewClient(s, mg1, end1)
	results := c1.GetResCh()
	orderedResults := make(chan RoundResult)
	go resultOrderer(results, orderedResults)
	c2 := NewClient(s, mg2, end2)
	c3 := NewClient(s, mg3, end3)

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
		var expectedMessages []string
		if i%2 == 0 {
			expectedMessages = []string{mg1.Message(r.Round)}
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
			expectedMessages = []string{mg1.Message(r.Round), mg2.Message(r.Round), mg3.Message(r.Round)}
		}
		if !equalContents(expectedMessages, r.Messages) {
			t.Fatalf("Wrong messages for this round. Expected %v, got %v", expectedMessages, r.Messages)
		}
	}

	c1.Kill()
	c2.Kill()
	c3.Kill()

	time.Sleep(100 * time.Millisecond)

	s.Kill()
}
