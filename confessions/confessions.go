package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/arvid220u/eggscrambler/anonbcast"
	"github.com/arvid220u/eggscrambler/libraft"
	"github.com/arvid220u/eggscrambler/network"
	"log"
	"os"
	"strings"
	"time"
)

type confessionsGenerator struct{}

func (cg confessionsGenerator) Message(round int) []byte {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Message to send in round %d: ", round)
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

func main() {
	var join string // TODO: make into list for nicer fault tolerance
	flag.StringVar(&join, "join", "", "address of server to join")
	flag.Parse()

	cp := network.NewLibp2p()

	seedConf := make(map[string]bool)

	if join == "" {
		fmt.Printf("Starting seed server with ID: %v\n\n", cp.Me())
		fmt.Printf("Tell everyone else to run:\ngo run confessions.go -join %v\n\n", cp.Me())
		seedConf[cp.Me()] = true
	} else {
		fmt.Printf("Joining seed server with ID: %v\n", join)
		seedConf[join] = true
	}

	persister := libraft.MakePersister()
	s, rf := anonbcast.MakeServer(cp, seedConf, persister, -1)
	defer s.Kill()
	err := cp.Server.Register(s)
	if err != nil {
		panic(err)
	}
	err = cp.Server.Register(rf)
	if err != nil {
		panic(err)
	}

	cg := confessionsGenerator{}
	clcf := anonbcast.ClientConfig{
		MessageTimeout:  time.Second * 30,
		ProtocolTimeout: time.Second * 10,
		MessageSize:     100,
	}
	c := anonbcast.NewClient(s, cg, cp, seedConf, clcf)
	defer c.Kill()
	results := c.CreateResCh()
	defer c.DestroyResCh(results)
	orderedResults := make(chan anonbcast.RoundResult)
	go resultOrderer(results, orderedResults)

	for i := 0; ; i++ {
		sm := c.GetLastStateMachine()
		if sm.Round > i {
			continue
		}
		fmt.Printf("Waiting for 60 seconds before deciding to start round %v...\n", i)
		var r anonbcast.RoundResult
		select {
		case <-time.NewTimer(time.Second * 60).C:
			fmt.Printf("Starting round %d!\n", i)
			err := c.Start(i)
			if err != nil {
				sm := c.GetLastStateMachine()
				if sm.Round > i {
					continue
				}
				i-- // we don't want to advance if the latest state machine is not ahead of us, we just want to retry
				continue
			}
			r = <-orderedResults
			fmt.Printf("Got round %d result!\n", i)
		case r = <-orderedResults:
			fmt.Printf("Got round %d result!\n", i)
		}
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
