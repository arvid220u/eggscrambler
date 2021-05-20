package network

import "github.com/arvid220u/eggscrambler/labrpc"

// Implements the ConnectionProvider interface using the laprpc library
type LabrpcConnectionProvider struct {
	ends []*labrpc.ClientEnd
}

func (cp *LabrpcConnectionProvider) Call(server int, svcMeth string, args interface{}, reply interface{}) bool {
	return cp.ends[server].Call(svcMeth, args, reply)
}

func New(peers []*labrpc.ClientEnd) ConnectionProvider {
	cp := &LabrpcConnectionProvider{}
	for _, v := range peers {
		cp.ends = append(cp.ends, v)
	}

	return cp
}

func (cp *LabrpcConnectionProvider) NumPeers() int {
	return len(cp.ends)
}
