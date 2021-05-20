package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/arvid220u/eggscrambler/anonbcast"
	"github.com/arvid220u/eggscrambler/libraft"
	"github.com/arvid220u/eggscrambler/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"
)

type confessionsGenerator struct {
	input   chan string
	timeout time.Duration
}

func (cg confessionsGenerator) Message(round int) []byte {
	// if someone else is waiting on input, frontrun them
	select {
	case cg.input <- "poison":
	default:
	}
	fmt.Printf("Message to send in round %d: ", round)
	var msg string
	done := false
	for i := 0; !done; i++ {
		select {
		case msg = <-cg.input:
			if msg == "poison" {
				return nil
			}
			fmt.Printf("Waiting for everyone to submit...\n")
			done = true
		case <-time.NewTimer(cg.timeout).C:
			fmt.Printf("too late!\n")
			done = true
		}
	}
	msg = strings.TrimSuffix(msg, "\n")
	return []byte(msg)
}

func readInput(input chan string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("err: %v", err)
		}
		input <- msg
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
		ma, err := multiaddr.NewMultiaddr(join)
		if err != nil {
			panic(err)
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			panic(err)
		}
		ctx := context.Background()
		err = cp.Host.Connect(ctx, *peerInfo)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Joining seed server with ID: %v\n", join)
		fmt.Printf("(My address: %v)\n", cp.Me())
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

	input := make(chan string)
	cg := confessionsGenerator{
		input:   input,
		timeout: time.Second * 20,
	}
	go readInput(input)
	clcf := anonbcast.ClientConfig{
		MessageTimeout:  time.Second * 30,
		ProtocolTimeout: time.Second * 10,
		MessageSize:     100,
	}
	c := anonbcast.NewClient(s, cg, cp, seedConf, clcf)
	defer c.Kill()
	results := c.CreateResCh()
	defer c.DestroyResCh(results)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	defer signal.Stop(signalChan)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	stop := false
	for i := 0; !stop; i++ {
		sm := c.GetLastStateMachine()
		if sm.Round > i {
			continue
		}
		me := false
		for _, p := range sm.CurrentRoundInfo().Participants {
			if p == c.Id {
				me = true
			}
		}
		for !me {
			select {
			case <-time.NewTimer(time.Second).C:
			case <-signalChan:
				fmt.Printf("Quitting gracefully...\n")
				stop = true
				break
			}
			if stop {
				break
			}
			fmt.Printf("Waiting for round %d to progress...\n", sm.Round)
			sm = c.GetLastStateMachine()
			if sm.Round > i {
				break
			}
			me = false
			for _, p := range sm.CurrentRoundInfo().Participants {
				if p == c.Id {
					me = true
				}
			}
		}
		if stop {
			continue
		}
		if sm.Round > i {
			continue
		}
		fmt.Printf("Press enter to start round %v...\n", i)
		var r anonbcast.RoundResult
		select {
		case inputMsg := <-input:
			if inputMsg != "poison" {
				fmt.Printf("Starting round %d!\n", i)
			}
			err := c.Start(i)
			if err != nil {
				sm := c.GetLastStateMachine()
				if sm.Round > i {
					continue
				}
				// TODO: abort this round! it may have gotten stuck for various reasons
				i--
				continue
			}
			doneSelect := false
			for !doneSelect {
				select {
				case r = <-results:
					doneSelect = true
				case <-signalChan:
					fmt.Printf("Quitting gracefully...\n")
					stop = true
					doneSelect = true
					break
				case <-ticker.C:
					sm := c.GetLastStateMachine()
					if sm.Round > i {
						doneSelect = true
						break
					}
				}
			}
			if stop {
				break
			}
			fmt.Printf("Got round %d result!\n", i)
		case r = <-results:
			fmt.Printf("Got round %d result!\n", i)
		case <-signalChan:
			fmt.Printf("Quitting gracefully...\n")
			stop = true
			break
		}
		select {
		case cg.input <- "poison":
		default:
		}
		if stop {
			continue
		}
		if r.Round != i {
			log.Fatalf("round %d not equal to index %d", r.Round, i)
		}
		if r.Succeeded {
			fmt.Printf("\n\u001b[32mRound %d succeeded!\u001b[0m Anonymized broadcast messages are:\n\n", r.Round)
			for _, m := range r.Messages {
				fmt.Printf("\t\u001b[31m%s\u001B[0m\n", string(m))
			}
			fmt.Printf("\n")
		} else {
			fmt.Printf("Round %d failed :(\n", r.Round)
		}
	}
}
