package main

import (
	"log"
	"net"
)

func main() {
	s := newServer()
	go s.run()

	listener, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatalf("[%s] Unable to start the server.", err.Error())
	}

	defer listener.Close()
	log.Printf("Server has been started on :8888")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[%s] Failed to accept the connection.", err.Error())
			continue
		}

		c := s.newClient(conn)
		go c.readInput()
	}
}
