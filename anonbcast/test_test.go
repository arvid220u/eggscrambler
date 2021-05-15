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

	updCh, i := s.GetUpdCh()

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

	s.CloseUpdCh(i)
	s.Kill()
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

	clcf := ClientConfig{
		MessageTimeout:  time.Second * 30,
		ProtocolTimeout: time.Second * 10,
		MessageSize:     100,
	}
	c1 := NewClient(s, mg1, cp1, clcf)
	results := c1.CreateResCh()
	defer c1.DestroyResCh(results)
	orderedResults := make(chan RoundResult)
	ro := &resultOrderer{}
	go ro.order(results, orderedResults, 0)
	c2 := NewClient(s, mg2, cp2, clcf)
	c3 := NewClient(s, mg3, cp3, clcf)

	for i := 0; i < 100; i++ {
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
	servers := 3
	cfg := make_config(t, servers, false, -1)
	defer cfg.cleanup()
	cfg.begin("Basic broadcast over reliable network")
	time.Sleep(3 * time.Second) // generous amount of time for leader election

	round := 0
	for i := 0; i < 10; i++ {
		round = cfg.oneRound(round) + 1
		time.Sleep(1 * time.Second) // probably could be much shorter
	}

	cfg.end()
}

func TestUnreliableNetBasic(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers, true, -1)
	defer cfg.cleanup()
	cfg.begin("Basic broadcast over unreliable network")
	time.Sleep(3 * time.Second) // generous amount of time for leader election

	round := 0
	for i := 0; i < 10; i++ {
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

	cfg := make_config(t, servers, false, -1)
	defer cfg.cleanup()
	cfg.begin("Checking all participants present when all servers in configuration")
	time.Sleep(3 * time.Second)

	cfg.checkConfigurationMatchesParticipants(expectedConfiguration)
}

func TestParticipantsOneNotInInitialConfiguration(t *testing.T) {
	servers := 4
	serversInitiallyInConfiguration := servers - 1
	expectedConfiguration := make(map[int]bool)
	initialConfiguration := make(map[int]bool)
	for i := 0; i < servers; i++ {
		if i < serversInitiallyInConfiguration {
			initialConfiguration[i] = true
		}
		expectedConfiguration[i] = true
	}

	cfg := make_config_with_initial_config(t, servers, false, -1, initialConfiguration)
	defer cfg.cleanup()
	cfg.begin("Check that client not in configuration adds self to configuration")
	time.Sleep(5 * time.Second) // generous amount of time for election and configuration change

	// The client out of configuration will miss the first round
	// TODO: it is possible for this to fail, if the configuration change was fast enough that the outsider
	//		client actually has time to become a participant in the first round.
	cfg.checkConfigurationMatchesParticipants(initialConfiguration)

	next := cfg.oneRoundWithExpectedConfiguration(0, initialConfiguration) + 1
	time.Sleep(1 * time.Second)
	// Initially excluded client should be in this round
	cfg.oneRoundWithExpectedConfiguration(next, expectedConfiguration)
}

func TestAbortSubmitTimeout(t *testing.T) {
	servers := 4
	indexToDisconnect := 0
	cfg := make_config(t, servers, false, -1)
	defer cfg.cleanup()
	cfg.begin("Abort timeout with disconnected client.")
	time.Sleep(5 * time.Second) // wait for everyone to become participant

	initialConfiguration := make(map[int]bool)
	liveConfiguration := make(map[int]bool)
	for i := 0; i < servers; i++ {
		if i != indexToDisconnect {
			liveConfiguration[i] = true
		}
		initialConfiguration[i] = true
	}

	// make sure all servers are in the round
	cfg.checkConfigurationMatchesParticipants(initialConfiguration)

	clIdToDisconnect := cfg.orderedClientIds[indexToDisconnect]
	cfg.DisconnectClientById(clIdToDisconnect, cfg.All())
	_, activeClient, _ := cfg.getActiveClient(liveConfiguration)
	activeClient.Start(0)
	time.Sleep(TEST_MESSAGE_TIMEOUT + TEST_PROTOCOL_TIMEOUT + time.Second) // submit timeout is message+protocol timeout, we add 1 second because goroutines might mess up timing
	fmt.Println("Wokeup")
	sm := activeClient.GetLastStateMachine()
	ri, err := sm.GetRoundInfo(0)
	if err != nil {
		t.Fatalf("Got error when requesting round info: %v", err)
	}

	if ri.Phase != FailedPhase {
		t.Fatalf("Expected round to fail but got phase %v", ri.Phase)
	}
}

func TestAbortClientKilled(t *testing.T) {
	servers := 4
	indexToKill := 0
	cfg := make_config(t, servers, false, -1)
	defer cfg.cleanup()
	cfg.begin("Abort caused by dead client.")
	time.Sleep(5 * time.Second) // wait for everyone to become participant

	initialConfiguration := make(map[int]bool)
	for i := 0; i < servers; i++ {
		initialConfiguration[i] = true
	}

	// make sure all servers are in the round
	cfg.checkConfigurationMatchesParticipants(initialConfiguration)

	clToKill := cfg.getClientById(cfg.orderedClientIds[indexToKill])
	activeClient := cfg.getClientById(cfg.orderedClientIds[indexToKill+1])
	activeClient.Start(0)
	// note: killing to send abort only works for reliable networks, because the client never retries sending the abort
	// for unreliable networks, we would want to wait for a protocol timeout instead.
	clToKill.Kill()
	time.Sleep(3 * time.Second) // wait for it to propagate
	sm := activeClient.GetLastStateMachine()
	ri, err := sm.GetRoundInfo(0)
	if err != nil {
		t.Fatalf("Got error when requesting round info: %v", err)
	}

	if ri.Phase != FailedPhase {
		t.Fatalf("Expected round to fail but got phase %v", ri.Phase)
	}
}
