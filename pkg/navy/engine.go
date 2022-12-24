package navy

import (
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const maxRetries = 5

// Captain is a `struct` representing a single node used by the `Bully Algorithm`.
//
// NOTE: More details about the `Bully algorithm` can be found here
// https://en.wikipedia.org/wiki/Bully_algorithm .
type Captain struct {
	*net.TCPListener

	rank         int
	addr         string
	proto        string
	leaderAddr   string
	leaderRank   int
	Ready        bool
	fleet        string
	callsign     string
	peers        Peers
	mu           *sync.RWMutex
	receiveChan  chan Message
	discoverChan chan Message
	electionChan chan Message
	promoted     func()
	demoted      func()
}

// Elect handles the leader election mechanism of the `Bully algorithm`.
func (c *Captain) Elect() {
	log.Debugf("[ELECTION] Current Rank %d, Peers: %v", c.rank, c.peers.PeerData())
	for _, peers := range c.peers.PeerData() {
		//if peers.Rank > c.rank {
		err := c.Send(peers.Rank, peers.Addr, ELECTION)
		if err != nil {
			log.Error(err)
		}
		//}
	}

	select {
	case <-c.electionChan:
		return
	case <-time.After(time.Second):
		c.SetLeader(c.addr, c.rank)
		for _, peers := range c.peers.PeerData() {
			log.Infof("[ELECTION] leader [%s], informing [%s]", c.addr, peers.Addr)
			err := c.Send(peers.Rank, peers.Addr, ADMIRAL)
			if err != nil {
				log.Error(err)
			}
		}
		return
	}
}

// Discover will discover the cluster
func (c *Captain) Discover() error {
	if c.fleet == "" {
		return fmt.Errorf("[Discover] No Fleet address")
	}

	// Ask the seed, who is the current leader
	err := c.SendOneShot(c.fleet, WHOISLEADER)
	if err != nil {
		return err
	}
	for msg := range c.discoverChan {
		switch msg.Type {
		case LEADER:
			// We've recieved the leader
			log.Infof("[LEADER] being updated to [%s %d]", msg.Addr, msg.Rank)
			c.SetLeader(msg.Addr, msg.Rank)

			//Ask the leader for all the peers
			err := c.SendOneShot(msg.Addr, PEERS)
			if err != nil {
				return err
			}

		case PEERLIST:
			// We should recieve the peer list for the current leader
			log.Infof("[PEERLIST] from [%s %d]", msg.Addr, msg.Rank)
			// Add the leader as a peer
			c.connect(c.proto, msg.Addr, msg.Rank)
			if c.LeaderRank() != msg.Rank {
				log.Errorf("Ignoring peers from [%s]", msg.Addr)
			} else {
				for x := range msg.Peers {
					// Stop loopback connections
					if msg.Peers[x].Addr != c.addr && msg.Peers[x].Rank != c.rank {
						c.connect(c.proto, msg.Peers[x].Addr, msg.Peers[x].Rank)
						err := c.Send(msg.Peers[x].Rank, msg.Peers[x].Addr, READY)
						if err != nil {
							log.Error(err)
						}
					}
				}
				log.Debugf("[PEERS] %v", c.peers.PeerData())
				c.Ready = true
				//c.Elect()
				return nil
			}
		}
	}
	return nil
}

// Run launches the two main goroutine. The first one is tied to the
// execution of `workFunc` while the other one is the `Bully algorithm`.
//
// NOTE: This function is an infinite loop.
func (c *Captain) Run(workFunc func()) {
	// Additional callback if needed
	if workFunc != nil {
		go workFunc()
	}
	// If this node is ready and has no other peers then run the election process
	// This effectively makes this node the leader
	if c.Ready {
		c.Elect()
	}

	// If this node isn't marked as ready, but has some peers then ask thos peers who is the leader
	if !c.Ready && len(c.peers.PeerData()) != 0 {
		for _, peer := range c.peers.PeerData() {
			if peer.Rank > c.rank || peer.Rank == 0 {
				err := c.Send(peer.Rank, peer.Addr, WHOISLEADER)
				if err != nil {
					log.Error(err)
				}
			}
		}
	}

	for msg := range c.receiveChan {

		switch msg.Type {
		case ELECTION:
			if c.Ready {
				if msg.Rank < c.rank {
					log.Warnf("[ELECTION] new election [%s %d]", msg.Addr, msg.Rank)
					err := c.Send(msg.Rank, msg.Addr, OK)
					if err != nil {
						log.Error(err)
					}
					c.Elect()
				}
			}
		case ADMIRAL:
			log.Infof("[ELECTION] setting new leader [%s %d]", msg.Addr, msg.Rank)
			c.SetLeader(msg.Addr, msg.Rank)

		case WHOISLEADER:
			log.Infof("[WHOISLEADER] from [%s %d]", msg.Addr, msg.Rank)
			if msg.CallSign != c.callsign {
				err := c.SendOneShot(msg.Addr, UNKNOWN)
				if err != nil {
					log.Error(err)
				}
			}

			err := c.SendOneShot(msg.Addr, LEADER)
			if err != nil {
				log.Error(err)
			}

		case PEERS:
			log.Infof("[PEERS] from [%s %d]", msg.Addr, msg.Rank)

			err := c.Send(msg.Rank, msg.Addr, PEERLIST)
			if err != nil {
				log.Error(err)
			}
		case READY:
			log.Debugf("[READY] member [%s / %d]", msg.Addr, msg.Rank)
			c.connect(c.proto, msg.Addr, msg.Rank)
		case UNKNOWN:
			log.Fatalf("[UNKNOWN] this peer has the wrong callsign for the fleet from [%s %d]", msg.Addr, msg.Rank)
		default:
			log.Warnf("Unknown message [%d]", msg.Type)

		}

	}
}
