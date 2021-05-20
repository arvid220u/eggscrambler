package anonbcast

import (
	"github.com/arvid220u/6.824-project/libraft"
	"time"
)

// Polls servers for configuration (active or provisional)
// until it gets a response from the Raft leader
// Then returns whether or not the client's underlying server is
// in the raft configuration.
func (c *Client) getMembership(isProvisionalReq bool) bool {
	args := InConfigurationArgs{Server: c.serverId, IsProvisionalReq: isProvisionalReq}
	inConfiguration := false
	for !c.killed() {
		reply := InConfigurationReply{}
		c.mu.Lock()
		potentialLeader := c.currConf[c.lastKnownLeaderInd]
		c.mu.Unlock()
		ok := c.cp.Call(potentialLeader, "Server.InConfiguration", &args, &reply)
		inConfiguration = reply.InConfiguration
		if !ok || !reply.IsLeader {
			c.updateLeader()
			time.Sleep(5 * time.Millisecond)
			continue
		}
		break
	}

	return inConfiguration
}

// Polls the leader server to determine whether this server
// is in the configuration or not
func (c *Client) setActive() {
	member := c.getMembership(false)
	c.mu.Lock()
	c.active = member
	c.mu.Unlock()
}

// AttemptProvisional makes RPC requests until it gets a successful response.
// If the server is already in the active configuration, sets status to active and returns true
// otherwise returns false, meaning the caller must check if the request has committed
func (c *Client) attemptProvisional() bool {
	args := AddProvisionalArgs{Server: c.serverId}
	for !c.killed() {
		reply := AddProvisionalReply{}
		c.mu.Lock()
		potentialLeader := c.currConf[c.lastKnownLeaderInd]
		c.mu.Unlock()
		ok := c.cp.Call(potentialLeader, "Server.AddProvisional", &args, &reply)
		if !ok || reply.Error == libraft.AP_NOT_LEADER {
			c.updateLeader()
			continue
		}

		inActiveConfiguration := false
		if reply.Error == libraft.AP_ALREADY_IN_CONFIGURATION {
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

func (c *Client) attemptAddRemove(isAdd bool) libraft.AddRemoveServerError {
	args := AddRemoveArgs{Server: c.serverId, IsAdd: isAdd}
	var arError libraft.AddRemoveServerError
	for !c.killed() {
		reply := AddRemoveReply{}
		c.mu.Lock()
		potentialLeader := c.currConf[c.lastKnownLeaderInd]
		c.mu.Unlock()
		ok := c.cp.Call(potentialLeader, "Server.AddRemove", &args, &reply)
		arError = reply.Error
		if !ok || reply.Error == libraft.AR_NOT_LEADER {
			c.updateLeader()
			continue
		} else if reply.Error == libraft.AR_NEW_LEADER || reply.Error == libraft.AR_CONCURRENT_CHANGE {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		break
	}

	return arError
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

		if err == libraft.AR_NOT_PROVISIONAL {
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

		if err == libraft.AR_NOT_PROVISIONAL {
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
