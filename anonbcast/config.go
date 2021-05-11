package anonbcast

import (
	"os"
	"testing"

	// import "log"
	crand "crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arvid220u/6.824-project/labrpc"
	"github.com/arvid220u/6.824-project/network"
	"github.com/arvid220u/6.824-project/raft"
	"github.com/google/uuid"
)

func randstring(n int) string {
	b := make([]byte, 2*n)
	crand.Read(b)
	s := base64.URLEncoding.EncodeToString(b)
	return s[0:n]
}

func makeSeed() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := crand.Int(crand.Reader, max)
	x := bigx.Int64()
	return x
}

// Randomize server handles
func random_handles(kvh []*labrpc.ClientEnd) []*labrpc.ClientEnd {
	sa := make([]*labrpc.ClientEnd, len(kvh))
	copy(sa, kvh)
	for i := range sa {
		j := rand.Intn(i + 1)
		sa[i], sa[j] = sa[j], sa[i]
	}
	return sa
}

type resultOrderer struct {
	dead int32
}

type config struct {
	mu           sync.Mutex
	t            *testing.T
	net          *labrpc.Network
	n            int
	servers      []*Server
	saved        []*raft.Persister
	endnames     [][]string // names of each server's sending ClientEnds
	clients      map[*Client][]string
	maxraftstate int
	start        time.Time // time at which make_config() was called
	// begin()/end() statistics
	t0    time.Time // time at which test_test.go called cfg.begin()
	rpcs0 int       // rpcTotal() at start of test
	ops   int32     // number of clerk get/put/append method calls

	orderedClientIds []uuid.UUID //client ids, where index corresponds to client's server
	orderedMessagers []Messager  // messager, where index corresponds to messager's client's server
}

func (ro *resultOrderer) kill() {
	atomic.StoreInt32(&ro.dead, 1)
}

func (ro *resultOrderer) killed() bool {
	z := atomic.LoadInt32(&ro.dead)
	return z == 1
}

func (ro *resultOrderer) order(unorderedResults <-chan RoundResult, orderedResults chan<- RoundResult) {
	var results []RoundResult
	applyIndex := 0
	for !ro.killed() {
		select {
		case r := <-unorderedResults:
			for len(results) <= r.Round {
				results = append(results, RoundResult{Round: -1})
			}
			results[r.Round] = r

			for applyIndex < len(results) && results[applyIndex].Round != -1 {
				orderedResults <- results[applyIndex]
				applyIndex++
			}
		case <-time.After(5 * time.Millisecond):
		}

	}
}

func (cfg *config) checkTimeout() {
	// enforce a two minute real-time limit on each test
	if !cfg.t.Failed() && time.Since(cfg.start) > 120*time.Second {
		cfg.t.Fatal("test took longer than 120 seconds")
	}
}

func (cfg *config) cleanup() {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	for i := 0; i < len(cfg.servers); i++ {
		if cfg.servers[i] != nil {
			cfg.servers[i].Kill()
		}
	}

	for cl := range cfg.clients {
		cl.Kill()
	}

	cfg.net.Cleanup()
	cfg.checkTimeout()
}

// Maximum log size across all servers
func (cfg *config) LogSize() int {
	logsize := 0
	for i := 0; i < cfg.n; i++ {
		n := cfg.saved[i].RaftStateSize()
		if n > logsize {
			logsize = n
		}
	}
	return logsize
}

// Maximum snapshot size across all servers
func (cfg *config) SnapshotSize() int {
	snapshotsize := 0
	for i := 0; i < cfg.n; i++ {
		n := cfg.saved[i].SnapshotSize()
		if n > snapshotsize {
			snapshotsize = n
		}
	}
	return snapshotsize
}

// attach server i to servers listed in to
// caller must hold cfg.mu
func (cfg *config) connectUnlocked(i int, to []int) {
	// log.Printf("connect peer %d to %v\n", i, to)

	// outgoing socket files
	for j := 0; j < len(to); j++ {
		endname := cfg.endnames[i][to[j]]
		cfg.net.Enable(endname, true)
	}

	// incoming socket files
	for j := 0; j < len(to); j++ {
		endname := cfg.endnames[to[j]][i]
		cfg.net.Enable(endname, true)
	}
}

func (cfg *config) connect(i int, to []int) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.connectUnlocked(i, to)
}

