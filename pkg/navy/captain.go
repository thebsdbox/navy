package navy

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// NewCaptain returns a new `Captain` or an `error`.
//
// NOTE: All connections to `Peer`s are established during this function.
//
// NOTE: The `proto` value can be one of this list: `tcp`, `tcp4`, `tcp6`.
func NewCaptain(rank int, bindaddr, extaddr, proto, callsign string, fleet []string, ready bool, peers map[int]string) (*Captain, error) {
	c := &Captain{
		quit:         make(chan interface{}),
		rank:         rank,
		bindaddr:     bindaddr,
		extaddr:      extaddr,
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

	// if the external address is left blank then default to using the binded address
	if extaddr == "" {
		c.extaddr = c.bindaddr
	}

	if err := c.Listen(proto, bindaddr); err != nil {
		return nil, fmt.Errorf("new: %v", err)
	}

	if len(fleet) != 0 {
		err := c.Discover()
		if err != nil {
			return nil, fmt.Errorf("discovery failure [%v]", err)
		}
	}
	c.Connect(proto, peers)
	c.DemoteOnQuit()
	return c, nil
}

// func (c *Captain) SetRank(rank int) {
// 	c.rank = rank
// 	for _, peers := range c.peers.PeerData() {
// 		err := c.Send(peers.Rank, peers.Addr, PROMOTION)
// 		if err != nil {
// 			log.Error(err)
// 		}
// 	}
// 	c.Elect()
// }

func (c *Captain) DemoteOnQuit() {
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-s
		c.Resign()
		os.Exit(0)
	}()
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
	c.leaderAddr = c.extaddr

	for _, peer := range c.peers.PeerData() {
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

func (c *Captain) Resign() {
	log.Info("[RESIGN] this captain is resigning from the fleet")
	for _, peers := range c.peers.PeerData() {
		err := c.Send(peers.Rank, peers.Addr, CLOSE)
		if err != nil {
			log.Error(err)
		}
	}

	// if the demotion funcion is set and we're the leader, then call the demotion function
	if c.demoted != nil && c.rank == c.leaderRank {
		c.demoted()
	}
	close(c.quit)    // Annouce the quit
	err := c.Close() // Close the networking
	if err != nil {
		log.Error(err)
	}
	c.wg.Wait() // wait for all work to complete
	close(c.receiveChan)
}
