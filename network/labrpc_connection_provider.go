package network

import (
	"fmt"
	"github.com/arvid220u/eggscrambler/labrpc"
	"strconv"
)

// Implements the ConnectionProvider interface using the laprpc library
type LabrpcConnectionProvider struct {
	ends []*labrpc.ClientEnd
	me   int
}

func (cp *LabrpcConnectionProvider) Call(server string, svcName string, svcMeth string, args interface{}, reply interface{}) bool {
	serverInt, err := strconv.Atoi(server)
	if err != nil {
		panic(fmt.Sprintf("labrpc requires server IDs to be ints, but %v", err))
	}
	return cp.ends[serverInt].Call(fmt.Sprintf("%s.%s", svcName, svcMeth), args, reply)
}

func New(peers []*labrpc.ClientEnd, me int) ConnectionProvider {
	cp := &LabrpcConnectionProvider{}
	cp.me = me
	for _, v := range peers {
		cp.ends = append(cp.ends, v)
	}

	return cp
}

func (cp *LabrpcConnectionProvider) NumPeers() int {
	return len(cp.ends)
}

func (cp *LabrpcConnectionProvider) Me() string {
	return strconv.Itoa(cp.me)
}