// detach server i from the servers listed in from
// caller must hold cfg.mu
func (cfg *config) disconnectUnlocked(i int, from []int) {
	// log.Printf("disconnect peer %d from %v\n", i, from)

	// outgoing socket files
	for j := 0; j < len(from); j++ {
		if cfg.endnames[i] != nil {
			endname := cfg.endnames[i][from[j]]
			cfg.net.Enable(endname, false)
		}
	}

	// incoming socket files
	for j := 0; j < len(from); j++ {
		if cfg.endnames[j] != nil {
			endname := cfg.endnames[from[j]][i]
			cfg.net.Enable(endname, false)
		}
	}
}

func (cfg *config) disconnect(i int, from []int) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.disconnectUnlocked(i, from)
}

func (cfg *config) All() []int {
	all := make([]int, cfg.n)
	for i := 0; i < cfg.n; i++ {
		all[i] = i
	}
	return all
}

func (cfg *config) ConnectAll() {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	for i := 0; i < cfg.n; i++ {
		cfg.connectUnlocked(i, cfg.All())
	}
}

// Sets up 2 partitions with connectivity between servers in each  partition.
func (cfg *config) partition(p1 []int, p2 []int) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	// log.Printf("partition servers into: %v %v\n", p1, p2)
	for i := 0; i < len(p1); i++ {
		cfg.disconnectUnlocked(p1[i], p2)
		cfg.connectUnlocked(p1[i], p1)
	}
	for i := 0; i < len(p2); i++ {
		cfg.disconnectUnlocked(p2[i], p1)
		cfg.connectUnlocked(p2[i], p2)
	}
}

type messageGenerator struct {
	id string
	mu *sync.Mutex
}

func (mg messageGenerator) Message(round int) []byte {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	return []byte(fmt.Sprintf("message in round %d from %s", round, mg.id))
}

// Create a clerk with clerk specific server names.
// Give it connections to all of the servers, but for
// now enable only connections to servers in to[].
// Client is assumed to be running on the same machine as localSrv
func (cfg *config) makeClient(m Messager, localSrv int, to []int) *Client {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	// a fresh set of ClientEnds.
	ends := make([]*labrpc.ClientEnd, cfg.n)
	endnames := make([]string, cfg.n)
	for j := 0; j < cfg.n; j++ {
		endnames[j] = randstring(20)
		ends[j] = cfg.net.MakeEnd(endnames[j])
		cfg.net.Connect(endnames[j], j)
	}

	cp := network.New(random_handles(ends))
	cl := NewClient(cfg.servers[localSrv], m, cp)
	cfg.clients[cl] = endnames
	cfg.ConnectClientUnlocked(cl, to)
	return cl
}

func (cfg *config) deleteClient(cl *Client) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	v := cfg.clients[cl]
	for i := 0; i < len(v); i++ {
		os.Remove(v[i])
	}
	delete(cfg.clients, cl)
}

// caller should hold cfg.mu
func (cfg *config) ConnectClientUnlocked(cl *Client, to []int) {
	// log.Printf("ConnectClient %v to %v\n", cl, to)
	endnames := cfg.clients[cl]
	for j := 0; j < len(to); j++ {
		s := endnames[to[j]]
		cfg.net.Enable(s, true)
	}
}

func (cfg *config) ConnectClient(cl *Client, to []int) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.ConnectClientUnlocked(cl, to)
}

// caller should hold cfg.mu
func (cfg *config) DisconnectClientUnlocked(cl *Client, from []int) {
	// log.Printf("DisconnectClient %v from %v\n", cl, from)
	endnames := cfg.clients[cl]
	for j := 0; j < len(from); j++ {
		s := endnames[from[j]]
		cfg.net.Enable(s, false)
	}
}

func (cfg *config) DisconnectClient(cl *Client, from []int) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.DisconnectClientUnlocked(cl, from)
}

// Shutdown a server by isolating it
func (cfg *config) ShutdownServer(i int) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	cfg.disconnectUnlocked(i, cfg.All())

	// disable client connections to the server.
	// it's important to do this before creating
	// the new Persister in saved[i], to avoid
	// the possibility of the server returning a
	// positive reply to an Append but persisting
	// the result in the superseded Persister.
	cfg.net.DeleteServer(i)

	// a fresh persister, in case old instance
	// continues to update the Persister.
	// but copy old persister's content so that we always
	// pass Make() the last persisted state.
	if cfg.saved[i] != nil {
		cfg.saved[i] = cfg.saved[i].Copy()
	}

	kv := cfg.servers[i]
	if kv != nil {
		cfg.mu.Unlock()
		kv.Kill()
		cfg.mu.Lock()
		cfg.servers[i] = nil
	}
}

