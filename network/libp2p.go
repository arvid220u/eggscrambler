package network

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/multiformats/go-multiaddr"
	"strings"
)

const rpcProtocolID = "/p2p/rpc/eggscrambler"

// Libp2pConnectionProvider implements the ConnectionProvider interface using libp2p
type Libp2pConnectionProvider struct {
	Host   host.Host
	Client *gorpc.Client
	Server *gorpc.Server
}

func (cp *Libp2pConnectionProvider) Call(server string, svcName string, svcMeth string, args interface{}, reply interface{}) bool {
	ma, err := multiaddr.NewMultiaddr(server)
	if err != nil {
		panic(err)
	}
	peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		panic(err)
	}

	if peerInfo.ID != cp.Host.ID() {
		// can we skip adding it every time????
		ctx := context.Background()
		err = cp.Host.Connect(ctx, *peerInfo)
		if err != nil {
			//log.Printf("Unsuccessfully connected to host (%s.%s): %v", svcName, svcMeth, err)
			return false
		}
	}

	err = cp.Client.Call(peerInfo.ID, svcName, svcMeth, args, reply)
	return err == nil
}

func createPeer(listenAddr string) host.Host {
	ctx := context.Background()

	// Create a new libp2p Host
	// TODO: we kind of don't need reliable transport, and we also don't need any security. but it's fineeee
	h, err := libp2p.New(ctx, libp2p.ListenAddrStrings(listenAddr))
	if err != nil {
		panic(err)
	}
	return h
}

func NewLibp2p() *Libp2pConnectionProvider {
	cp := &Libp2pConnectionProvider{}
	// TODO: listen on 0.0.0.0 so we can allow non-localhost connections too :)
	//cp.Host = createPeer("/ip4/0.0.0.0/tcp/0")
	cp.Host = createPeer("/ip4/0.0.0.0/tcp/0")
	fmt.Printf("Hello World, new libp2p with ID %s\n", cp.Host.ID().Pretty())
	cp.Server = gorpc.NewServer(cp.Host, rpcProtocolID)
	cp.Client = gorpc.NewClientWithServer(cp.Host, rpcProtocolID, cp.Server)

	return cp
}

func (cp *Libp2pConnectionProvider) NumPeers() int {
	panic("idk")
}

func (cp *Libp2pConnectionProvider) Me() string {
	pi := peer.AddrInfo{
		ID:    cp.Host.ID(),
		Addrs: cp.Host.Addrs(),
	}
	addrs, err := peer.AddrInfoToP2pAddrs(&pi)
	if err != nil {
		panic(err)
	}
	// choose the non-127.0.0.1 address
	var chosenAddr string
	for _, addr := range addrs {
		if !strings.Contains(addr.String(), "127.0.0.1") {
			chosenAddr = addr.String()
		}
	}
	return chosenAddr
}
