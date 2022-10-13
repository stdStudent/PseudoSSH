package main

import (
	"C"
	"flag"
	"fmt"
	"github.com/reiver/go-telnet"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) < 3 {
		printHelpMsg()
	}

	ip := os.Args[1]
	port := os.Args[2]

	help := flag.Bool("help", false, "Display help")
	flag.Parse()

	if *help {
		printHelpMsg()
	}

	/* Handle CTRL+C. */
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nExiting.")
		//os.Stdin.WriteString("\\quit\r\n")
		//time.Sleep(5 * time.Second)
		os.Exit(0)
	}()

	/* Client itself. */
	fmt.Printf("Connecting to %s:%s\n", ip, port)
	fmt.Println("To exit the program press CTRL+C.")

	var caller telnet.Caller = telnet.StandardCaller
	err := telnet.DialToAndCall(ip+":"+port, caller)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func printHelpMsg() {
	fmt.Println("This program allows to connect to a pseudo ssh server.")
	fmt.Println("Usage: Run './clientPSSH [ip] [port]' to connect to a server.")
	os.Exit(0)
}
