package anonbcast

// Client performs the anonymous broadcasting protocol, and exposes the bare minimum of
// communication necessary for an application to broadcast anonymous messages. A Client
// interacts with a local Server instance to get information, and a possibly remote Server
// instance to write information.
type Client interface {
	// Start indicates the intent
	Start(round int)
}

type client struct {
}

func (c *client) Start(round int) {

}

func NewClient(s Server) Client {
	return &client{}
}
