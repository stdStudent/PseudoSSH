package main

import (
	"github.com/tidwall/sjson"
	"log"
	"net"
	"os"
	"path/filepath"
)

func main() {
	matches, _ := filepath.Glob(filepath.Join(db_path, "*.json"))
	for _, file := range matches {
		content, _ := os.ReadFile(file)
		db := string(content)
		db, _ = sjson.Set(db, "isActive", false)
		db, _ = sjson.Set(db, "isBeingAudited", false) // lab4
		err := os.WriteFile(file, []byte(db), 0755)
		if err != nil {
			log.Printf(err.Error())
		}
	}

	_ = os.MkdirAll("files", os.ModePerm)

	// The server itself
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
