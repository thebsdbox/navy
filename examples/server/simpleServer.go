package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/thebsdbox/navy/pkg/navy"
)

func main() {
	log.SetLevel(log.DebugLevel)
	addr := flag.String("address", "0.0.0.0:9990", "The address of a peer and port")
	rank := flag.Int("rank", 0, "The rank of a peer")
	ready := flag.Bool("ready", false, "Set this instance to ready")
	peers := flag.String("peers", "", "A comma seperated list of peers, each peer should be <rank>:<ADDRESS>:<PORT>")
	fleet := flag.String("fleet", "", "The address of an existing fleet member")
	callsign := flag.String("callsign", "", "The address of an existing fleet member")

	logLevel := flag.Int("log", 4, "The level of logging, (set to 5 for debug logs)")
	//Parse the flags
	flag.Parse()

	// Set the logging level
	log.SetLevel(log.Level(*logLevel))

	// Prepopulate peers (optional)
	remotePeers := map[int]string{}
	if *peers != "" {
		peer := strings.Split(*peers, ",")

		for x := range peer {
			peerConfig := strings.Split(peer[x], ":")
			if len(peerConfig) != 3 {
				log.Fatalf("A Peer should consist of <rank>:<ADDRESS>:<PORT>")
			}
			peerrank, err := strconv.Atoi(peerConfig[0])
			if err != nil {
				log.Fatal(err)
			}
			remotePeers[peerrank] = fmt.Sprintf("%s:%s", peerConfig[1], peerConfig[2])
		}
	}

	// Start the new member (captain)
	log.Infof("Listenting on [%s]", *addr)

	b, err := navy.NewCaptain(*rank, *addr, "tcp4", *fleet, *callsign, *ready, remotePeers)
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.OpenFile("/tmp/navy", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		log.Fatalf("Could not open /tmp/navy")
		return
	}

	defer file.Close()

	promotedFunc := func() {
		log.Info("Im the Admiral")
		_, err = file.WriteString(fmt.Sprintf("-> %d\n", *rank))
		if err != nil {
			log.Fatalf("Could write to /tmp/navy [%v]", err)
			return
		}
	}
	demotionFunc := func() {
		log.Info("I've been demoted to Captain'")
		_, err = file.WriteString(fmt.Sprintf("<- %d\n", *rank))
		if err != nil {
			log.Fatalf("Could write to /tmp/navy [%v]", err)
			return
		}
	}

	b.OnPromotion(promotedFunc)
	b.OnDemotion(demotionFunc)

	b.Run(nil)
}
