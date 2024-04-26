package navy

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

// receive is a helper function handling communication between `Peer`s
// and `b`. It creates a `gob.Decoder` and a from a `io.ReadCloser`. Each
// `Message` received that is not of type `CLOSE` or `OK` is pushed to
// `b.receiveChan`.
//
// NOTE: this function is an infinite loop.
func (c *Captain) receive(rwc io.ReadCloser) {
	var msg Message
	dec := gob.NewDecoder(rwc)
	for {
		err := dec.Decode(&msg)
		log.Debugf("[RECEIVE] OneShot [%t] From [%s] Type [%s] err [%v]", msg.OneShot, msg.Addr, MessageStrings[msg.Type], err)
		if err == io.EOF || msg.Type == CLOSE {
			_ = rwc.Close()
			//check if this is an actual peer
			if c.peers.Find(Peer{addr: msg.Addr, rank: msg.Rank}) {
				log.Warnf("[PEER] lost [%s] Rank [%d] leaderRank [%d]", msg.Addr, msg.Rank, c.LeaderRank())
				c.peers.Delete(msg.Rank)
				// Check if this peer was the leader!
				if msg.Rank >= c.LeaderRank() {
					log.Errorf("[LEADER] lost [%s] ID [%d]", msg.Addr, msg.Rank)
					c.ResetLeader(msg.Addr, msg.Rank)
					c.Elect()
				}
			}

			break
		} else if msg.Type == OK {
			select {
			case c.electionChan <- msg:
				continue
			case <-time.After(200 * time.Millisecond):
				continue
			}
		} else if msg.Type == LEADER || msg.Type == PEERLIST || msg.Type == UNREADY || msg.Type == UNKNOWN {
			c.discoverChan <- msg
		} else {
			c.receiveChan <- msg
		}
	}
}

// listen is a helper function that spawns goroutines handling new `Peers`
// connections to `b`'s socket.
//
// NOTE: this function is an infinite loop.
func (c *Captain) listen() {
	defer c.wg.Done()
	for {
		conn, err := c.AcceptTCP()
		if err != nil {
			select {
			case <-c.quit:
				return
			default:
				log.Println("accept error", err)
			}
		} else {
			c.wg.Add(1)
			go c.receive(conn)
			c.wg.Done()
		}
	}
}

// Listen makes `b` listens on the address `addr` provided using the protocol
// `proto` and returns an `error` if something occurs.
func (c *Captain) Listen() error {
	laddr, err := net.ResolveTCPAddr(c.proto, c.bindaddr)
	if err != nil {
		return fmt.Errorf("Listen: %v", err)
	}
	c.TCPListener, err = net.ListenTCP(c.proto, laddr)
	if err != nil {
		return fmt.Errorf("Listen: %v", err)
	}
	c.wg.Add(1)
	go c.listen()
	return nil
}

// connect is a helper function that resolves the tcp address `addr` and try
// to establish a tcp connection using the protocol `proto`. The established
// connection is set to `c.peers[ID]` or the function returns an `error`
// if something occurs.
//
// NOTE: In the case `ID` already exists in `c.peers`, the new connection
// replaces the old one.
func (c *Captain) connect(proto, addr string, rank int) error {
	if c.peers.Find(Peer{addr: addr, rank: rank}) {
		log.Debugf("[CONNECT] member already exists [%d]", rank)
		return nil
	}
	log.Debugf("[CONNECT] -> [%s]", addr)
	raddr, err := net.ResolveTCPAddr(proto, addr)
	if err != nil {
		return fmt.Errorf("connect: %v", err)
	}
	sock, err := net.DialTCP(proto, nil, raddr)
	if err != nil {
		return fmt.Errorf("connect: %v", err)
	}
	c.peers.Add(rank, addr, sock, sock)
	log.Debugf("[PEERLIST] %v", c.peers.PeerData())
	return nil
}

// Connect performs a connection to the remote `Peer`s.
func (c *Captain) Connect(proto string, peers map[int]string) {
	for Rank, addr := range peers {
		if c.rank == Rank {
			continue
		}
		if err := c.connect(proto, addr, Rank); err != nil {
			log.Errorf("[Connect] %v", err)
			c.peers.Delete(Rank)
		}
	}
}

