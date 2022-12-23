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
		if err := dec.Decode(&msg); err == io.EOF || msg.Type == CLOSE {
			//log.Debugf("%v", msg)
			_ = rwc.Close()
			//check if this is an actual peer
			if c.peers.Find(msg.Rank) {
				log.Warnf("[PEER] lost [%s] ID [%d]", msg.Addr, msg.Rank)
				c.peers.Delete(msg.Rank)
				// Check if this peer was the leader!
				if msg.Rank == c.LeaderRank() {
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
		} else if msg.Type == LEADER || msg.Type == PEERLIST {
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
	for {
		conn, err := c.AcceptTCP()
		if err != nil {
			log.Printf("listen: %v", err)
			continue
		}
		go c.receive(conn)
	}
}

// Listen makes `b` listens on the address `addr` provided using the protocol
// `proto` and returns an `error` if something occurs.
func (c *Captain) Listen(proto, addr string) error {
	laddr, err := net.ResolveTCPAddr(proto, addr)
	if err != nil {
		return fmt.Errorf("Listen: %v", err)
	}
	c.TCPListener, err = net.ListenTCP(proto, laddr)
	if err != nil {
		return fmt.Errorf("Listen: %v", err)
	}
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
func (c *Captain) connect(proto, addr string, ID int) error {
	if c.peers.Find(ID) {
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
	c.peers.Add(ID, addr, sock)
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
func (c *Captain) Send(to int, addr string, what int) error {

	if !c.peers.Find(to) {
		err := c.connect("tcp4", addr, to)
		if err != nil {
			log.Error(err)
		}
	}
	var err error
	for attempts := 0; ; attempts++ {
		switch what {
		case PEERLIST:
			err = c.peers.Write(to, &Message{Rank: c.rank, Addr: c.addr, Peers: c.peers.PeerData(), Type: what, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		case LEADER:
			log.Infof("[LEADER] informing %s of leader %s %d", addr, c.LeaderAddress(), c.LeaderRank())
			err = c.peers.Write(to, &Message{Rank: c.LeaderRank(), Addr: c.LeaderAddress(), Type: what, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		case PEERS:
			err = c.peers.Write(to, &Message{Rank: c.rank, Addr: c.addr, Type: what, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		default:
			err = c.peers.Write(to, &Message{Rank: c.rank, Addr: c.addr, Type: what, CallSign: c.callsign})
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
		err = c.connect("tcp4", addr, to)
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
			err = encoder.Encode(&Message{Rank: c.rank, Addr: c.addr, Peers: c.peers.PeerData(), Type: msg, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		case LEADER:
			log.Infof("[LEADER] informing %s of leader %s %d", addr, c.LeaderAddress(), c.LeaderRank())
			err = encoder.Encode(&Message{Rank: c.LeaderRank(), Addr: c.LeaderAddress(), Type: msg, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		case PEERS:
			err = encoder.Encode(&Message{Rank: c.rank, Addr: c.addr, Type: msg, CallSign: c.callsign})
			if err != nil {
				log.Error(err)
			}
		default:
			err = encoder.Encode(&Message{Rank: c.rank, Addr: c.addr, Type: msg, CallSign: c.callsign})
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
		encoder := gob.NewEncoder(sock)
		err = encoder.Encode(&Message{Rank: c.rank, Addr: c.addr, Peers: nil, Type: WHOISLEADER, CallSign: c.callsign})
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}
