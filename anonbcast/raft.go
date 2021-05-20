package anonbcast

import (
	"fmt"
	"github.com/arvid220u/eggscrambler/libraft"
	"github.com/arvid220u/eggscrambler/network"
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
		panic(fmt.Sprintf("need to specify raft.so location using %v environment variable! or if you want to run the non-libified version, uncomment the line below this line in anonbcast/raft.go", compiledRaftEnvKey))
		//return raft.Make(cp, me, initialConfig, persister, applyCh, sendAllLogAsInt)
		return nil
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
