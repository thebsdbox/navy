package navy

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
)

// NewCaptain returns a new `Captain` or an `error`.
//
// NOTE: All connections to `Peer`s are established during this function.
//
// NOTE: The `proto` value can be one of this list: `tcp`, `tcp4`, `tcp6`.
func NewCaptain(rank int, addr, proto, fleet, callsign string, ready bool, peers map[int]string) (*Captain, error) {
	c := &Captain{
		rank:         rank,
		addr:         addr,
		proto:        proto,
		Ready:        ready,
		fleet:        fleet,
		callsign:     callsign,
		peers:        NewPeerMap(),
		mu:           &sync.RWMutex{},
		electionChan: make(chan Message, 1),
		receiveChan:  make(chan Message),
		discoverChan: make(chan Message),
	}

	if err := c.Listen(proto, addr); err != nil {
		return nil, fmt.Errorf("new: %v", err)
	}
	if fleet != "" {
		err := c.Discover()
		if err != nil {
			log.Fatal(err)
		}
	}
	c.Connect(proto, peers)
	return c, nil
}

func (c *Captain) OnPromotion(promotion func()) {
	c.promoted = promotion
}

func (c *Captain) OnDemotion(demotion func()) {
	c.demoted = demotion
}

// NOTE: This function is thread-safe.
func (c *Captain) SetLeader(Addr string, rank int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If the new leader has a higher rank they become leader
	if rank > c.leaderRank {

		if c.leaderRank == c.rank {
			// If this is true then we're leading
			if rank > c.rank {
				if c.demoted != nil {
					c.demoted()
				}
			}
		}
		if rank == c.rank {
			if c.promoted != nil {
				c.promoted()
			}
		}
		c.leaderRank = rank
		c.leaderAddr = Addr
	}

}

func (c *Captain) ResetLeader(Addr string, rank int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.leaderRank = c.rank
	c.leaderAddr = c.addr

	for _, peer := range c.peers.PeerData() {
		log.Infof("%v", peer)
		if peer.Rank > c.leaderRank {
			c.leaderRank = peer.Rank
			c.leaderAddr = peer.Addr
		}

	}
	if c.rank == c.leaderRank {
		if c.promoted != nil {
			c.promoted()
		}
	}
}

func (c *Captain) LeaderAddress() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.leaderAddr
}

func (c *Captain) LeaderRank() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.leaderRank
}
