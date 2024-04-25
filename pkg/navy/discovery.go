package navy

import (
	"fmt"
	"math"
	"time"

	log "github.com/sirupsen/logrus"
)

type Backoff struct {
	MaxRetries int
	Delay      time.Duration
}

// Discover will discover the cluster
func (c *Captain) Discover() error {
	if len(c.fleet) == 0 {
		return fmt.Errorf("[Discover] No Fleet address")
	}
	err := c.discover()
	if err != nil {
		return err
	}
	return nil
}

func (c *Captain) DiscoverWithBackoff(b Backoff) error {

	if len(c.fleet) == 0 {
		return fmt.Errorf("[Discover] No Fleet address")
	}

	return RetryWithBackoff(b, c.discover)

}

func RetryWithBackoff(b Backoff, retryFunc func() error) error {

	var lastError error
	for i := 0; i < b.MaxRetries; i++ {
		err := retryFunc()
		if err == nil {
			// It has worked
			return err
		}
		// power of 2 for each attempt (1, 2, 4)
		nextRetry := math.Pow(2, float64(i))
		time.Sleep(time.Duration(nextRetry) * b.Delay)
		lastError = err
	}
	return lastError
}

func (c *Captain) discover() (err error) {
	for member := range c.fleet {
		// Ask the seed, who is the current leader
		err = c.SendOneShot(c.fleet[member], WHOISLEADER)
		if err == nil {
			return err
		}
	}
	return err
}

func (c *Captain) DiscoverResponse(ready chan interface{}) error {
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
			err := c.connect(c.proto, msg.Addr, msg.Rank)
			if err != nil {
				return err
			}
			if c.LeaderRank() != msg.Rank {
				log.Errorf("Ignoring peers from [%s]", msg.Addr)
			} else {
				for x := range msg.Peers {
					// Stop loopback connections
					if msg.Peers[x].Addr != c.extaddr && msg.Peers[x].Rank != c.rank {
						err := c.connect(c.proto, msg.Peers[x].Addr, msg.Peers[x].Rank)
						if err != nil {
							return err
						}
						err = c.Send(msg.Peers[x].Rank, msg.Peers[x].Addr, READY)
						if err != nil {
							log.Error(err)
						}
					}
				}
				log.Debugf("[PEERS] %v", c.peers.PeerData())
				c.Ready = true
				close(ready)
				//c.Elect()
				return nil
			}
		case UNREADY:
			log.Warnf("[UNREADY] no leader currently exists in the cluster from [%s]", msg.Addr)
		case UNKNOWN:
			log.Fatalf("[UNKNOWN] this peer has the wrong callsign for the fleet from [%s %d]", msg.Addr, msg.Rank)
		}
	}
	return nil
}
