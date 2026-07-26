package main

import (
	"flag"
	"github.com/advpl/compiler/pkg/p2p"
	"log"
	"net"
)

func cmdPeer(args []string) {
	fs := flag.NewFlagSet("peer", flag.ExitOnError)
	listenAddr := fs.String("listen", "0.0.0.0:9000", "Listen address")
	_ = fs.String("bootstrap", "", "Bootstrap peer address")

	fs.Parse(args)

	addr, err := net.ResolveUDPAddr("udp", *listenAddr)
	if err != nil {
		log.Fatalf("resolve listen: %v", err)
	}

	peer := p2p.NewPeer("my-peer", addr)

	transport, err := p2p.NewTransport(addr)
	if err != nil {
		log.Fatalf("transport: %v", err)
	}
	defer transport.Close()

	go transport.Listen()

	log.Printf("Peer %s listening on %s", peer.ID, peer.Addr)

	// TODO: Bootstrap if specified
	// TODO: Join ring
	// TODO: Serve indefinitely

	for msg := range transport.Received {
		log.Printf("Received: %s", msg.Type)
	}
}
