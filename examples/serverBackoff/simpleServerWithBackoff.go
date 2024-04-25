package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/thebsdbox/navy/pkg/navy"
)

func main() {
	log.SetLevel(log.DebugLevel)
	bindaddr := flag.String("address", "0.0.0.0:9990", "The address of a peer and port")
	extadd := flag.String("externalAddress", "", "An external address to use as the peer address")
	rank := flag.Int("rank", 0, "The rank of a peer")
	ready := flag.Bool("ready", false, "Set this instance to ready")
	peers := flag.String("peers", "", "A comma seperated list of peers, each peer should be <rank>:<ADDRESS>:<PORT>")
	fleet := flag.String("fleet", "", "The address of an existing fleet member")
	callsign := flag.String("callsign", "", "The address of an existing fleet member")

	logLevel := flag.Int("log", 4, "The level of logging, (set to 5 for debug logs)")

	timeout := flag.Int("timeout", 0, "How long to wait before resigning from the fleet")
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
	log.Infof("Listenting on [%s]", *bindaddr)
	var members []string
	if *fleet != "" {
		members = strings.Split(*fleet, ",")
	}

	c := navy.NewCaptain(*rank, *bindaddr, *extadd, "tcp4", *callsign, members, *ready, true, remotePeers)

	err := c.Listen()
	if err != nil {
		log.Fatal(err)
	}

	// Enable the interupt
	c.DemoteOnQuit()

	//Discover!
	readyWatcher := make(chan interface{})
	go c.DiscoverResponse(readyWatcher)
	if !c.Ready {
		log.Info("Attepting to discover the fleet, with a backoff")
		err = c.DiscoverWithBackoff(navy.Backoff{MaxRetries: 3, Delay: time.Second})
		if err != nil {
			log.Fatal(err)
		}
		// hang about here until we're ready
		<-readyWatcher
	}

	file, err := os.OpenFile("/tmp/navy", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		log.Fatalf("Could not open /tmp/navy")
		return
	}

	defer file.Close()
	promotedFunc := func() {
		fmt.Printf("%s -> %d\n", time.Now().Format("15:04:05"), *rank)
	}

	demotionFunc := func() {
		fmt.Printf("%s <- %d\n", time.Now().Format("15:04:05"), *rank)
	}

	if *timeout != 0 {
		go func() {
			time.Sleep(time.Duration(*timeout) * time.Second)
			fmt.Println("Reached the timeout, resigning")

			c.Resign()
			//b.SetRank(10)
		}()
	}
	c.OnPromotion(promotedFunc)
	c.OnDemotion(demotionFunc)

	err = c.Run(nil)
	if err != nil {
		log.Fatal(err)
	}
}
