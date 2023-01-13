package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"syscall"
)

type client struct {
	conn       net.Conn
	isLoggedIn bool
	isAdmin    bool
	nick       string
	pswd       string
	actDir     string
	homeDir    string
	currDir    string
	commands   chan<- command
	isConnErr  bool

	// lab2
	groups []string
}

func isNetConnClosedErr(err error) bool {
	switch {
	case
		errors.Is(err, net.ErrClosed),
		errors.Is(err, io.EOF),
		errors.Is(err, syscall.EPIPE):
		return true

	default:
		return false
	}
}

func (c *client) readInput() {
	for {
		msg, err := bufio.NewReader(c.conn).ReadString('\n')
		if isNetConnClosedErr(err) {
			c.isConnErr = true
			c.commands <- command{
				id:     CmdLogout,
				client: c,
			}
		}

		msg = strings.Trim(msg, "\r\n")

		args := strings.Split(msg, " ")
		cmd := strings.TrimSpace(args[0])

		switch cmd {
		case "reg":
			c.commands <- command{
				id:     CmdReg,
				client: c,
				args:   args,
			}

		case "chpswd":
			c.commands <- command{
				id:     CmdChPswd,
				client: c,
				args:   args,
			}

		case "login":
			c.commands <- command{
				id:     CmdLogout,
				client: c,
			}
			c.commands <- command{
				id:     CmdLogin,
				client: c,
				args:   args,
			}

		case "pwd":
			c.commands <- command{
				id:     CmdPwd,
				client: c,
			}

		case "write":
			c.commands <- command{
				id:     CmdWrite,
				client: c,
				args:   args,
			}

		case "read":
			c.commands <- command{
				id:     CmdRead,
				client: c,
				args:   args,
			}

		case "ls":
			c.commands <- command{
				id:     CmdLs,
				client: c,
				args:   args,
			}

		case "logout":
			c.commands <- command{
				id:     CmdLogout,
				client: c,
			}

		case "help":
			c.commands <- command{
				id:     CmdHelp,
				client: c,
				args:   args,
			}

		case "rmuser":
			c.commands <- command{
				id:     CmdRmUser,
				client: c,
				args:   args,
			}

		case "lsusers":
			c.commands <- command{
				id:     CmdLsUsers,
				client: c,
			}

		case "quit":
			c.commands <- command{
				id:     CmdLogout,
				client: c,
			}
			c.commands <- command{
				id:     CmdQuit,
				client: c,
			}

		// lab 2
		case "addgroup":
			c.commands <- command{
				id:     CmdAddGroup,
				client: c,
				args:   args,
			}

		case "u2g":
			c.commands <- command{
				id:     CmdU2G,
				client: c,
				args:   args,
			}

		case "trimgroup":
			c.commands <- command{
				id:     CmdTrimGroup,
				client: c,
				args:   args,
			}

		case "rmgroup":
			c.commands <- command{
				id:     CmdRmGroup,
				client: c,
				args:   args,
			}

		case "rr":
			c.commands <- command{
				id:     CmdRR,
				client: c,
				args:   args,
			}

		default:
			c.err(fmt.Errorf(`unknown command "%s"`, cmd))
		}
	}
}

func (c *client) err(err error) {
	if c.isConnErr {
		return
	}

	write, err := c.conn.Write([]byte("Error: " + err.Error() + "\n"))
	if err != nil {
		log.Printf("Error c.err(): %s. Bytes written: %d", err.Error(), write)
		return
	}
}

func (c *client) msg(msg string) {
	if c.isConnErr {
		return
	}

	write, err := c.conn.Write([]byte("> " + msg + "\n"))
	if err != nil {
		log.Printf("Error c.msg(): %s. Bytes written: %d", err.Error(), write)
		return
	}
}
