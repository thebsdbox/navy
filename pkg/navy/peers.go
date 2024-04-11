package navy

import (
	"fmt"
	"io"
	"sync"
)

// Peers is an `interface` exposing methods to handle communication with other
// `captain.captain`s.
//
// NOTE: This project offers a default implementation of the `Peers` interface
// that provides basic functions. This will work for the most simple of use
// cases fo exemples, although I strongly recommend you provide your own, safer
// implementation while doing real work.
type Peers interface {
	Add(rank int, addr string, fd io.Writer)
	Delete(rank int)
	Find(Peer) bool
	Write(rank int, msg interface{}) error
	PeerData() []struct {
		Rank  int
		Addr  string
		Ready bool
	}
}

// PeerMap is a `struct` implementing the `Peers` interface and representing
// a container of `captain.Peer`s.
type PeerMap struct {
	mu    *sync.RWMutex
	peers map[int]*Peer
}

// NewPeerMap returns a new `captain.PeerMap`.
func NewPeerMap() *PeerMap {
	return &PeerMap{mu: &sync.RWMutex{}, peers: make(map[int]*Peer)}
}

// Add creates a new `captain.Peer` and adds it to `pm.peers` using `ID` as a key.
//
// NOTE: This function is thread-safe.
func (pm *PeerMap) Add(rank int, addr string, fd io.Writer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.peers[rank] = NewPeer(rank, addr, fd)
}

// Delete erases the `captain.Peer` corresponding to `ID` from `pm.peers`.
//
// NOTE: This function is thread-safe.
func (pm *PeerMap) Delete(rank int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	// close(*net.TCPConn(pm.peers[ID].sock))
	delete(pm.peers, rank)
}

// Find returns `true` if `pm.peers[ID]` exists, `false` otherwise.
//
// NOTE: This function is thread-safe.
func (pm *PeerMap) Find(p Peer) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	foundp := pm.peers[p.rank]
	if foundp == nil {
		return false
	}
	if foundp.addr == p.addr && foundp.rank == p.rank {
		return true
	}
	return false
}

// Write writes `msg` to `pm.peers[ID]`. It returns `nil` or an `error` if
// something occurs.
//
// NOTE: This function is thread-safe.
func (pm *PeerMap) Write(rank int, msg interface{}) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if p, ok := pm.peers[rank]; !ok {
		return fmt.Errorf("Write: peer %d not found in PeerMap", rank)

	} else if err := p.sock.Encode(msg); err != nil {
		return fmt.Errorf("Write: %v", err)
	}
	return nil
}

// PeerData returns a slice of anonymous structures representing a tupple
// composed of a `Peer.ID` and `Peer.addr`.
//
// NOTE: This function is thread-safe.
func (pm *PeerMap) PeerData() []struct {
	Rank  int
	Addr  string
	Ready bool
} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var RankSlice []struct {
		Rank  int
		Addr  string
		Ready bool
	}
	for _, peer := range pm.peers {
		RankSlice = append(RankSlice, struct {
			Rank  int
			Addr  string
			Ready bool
		}{
			peer.rank,
			peer.addr,
			peer.Ready,
		})
	}
	return RankSlice
}
