package network

/* Serves as an additional abstraction over labrpc/networking. Raft + the client
identify servers by indices. This interface is responsible for taking a call,
translating the index into either a 'labrpc' end or an actual network connection
and executing the rpc just as labrpc does. Using this class should let us maintain a
similar interface in our code but swap out whether we want to use labrpc or real
ip addresses */
type ConnectionProvider interface {
	Call(server int, svcMeth string, args interface{}, reply interface{}) bool
	NumPeers() int
}
