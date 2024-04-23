package navy

import (
	"encoding/gob"
	"io"
	"net"
)

// Peer is a `struct` representing a remote Peer.
type Peer struct {
	sock  *gob.Encoder
	conn  *net.TCPConn
	Ready bool

	rank int
	addr string
}

// NewPeer returns a new `*Peer`.
func NewPeer(rank int, addr string, fd io.Writer, conn *net.TCPConn) *Peer {

	return &Peer{rank: rank, addr: addr, sock: gob.NewEncoder(fd), Ready: true, conn: conn}
}
