package main

import (
	"fmt"
	"github.com/arvid220u/6.824-project/anonbcast"
	"github.com/arvid220u/6.824-project/labrpc"
	"github.com/arvid220u/6.824-project/mockraft"
	"github.com/arvid220u/6.824-project/raft"
	"time"
)

type ConfessionsGenerator int

func (cg ConfessionsGenerator) Message(round int) string {
	return fmt.Sprintf("this is a confession in round %d", round)
}

// TODO: extract the confessions app into its own package, and create a very simple
// 	main package that starts the confessions app.
func main() {
	println("welcome to the fully anonymous MIT confessions!")
	applyCh := make(chan raft.ApplyMsg)
	rf := mockraft.New(applyCh)
	s := anonbcast.NewServer(rf)
	var cg ConfessionsGenerator

	net := labrpc.MakeNetwork()
	defer net.Cleanup()
	end := net.MakeEnd("client")
	svc := labrpc.MakeService(s)
	srv := labrpc.MakeServer()
	srv.AddService(svc)
	net.AddServer("server", srv)
	net.Connect("client", "server")
	net.Enable("client", true)

	c := anonbcast.NewClient(s, cg, end)

	fmt.Printf("client: %v, server: %v\n", c, s)

	time.Sleep(time.Second * 1)

	c.Start(0)

	time.Sleep(time.Second * 5)

	// TODO: think more about the api between clients and users!
	//for i := 0; ; i++ {
	//	c.Start(i) // TODO: only start a round if it is actually the right round / etc
	//	reader := bufio.NewReader(os.Stdin)
	//	fmt.Printf("Message to send: ")
	//	msg, err := reader.ReadString('\n')
	//	if err != nil {
	//		fmt.Fprintln(os.Stderr, err)
	//		return
	//	}
	//	msg = strings.TrimSuffix(msg, "\n")
	//}
}
