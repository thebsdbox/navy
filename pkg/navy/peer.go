package navy

import (
	"encoding/gob"
	"io"
)

// Peer is a `struct` representing a remote Peer.
type Peer struct {
	rank  int
	addr  string
	sock  *gob.Encoder
	Ready bool
}

// NewPeer returns a new `*Peer`.
func NewPeer(rank int, addr string, fd io.Writer) *Peer {
	return &Peer{rank: rank, addr: addr, sock: gob.NewEncoder(fd), Ready: true}
}
