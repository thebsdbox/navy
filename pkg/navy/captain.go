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
func NewCaptain(rank int, bindaddr, extaddr, proto, callsign string, fleet []string, ready, interupt bool, peers map[int]string) *Captain {
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
		interupt:     interupt,
	}

	// if the external address is left blank then default to using the binded address
	if extaddr == "" {
		c.extaddr = c.bindaddr
	}

	return c
}

func NewCaptainandGo(rank int, bindaddr, extaddr, proto, callsign, payload string, fleet []string, ready, interupt bool, peers map[int]string) (*Captain, error) {

	c := NewCaptain(rank, bindaddr, extaddr, proto, callsign, fleet, ready, interupt, peers)
	c.internalPayload = payload
	if err := c.Listen(); err != nil {
		return nil, fmt.Errorf("new: %v", err)
	}

	// enable the interupt handler
	if c.interupt {
		c.DemoteOnQuit()
	}
	// Start the loop to handle the responses from the discovery

	// Do basic discovery on the fleet

	if len(fleet) != 0 {
		//Discover!
		readyWatcher := make(chan interface{})
		go c.DiscoverResponse(readyWatcher)
		err := c.Discover()
		if err != nil {
			return nil, fmt.Errorf("discovery failure [%v]", err)
		}
		<-readyWatcher
	}

	// attempt to connect with hardcoded peers
	if len(peers) != 0 {
		c.Connect(proto, peers)
	}

	return c, nil
}

func (c *Captain) SetPayload(payload string) {
	c.internalPayload = payload
}

func (c *Captain) GetLeaderPayload() string {
	return c.leaderPayload
}

func (c *Captain) DemoteOnQuit() {
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-s
		log.Infoln("[SIGNAL] caught syscall signal, ending")
		c.Resign()
		os.Exit(0)
	}()
}

func (c *Captain) OnPromotion(promotion func(chan interface{})) {
	c.promoted = promotion
}

func (c *Captain) OnDemotion(demotion func(chan interface{})) {
	c.demoted = demotion
}

// NOTE: This function is thread-safe.
func (c *Captain) SetLeader(Addr, payload string, rank int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fmt.Printf("rank %d leader %d myrank %d\n", rank, c.leaderRank, c.rank)

	// If the new leader has a higher rank they become leader
	if rank > c.leaderRank {

		// are we the current leader (i.e does the current leader, match our rank)
		// If this is true then we're leading
		if c.leaderRank <= c.rank {

			// Is the incoming rank higher, if so we're being demoted
			if rank > c.rank {
				if c.demoted != nil {
					c.leaderPayload = payload
					exit := make(chan interface{})
					c.demoted(exit)
					fmt.Printf("Demotion function complete")

					<-exit
				}
			}

			// If the incoming rank is our rank, we're being promoted
			if rank == c.rank {
				if c.promoted != nil {
					c.leaderPayload = payload
					exit := make(chan interface{})
					c.promoted(exit)
					<-exit
				}
			}

		}

		// Set all leader details
		c.leaderRank = rank
		c.leaderAddr = Addr
		c.leaderPayload = payload
	}

}

func (c *Captain) ResetLeader(Addr string, rank int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.leaderRank = c.rank
	c.leaderAddr = c.extaddr
	c.leaderPayload = c.internalPayload
	for _, peer := range c.peers.PeerData() {
		if peer.Rank > c.leaderRank {
			c.leaderRank = peer.Rank
			c.leaderAddr = peer.Addr
		}

	}
	if c.rank == c.leaderRank {
		if c.promoted != nil {
			exit := make(chan interface{})
			c.promoted(exit)
			<-exit
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

func (c *Captain) LeaveFleet() {
	log.Info("[Leave] this captain is leaving from the fleet")
	for _, peers := range c.peers.PeerData() {
		err := c.Send(peers.Rank, peers.Addr, CLOSE)
		if err != nil {
			log.Error(err)
		}
	}

	// if the demotion funcion is set and we're the leader, then call the demotion function
	if c.demoted != nil && c.rank == c.leaderRank {
		exit := make(chan interface{})
		c.demoted(exit)
		<-exit
	}
	close(c.quit)    // Annouce the quit
	err := c.Close() // Close the networking
	if err != nil {
		log.Error(err)
	}
	c.wg.Wait() // wait for all work to complete

}

func (c *Captain) Resign() {
	log.Info("[Leave] this captain is resigning from duty")
	// Stop processing any more messages
	close(c.discoverChan)
	close(c.receiveChan)
}
