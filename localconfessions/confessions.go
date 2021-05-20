package main

import (
	"bufio"
	"fmt"
	"github.com/arvid220u/eggscrambler/libraft"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/arvid220u/eggscrambler/anonbcast"
	"github.com/arvid220u/eggscrambler/labrpc"
	"github.com/arvid220u/eggscrambler/mockraft"
	"github.com/arvid220u/eggscrambler/network"
)

type ConfessionsGenerator struct {
	id string
	mu *sync.Mutex
}

func (cg ConfessionsGenerator) Message(round int) []byte {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Message to send in round %d, client %s: ", round, cg.id)
	msg, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	msg = strings.TrimSuffix(msg, "\n")
	return []byte(msg)
}

func resultOrderer(unorderedResults <-chan anonbcast.RoundResult, orderedResults chan<- anonbcast.RoundResult) {
	var results []anonbcast.RoundResult
	applyIndex := 0
	for {
		r := <-unorderedResults
		for len(results) <= r.Round {
			results = append(results, anonbcast.RoundResult{Round: -1})
		}
		results[r.Round] = r

		for applyIndex < len(results) && results[applyIndex].Round != -1 {
			orderedResults <- results[applyIndex]
			applyIndex++
		}
	}
}

// TODO: extract the confessions app into its own package, and create a very simple
// 	main package that starts the confessions app.
func main() {
	fmt.Println("welcome to the fully anonymous MIT confessions!")
	applyCh := make(chan libraft.ApplyMsg)
	rf := mockraft.New(applyCh)
	s := anonbcast.NewServer("0", rf)
	defer s.Kill()

	net := labrpc.MakeNetwork()
	defer net.Cleanup()
	end1 := net.MakeEnd("client1")
	end2 := net.MakeEnd("client2")
	svc := labrpc.MakeService(s)
	srv := labrpc.MakeServer()
	srv.AddService(svc)
	net.AddServer("server", srv)
	net.Connect("client1", "server")
	net.Enable("client1", true)
	net.Connect("client2", "server")
	net.Enable("client2", true)

	cp1 := network.New([]*labrpc.ClientEnd{end1}, 0)
	cp2 := network.New([]*labrpc.ClientEnd{end2}, 0)

	var mu sync.Mutex
	cg1 := ConfessionsGenerator{
		id: "1",
		mu: &mu,
	}
	cg2 := ConfessionsGenerator{
		id: "2",
		mu: &mu,
	}

	clcf := anonbcast.ClientConfig{
		MessageTimeout:  time.Second * 30,
		ProtocolTimeout: time.Second * 10,
		MessageSize:     100,
	}
	seedConf := make(map[string]bool)
	seedConf["0"] = true
	c1 := anonbcast.NewClient(s, cg1, cp1, seedConf, clcf)
	defer c1.Kill()
	results := c1.CreateResCh()
	defer c1.DestroyResCh(results)
	orderedResults := make(chan anonbcast.RoundResult)
	go resultOrderer(results, orderedResults)
	c2 := anonbcast.NewClient(s, cg2, cp2, seedConf, clcf)
	defer c2.Kill()

	for i := 0; ; i++ {
		time.Sleep(time.Millisecond * 100)
		err := c1.Start(i)
		for err != nil {
			time.Sleep(time.Millisecond * 100)
			err = c1.Start(i)
		}
		fmt.Printf("Starting round %d!\n", i)
		r := <-orderedResults
		if r.Round != i {
			log.Fatalf("round %d not equal to index %d", r.Round, i)
		}
		if r.Succeeded {
			fmt.Printf("Round %d succeeded! Anonymized broadcast messages are:\n", r.Round)
			for _, m := range r.Messages {
				fmt.Printf("\t%s\n", string(m))
			}
		} else {
			fmt.Printf("Round %d failed :(\n", r.Round)
		}

	}

}