// If restart servers, first call ShutdownServer
func (cfg *config) StartServer(i int) {
	cfg.mu.Lock()

	// TODO change to make this an argument
	initialCfg := make(map[int]bool)
	// a fresh set of outgoing ClientEnd names.
	cfg.endnames[i] = make([]string, cfg.n)
	for j := 0; j < cfg.n; j++ {
		initialCfg[j] = true
		cfg.endnames[i][j] = randstring(20)
	}

	// a fresh set of ClientEnds.
	ends := make([]*labrpc.ClientEnd, cfg.n)
	for j := 0; j < cfg.n; j++ {
		ends[j] = cfg.net.MakeEnd(cfg.endnames[i][j])
		cfg.net.Connect(cfg.endnames[i][j], j)
	}

	// a fresh persister, so old instance doesn't overwrite
	// new instance's persisted state.
	// give the fresh persister a copy of the old persister's
	// state, so that the spec is that we pass StartKVServer()
	// the last persisted state.
	if cfg.saved[i] != nil {
		cfg.saved[i] = cfg.saved[i].Copy()
	} else {
		cfg.saved[i] = raft.MakePersister()
	}
	cfg.mu.Unlock()

	cp := network.New(ends)
	cfg.servers[i] = MakeServer(cp, i, initialCfg, cfg.saved[i], cfg.maxraftstate)

	anonsvc := labrpc.MakeService(cfg.servers[i])
	rfsvc := labrpc.MakeService(cfg.servers[i].rf)
	srv := labrpc.MakeServer()
	srv.AddService(anonsvc)
	srv.AddService(rfsvc)
	cfg.net.AddServer(i, srv)

	var mu sync.Mutex
	mg := messageGenerator{
		id: fmt.Sprint(i),
		mu: &mu,
	}
	cfg.orderedMessagers[i] = &mg

	cl := cfg.makeClient(mg, i, cfg.All())
	cfg.orderedClientIds[i] = cl.Id

}

func (cfg *config) Leader() (bool, int) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	for i := 0; i < cfg.n; i++ {
		_, is_leader := cfg.servers[i].rf.GetState()
		if is_leader {
			return true, i
		}
	}
	return false, 0
}

// Partition servers into 2 groups and put current leader in minority
func (cfg *config) make_partition() ([]int, []int) {
	_, l := cfg.Leader()
	p1 := make([]int, cfg.n/2+1)
	p2 := make([]int, cfg.n/2)
	j := 0
	for i := 0; i < cfg.n; i++ {
		if i != l {
			if j < len(p1) {
				p1[j] = i
			} else {
				p2[j-len(p1)] = i
			}
			j++
		}
	}
	p2[len(p2)-1] = l
	return p1, p2
}

var ncpu_once sync.Once

func make_config(t *testing.T, n int, unreliable bool, maxraftstate int) *config {
	ncpu_once.Do(func() {
		if runtime.NumCPU() < 2 {
			fmt.Printf("warning: only one CPU, which may conceal locking bugs\n")
		}
		rand.Seed(makeSeed())
	})
	runtime.GOMAXPROCS(4)
	cfg := &config{}
	cfg.t = t
	cfg.net = labrpc.MakeNetwork()
	cfg.n = n
	cfg.servers = make([]*Server, cfg.n)
	cfg.saved = make([]*raft.Persister, cfg.n)
	cfg.endnames = make([][]string, cfg.n)
	cfg.clients = make(map[*Client][]string)
	cfg.maxraftstate = maxraftstate
	cfg.start = time.Now()
	cfg.orderedClientIds = make([]uuid.UUID, cfg.n)
	cfg.orderedMessagers = make([]Messager, cfg.n)

	// create a full set of KV servers.
	for i := 0; i < cfg.n; i++ {
		cfg.StartServer(i)
	}

	cfg.ConnectAll()

	cfg.net.Reliable(!unreliable)

	return cfg
}

func (cfg *config) rpcTotal() int {
	return cfg.net.GetTotalCount()
}

// start a Test.
// print the Test message.
// e.g. cfg.begin("Test (2B): RPC counts aren't too high")
func (cfg *config) begin(description string) {
	fmt.Printf("%s ...\n", description)
	cfg.t0 = time.Now()
	cfg.rpcs0 = cfg.rpcTotal()
	atomic.StoreInt32(&cfg.ops, 0)
}

func (cfg *config) op() {
	atomic.AddInt32(&cfg.ops, 1)
}

