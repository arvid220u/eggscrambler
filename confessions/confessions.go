package main

import (
	"bufio"
	"fmt"
	"github.com/arvid220u/6.824-project/anonbcast"
	"github.com/arvid220u/6.824-project/labrpc"
	"github.com/arvid220u/6.824-project/mockraft"
	"github.com/arvid220u/6.824-project/raft"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type ConfessionsGenerator struct {
	id string
	mu *sync.Mutex
}

func (cg ConfessionsGenerator) Message(round int) string {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Message to send in round %d, client %s: ", round, cg.id)
	msg, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	msg = strings.TrimSuffix(msg, "\n")
	return msg
}

// TODO: extract the confessions app into its own package, and create a very simple
// 	main package that starts the confessions app.
func main() {
	fmt.Println("welcome to the fully anonymous MIT confessions!")
	applyCh := make(chan raft.ApplyMsg)
	rf := mockraft.New(applyCh)
	s := anonbcast.NewServer(rf)

	net := labrpc.MakeNetwork()
	defer net.Cleanup()
	end := net.MakeEnd("client")
	svc := labrpc.MakeService(s)
	srv := labrpc.MakeServer()
	srv.AddService(svc)
	net.AddServer("server", srv)
	net.Connect("client", "server")
	net.Enable("client", true)

	results := make(chan anonbcast.RoundResult)

	var mu sync.Mutex
	cg1 := ConfessionsGenerator{
		id: "1",
		mu: &mu,
	}
	cg2 := ConfessionsGenerator{
		id: "2",
		mu: &mu,
	}

	c1 := anonbcast.NewClient(s, cg1, end, results)
	c2 := anonbcast.NewClient(s, cg2, end, make(chan anonbcast.RoundResult))

	log.Printf("server: %+v, client 1: %+v, client 2: %+v\n", s, c1, c2)

	for i := 0; ; i++ {
		time.Sleep(time.Millisecond * 500)
		fmt.Printf("Starting round %d!\n", i)
		c1.Start(i)
		for {
			r := <-results
			if r.Round != i { // TODO: this assumes the results are sent in order, which they aren't
				continue
			}
			if r.Succeeded {
				fmt.Printf("Round %d succeeded! Anonymized broadcast messages are:\n", r.Round)
				for _, m := range r.Messages {
					fmt.Printf("\t%s\n", m)
				}
			} else {
				fmt.Printf("Round %d failed :(\n", r.Round)
			}
			break
		}
	}
}
