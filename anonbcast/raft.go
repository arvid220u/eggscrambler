package anonbcast

import (
	"github.com/arvid220u/6.824-project/libraft"
	"github.com/arvid220u/6.824-project/network"
	"github.com/arvid220u/6.824-project/raft"
	"log"
	"os"
	"plugin"
)

const compiledRaftEnvKey = "RAFT"

func makeRaft(cp network.ConnectionProvider, me int, initialConfig map[int]bool,
	persister *libraft.Persister, applyCh chan libraft.ApplyMsg, sendAllLogAsInt bool) libraft.Raft {

	compiledRaft := os.Getenv(compiledRaftEnvKey)

	if compiledRaft == "" {
		// use the actual raft!
		return raft.Make(cp, me, initialConfig, persister, applyCh, sendAllLogAsInt)
	} else {
		// use the libified raft!
		p, err := plugin.Open(compiledRaft)
		if err != nil {
			log.Fatalf("cannot load raft plugin %v", compiledRaft)
		}
		xmakef, err := p.Lookup("Make")
		if err != nil {
			log.Fatalf("cannot find Make in %v", compiledRaft)
		}
		makef := xmakef.(func(network.ConnectionProvider, int, map[int]bool, *libraft.Persister, chan libraft.ApplyMsg, bool) libraft.Raft)
		return makef(cp, me, initialConfig, persister, applyCh, sendAllLogAsInt)
	}
}