// Send sends a `captain.Message` of type `what` to `c.peer[to]` at the address
// `addr`. If no connection is reachable at `addr` or if `c.peer[to]` does not
// exist, the function retries five times and returns an `error` if it does not
// succeed.
func (c *Captain) Send(rank int, addr string, msg int) error {

	if !c.peers.Find(Peer{addr: addr, rank: rank}) {
		log.Debugf("[SEND] Didn't find [%d]", rank)
		err := c.connect("tcp4", addr, rank)
		if err != nil {
			log.Error(err)
		}
	}
	var err error
	for attempts := 0; ; attempts++ {
		switch msg {
		case PEERLIST:
			err = c.peers.Write(rank, &Message{Rank: c.rank, Addr: c.extaddr, Peers: c.peers.PeerData(), Type: msg, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		case LEADER:
			log.Infof("[LEADER] informing %s of leader %s %d", addr, c.LeaderAddress(), c.LeaderRank())
			err = c.peers.Write(rank, &Message{Rank: c.LeaderRank(), Addr: c.LeaderAddress(), Type: msg, CallSign: c.callsign, Payload: c.internalPayload}) //TODO: check if payload is needed here otherwise we're sending more data than needed
			if err != nil {
				log.Error(err)
			}
		case PEERS:
			err = c.peers.Write(rank, &Message{Rank: c.rank, Addr: c.extaddr, Type: msg, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		case ADMIRAL:
			err = c.peers.Write(rank, &Message{Rank: c.rank, Addr: c.extaddr, Type: msg, CallSign: c.callsign, Payload: c.internalPayload})
			if err != nil {
				log.Error(err)
			}
		default:
			err = c.peers.Write(rank, &Message{Rank: c.rank, Addr: c.extaddr, Type: msg, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		}

		if err == nil {
			break
		}
		if attempts > maxRetries && err != nil {
			return fmt.Errorf("Send: %v", err)
		}
		err = c.connect("tcp4", addr, rank)
		if err != nil {
			log.Error(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func (c *Captain) SendOneShot(addr string, msg int) error {
	raddr, err := net.ResolveTCPAddr(c.proto, addr)
	if err != nil {
		return fmt.Errorf("connect: %v", err)
	}
	sock, err := net.DialTCP(c.proto, nil, raddr)
	if err != nil {
		return fmt.Errorf("connect: %v", err)
	}
	log.Debugf("[CONNECT] -> [%s], for discovery", addr)

	defer sock.Close()
	encoder := gob.NewEncoder(sock)

	for attempts := 0; ; attempts++ {
		switch msg {
		case PEERLIST:
			err = encoder.Encode(&Message{Rank: c.rank, Addr: c.extaddr, Peers: c.peers.PeerData(), Type: msg, CallSign: c.callsign, OneShot: true})
			if err != nil {
				log.Error(err)
			}
		case LEADER:
			if c.LeaderAddress() == "" {
				log.Warnf("[LEADER] unable to informing [%s] of a LEADER as one currently doesn't exist", addr)
				err = encoder.Encode(&Message{Rank: c.LeaderRank(), Addr: c.LeaderAddress(), Type: UNREADY, CallSign: c.callsign, OneShot: true})
				if err != nil {
					log.Error(err)
				}
			} else {
				log.Infof("[LEADER] informing %s of leader %s %d", addr, c.LeaderAddress(), c.LeaderRank())
				err = encoder.Encode(&Message{Rank: c.LeaderRank(), Addr: c.LeaderAddress(), Type: msg, CallSign: c.callsign, OneShot: true})
				if err != nil {
					log.Error(err)
				}
			}
		case PEERS:
			err = encoder.Encode(&Message{Rank: c.rank, Addr: c.extaddr, Type: msg, CallSign: c.callsign, OneShot: true})
			if err != nil {
				log.Error(err)
			}
		case UNKNOWN:
			log.Infof("[UNKNOWN] informing %s of leader %s %d", addr, c.LeaderAddress(), c.LeaderRank())

			err = encoder.Encode(&Message{Rank: c.rank, Addr: c.extaddr, Type: msg, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		default:
			err = encoder.Encode(&Message{Rank: c.rank, Addr: c.extaddr, Type: msg, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		}

		if err == nil {
			break
		}
		if attempts > maxRetries && err != nil {
			return fmt.Errorf("Send: %v", err)
		}
		sock, err := net.DialTCP(c.proto, nil, raddr)
		if err != nil {
			return fmt.Errorf("connect: %v", err)
		}
		encoder = gob.NewEncoder(sock)
		time.Sleep(100 * time.Millisecond)
	}
	// Send a close message as this is a oneshot
	return encoder.Encode(&Message{Rank: c.rank, Addr: c.extaddr, Type: CLOSE, CallSign: c.callsign})
}
