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
	READY       // node is ready
	UNREADY     // cluster is unready (no admiral)
	UNKNOWN     // don't recognise the callsign
	PROMOTION   // This captain got a promotion
	CLOSE       // Close the connection
)

// Message is a `struct` used for communication between `captain`s.
type Message struct {
	Rank     int    // incoming rank of a captain
	Addr     string // address they're coming from
	Type     int    // Message type
	CallSign string //
	OneShot  bool   // A OneShot message
	Peers    []struct {
		Rank  int
		Addr  string
		Ready bool
	}
}
