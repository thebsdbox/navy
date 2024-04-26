package navy

import (
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
	// handle the networking
	*net.TCPListener

	// handle all of the closing of connections
	quit chan interface{}
	wg   sync.WaitGroup

	// cluster confoguration
	rank         int
	bindaddr     string
	extaddr      string
	proto        string
	leaderAddr   string
	leaderRank   int
	Ready        bool
	fleet        []string
	callsign     string
	peers        Peers
	mu           *sync.RWMutex
	receiveChan  chan Message
	discoverChan chan Message
	electionChan chan Message
	promoted     func()
	demoted      func()

	interupt bool

	internalPayload string // optional, contains our local payload to transmit
	leaderPayload   string // optional, contains the payload of the current leader
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
		// Timer for election has expired
		c.SetLeader(c.extaddr, c.internalPayload, c.rank)
		for _, peers := range c.peers.PeerData() {
			log.Infof("[ELECTION] leader [%s], informing [%s]", c.extaddr, peers.Addr)
			err := c.Send(peers.Rank, peers.Addr, ADMIRAL)
			if err != nil {
				log.Error(err)
			}
		}
		return
	}
}

// Run launches the two main goroutine. The first one is tied to the
// execution of `workFunc` while the other one is the `Bully algorithm`.
//
// NOTE: This function is an infinite loop.
func (c *Captain) Run(workFunc func()) error {
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
		// format, _ := json.MarshalIndent(msg, "", "   ")
		// log.Debugf("%s", format)
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
			c.SetLeader(msg.Addr, msg.Payload, msg.Rank)

		case WHOISLEADER:
			if msg.CallSign != c.callsign {
				log.Warnf("[WHOISLEADER] unknown callsign from [%s %d]", msg.Addr, msg.Rank)
				err := c.SendOneShot(msg.Addr, UNKNOWN)
				if err != nil {
					log.Error(err)
				}
			} else {
				log.Infof("[WHOISLEADER] from [%s %d]", msg.Addr, msg.Rank)
				err := c.SendOneShot(msg.Addr, LEADER)
				if err != nil {
					log.Error(err)
				}
			}
		case PEERS:
			log.Infof("[PEERS] from [%s %d]", msg.Addr, msg.Rank)

			err := c.Send(msg.Rank, msg.Addr, PEERLIST)
			if err != nil {
				log.Error(err)
			}
		case READY:
			log.Debugf("[READY] member [%s / %d]", msg.Addr, msg.Rank)
			err := c.connect(c.proto, msg.Addr, msg.Rank)
			if err != nil {
				return err
			}
		case PROMOTION:
			log.Debugf("[PROMOTION] member [%s / %d]", msg.Addr, msg.Rank)

		default:
			log.Warnf("Unknown message [%d]", msg.Type)

		}

	}
	return nil
}