// end a Test -- the fact that we got here means there
// was no failure.
// print the Passed message,
// and some performance numbers.
func (cfg *config) end() {
	cfg.checkTimeout()
	if cfg.t.Failed() == false {
		t := time.Since(cfg.t0).Seconds()  // real time
		npeers := cfg.n                    // number of Raft peers
		nrpc := cfg.rpcTotal() - cfg.rpcs0 // number of RPC sends
		ops := atomic.LoadInt32(&cfg.ops)  //  number of clerk get/put/append calls

		fmt.Printf("  ... Passed --")
		fmt.Printf("  %4.1f  %d %5d %4d\n", t, npeers, nrpc, ops)
	}
}

// Assuming at least 1 client has been created using the default generator,
// completes one broadcast round
// Returns the current round at the end of the process.
func (cfg *config) oneRound(round int) int {
	c1Id := cfg.orderedClientIds[0]
	mg1 := cfg.orderedMessagers[0]

	var c1 *Client
	for cl := range cfg.clients {
		if cl.Id == c1Id {
			c1 = cl
			break
		}
	}

	results := c1.GetResCh()
	orderedResults := make(chan RoundResult)
	ro := &resultOrderer{}
	defer ro.kill()
	go ro.order(results, orderedResults)
	for i := 0; i < 2; i++ {
		actualRound := round + i
		fmt.Printf("XXX Starting round %d\n", actualRound) // TODO remove

		if i%2 == 0 {
			// race to the start
			err := c1.Start(actualRound)
			for err != nil {
				time.Sleep(time.Millisecond * 1)
				err = c1.Start(actualRound)
			}

			fmt.Printf("XXX Successful Start 1\n")
		} else {
			// wait for everyone to submit :)
			sm1 := c1.GetLastStateMachine()
			for sm1.Round != actualRound || len(sm1.CurrentRoundInfo().Participants) < cfg.n {
				fmt.Printf("XXX 1 Waiting for actual round %d: %d or participants: %d\n", actualRound, sm1.Round, sm1.CurrentRoundInfo().Participants)
				time.Sleep(time.Millisecond * 5)
				sm1 = c1.GetLastStateMachine()
			}
			err := c1.Start(actualRound)
			if err != nil {
				cfg.t.Fatalf("problem occurred in start should never happen: %v", err)
			}
			fmt.Printf("XXX Successful Start 2\n")
		}
		r := <-orderedResults

		fmt.Printf("XXX Got results\n")

		if r.Round != actualRound {
			cfg.t.Fatalf("Round %d not equal to round %d", r.Round, actualRound)
		}
		if !r.Succeeded {
			cfg.t.Fatalf("Round %d failed which should not happen", r.Round)
		}
		var expectedMessages [][]byte
		if i%2 == 0 {
			expectedMessages = [][]byte{mg1.Message(r.Round)}
			sm1 := c1.GetLastStateMachine()
			for sm1.Round != r.Round+1 {
				fmt.Printf("XXX 1 Waiting for actual round %d: %d\n", r.Round+1, sm1.Round)
				time.Sleep(time.Millisecond * 5)
				sm1 = c1.GetLastStateMachine()
			}
			ri, err := sm1.GetRoundInfo(r.Round)
			if err != nil {
				cfg.t.Fatalf("error getting round info: %v", err)
			}
			for _, p := range ri.Participants {
				for j, id := range cfg.orderedClientIds {
					if p == id && id != c1Id {
						mgj := cfg.orderedMessagers[j]
						expectedMessages = append(expectedMessages, mgj.Message(r.Round))
					}
				}
			}
		} else {
			expectedMessages = make([][]byte, 0)
			for _, mgj := range cfg.orderedMessagers {
				expectedMessages = append(expectedMessages, mgj.Message(r.Round))
			}
		}
		if !equalContentsBytes(expectedMessages, r.Messages) {
			cfg.t.Fatalf("Wrong messages for this round. Expected %v, got %v", expectedMessages, r.Messages)
		}
	}
	return round + 1
}

func equalContentsBytes(b1 [][]byte, b2 [][]byte) bool {
	var s1 []string
	for _, b := range b1 {
		s1 = append(s1, string(b))
	}
	var s2 []string
	for _, b := range b2 {
		s2 = append(s2, string(b))
	}
	return equalContents(s1, s2)
}

func equalContents(s1 []string, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	s1c := make([]string, len(s1))
	copy(s1c, s1)
	s2c := make([]string, len(s2))
	copy(s2c, s2)
	sort.Strings(s1c)
	sort.Strings(s2c)

	for i, c1 := range s1c {
		if c1 != s2c[i] {
			return false
		}
	}
	return true
}
