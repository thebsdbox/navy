package navy

// Message Types.
const (
	ELECTION = iota
	OK
	ADMIRAL
	WHOISLEADER // client > existing member
	LEADER      // existing member (sends leader) to discover client
	PEERS       // discover client asks leader
	PEERLIST    // leader deliver peers
	CLOSE
)

// Message is a `struct` used for communication between `captain`s.
type Message struct {
	Rank   int
	Addr   string
	Type   int
	Origin string
	Peers  []struct {
		Rank  int
		Addr  string
		Ready bool
	}
}
