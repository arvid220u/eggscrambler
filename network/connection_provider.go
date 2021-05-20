package network

/* Serves as an additional abstraction over labrpc/networking. Raft + the client
identify servers by indices. This interface is responsible for taking a call,
translating the index into either a 'labrpc' end or an actual network connection
and executing the rpc just as labrpc does. Using this class should let us maintain a
similar interface in our code but swap out whether we want to use labrpc or real
ip addresses */
type ConnectionProvider interface {
	// Call returns true if and only if the call was successful
	Call(server string, svcName string, svcMeth string, args interface{}, reply interface{}) bool
	// Me returns the address of this host
	Me() string
	NumPeers() int
}
