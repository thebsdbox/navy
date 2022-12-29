package main

import (
	"flag"

	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.DebugLevel)
	addr := flag.String("address", "0.0.0.0:9990", "The address of a peer and port")
	//rank := flag.Int("rank", 0, "The rank of a peer")
	// ready := flag.Bool("ready", false, "Set this instance to ready")
	// peers := flag.String("peers", "", "A comma seperated list of peers, each peer should be <rank>:<ADDRESS>:<PORT>")
	// fleet := flag.String("fleet", "", "The address of an existing fleet member")
	// callsign := flag.String("callsign", "", "The address of an existing fleet member")

	logLevel := flag.Int("log", 4, "The level of logging, (set to 5 for debug logs)")
	//Parse the flags
	flag.Parse()

	// Set the logging level
	log.SetLevel(log.Level(*logLevel))

	log.Infof("Connecting to [%s]", *addr)

}
