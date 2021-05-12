package anonbcast

import (
	"time"

	"github.com/arvid220u/6.824-project/raft"
)

// Polls servers for configuration (active or provisional)
// until it gets a response from the Raft leader
// Then returns whether or not the client's underlying server is
// in the raft configuration.
func (c *Client) getMembership(isProvisionalReq bool) bool {
	args := InConfigurationArgs{Server: c.serverId, IsProvisionalReq: isProvisionalReq}
	reply := InConfigurationReply{}
	for !c.killed() {
		c.mu.Lock()
		ok := c.cp.Call(c.currConf[c.lastKnownLeaderInd], "Server.InConfiguration", &args, &reply)
		c.mu.Unlock()
		if !ok || !reply.IsLeader {
			c.updateLeader()
			continue
		}

		break
	}

	return reply.InConfiguration
}

// Polls the leader server to determine whether this server
// is in the configuration or not
func (c *Client) setActive() {
	member := c.getMembership(false)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = member
}

// AttemptProvisional makes RPC requests until it gets a successful response.
// If the server is already in the active configuration, sets status to active and returns true
// otherwise returns false, meaning the caller must check if the request has committed
func (c *Client) attemptProvisional() bool {
	args := AddProvisionalArgs{Server: c.serverId}
	reply := AddProvisionalReply{}
	for !c.killed() {
		c.mu.Lock()
		ok := c.cp.Call(c.currConf[c.lastKnownLeaderInd], "Server.AddProvisional", &args, &reply)
		c.mu.Unlock()
		if !ok || reply.Error == raft.AP_NOT_LEADER {
			c.updateLeader()
			continue
		}

		inActiveConfiguration := false
		if reply.Error == raft.AP_ALREADY_IN_CONFIGURATION {
			inActiveConfiguration = true
			c.mu.Lock()
			c.active = true
			c.mu.Unlock()
		}

		return inActiveConfiguration
	}

	// this value shouldn't matter, client is dead
	return false
}

func (c *Client) attemptAddRemove(isAdd bool) raft.AddRemoveServerError {
	args := AddRemoveArgs{Server: c.serverId, IsAdd: isAdd}
	reply := AddRemoveReply{}
	for !c.killed() {
		c.mu.Lock()
		ok := c.cp.Call(c.currConf[c.lastKnownLeaderInd], "Server.AddRemove", &args, &reply)
		c.mu.Unlock()
		if !ok || reply.Error == raft.AR_NOT_LEADER {
			c.updateLeader()
			continue
		} else if reply.Error == raft.AR_NEW_LEADER || reply.Error == raft.AR_CONCURRENT_CHANGE {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		break
	}

	return reply.Error
}

// Makes the appropriate requests to become provisional.
// Returns once server is confirmed to be provisional or
// is determined to already be in the configuration
func (c *Client) addProvisional() {
	for !c.killed() {
		if inActiveConfiguration := c.attemptProvisional(); inActiveConfiguration {
			return
		}

		if c.checkMembership(1*time.Second, true) {
			return
		}
	}
}

func (c *Client) checkMembership(timeout time.Duration, isProvisionalReq bool) bool {
	startTime := time.Now()
	for !c.killed() && (time.Since(startTime) < timeout) {
		if c.getMembership(isProvisionalReq) {
			return true
		}
	}

	return false
}

// Attempts an AddServer RPC until it confirms the leader has updated
// the configuration correspondingly.
// This doesn't mean that the leader has committed the entry, however.
func (c *Client) addSelf() {
	for !c.killed() {
		err := c.attemptAddRemove(true)

		if err == raft.AR_NOT_PROVISIONAL {
			panic("(Client) Add without becoming provisional shouldn't occur.")
		}

		c.mu.Lock()
		tempActive := c.active
		c.mu.Unlock()
		if tempActive {
			return
		}

		if c.checkMembership(1*time.Second, false) {
			return
		}
	}
}

// Attempts a RemoveServer RPC until it confirms the leader has updated
// the configuration correspondingly.
// This doesn't mean that the leader has committed the entry, however.
func (c *Client) removeSelf() {
	for !c.killed() {
		err := c.attemptAddRemove(false)

		if err == raft.AR_NOT_PROVISIONAL {
			panic("(Client) Shouldn't get provisional err on remove.")
		}

		c.mu.Lock()
		tempActive := c.active
		c.mu.Unlock()
		if !tempActive {
			return
		}

		if !c.checkMembership(1*time.Second, false) {
			return
		}
	}
}

// Handles joining the raft configuration.
// Submits the raft configuration change.
// Blocks return until it confirms the client state is active
// or the client is dead.
func (c *Client) BecomeActive() {
	c.addProvisional()
	for !c.killed() {
		c.addSelf()
		startTime := time.Now()
		timeout := time.Second * 2
		for time.Since(startTime) < timeout {
			var tempActive bool
			c.mu.Lock()
			// this is set when client receives configuration update
			tempActive = c.active
			c.mu.Unlock()

			if tempActive {
				return
			}

			time.Sleep(50 * time.Millisecond)
		}
	}
}

// Handles leaving the raft configuration.
// Prevents the client from joining any further rounds.
// Submits the raft configuration change.
// Blocks return until it confirms the client state is inactive
// or the client is dead.
func (c *Client) BecomeInactive() {
	c.mu.Lock()
	// We won't join any new rounds once this is set
	c.leaving = true
	c.mu.Unlock()

	for !c.killed() {
		c.removeSelf()
		startTime := time.Now()
		timeout := time.Second * 2
		for time.Since(startTime) < timeout {
			var tempActive bool
			c.mu.Lock()
			// this is set when client receives configuration update
			tempActive = c.active
			c.mu.Unlock()

			if !tempActive {
				return
			}

			time.Sleep(50 * time.Millisecond)
		}
	}
}
