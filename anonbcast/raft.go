package anonbcast

import (
	"github.com/arvid220u/eggscrambler/libraft"
	"github.com/arvid220u/eggscrambler/network"
	"github.com/arvid220u/eggscrambler/raft"
	"log"
	"os"
	"plugin"
)

const compiledRaftEnvKey = "RAFT"

func makeRaft(cp network.ConnectionProvider, initialConfig map[string]bool,
	persister *libraft.Persister, applyCh chan libraft.ApplyMsg, sendAllLogAsInt bool) libraft.Raft {

	me := cp.Me()

	compiledRaft := os.Getenv(compiledRaftEnvKey)

	if compiledRaft == "" {
		// use the actual raft!
		//panic(fmt.Sprintf("need to specify raft.so location using %v environment variable! or if you want to run the non-libified version, uncomment the line below this line in anonbcast/raft.go", compiledRaftEnvKey))
		return raft.Make(cp, me, initialConfig, persister, applyCh, sendAllLogAsInt)
		return nil
	} else {
		// use the libified raft!
		p, err := plugin.Open(compiledRaft)
		if err != nil {
			log.Fatalf("cannot load raft plugin %v with error: %v", compiledRaft, err)
		}
		xmakef, err := p.Lookup("Make")
		if err != nil {
			log.Fatalf("cannot find Make in %v with error: %v", compiledRaft, err)
		}
		makef := xmakef.(func(network.ConnectionProvider, string, map[string]bool, *libraft.Persister, chan libraft.ApplyMsg, bool) libraft.Raft)
		return makef(cp, me, initialConfig, persister, applyCh, sendAllLogAsInt)
	}
}
