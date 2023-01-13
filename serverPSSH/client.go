package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
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
}

func (c *client) readInput() {
	for {
		msg, err := bufio.NewReader(c.conn).ReadString('\n')
		if err != nil {
			return
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

		default:
			c.err(fmt.Errorf(`unknown command "%s"`, cmd))
		}
	}
}

func (c *client) err(err error) {
	write, err := c.conn.Write([]byte("Error: " + err.Error() + "\n"))
	if err != nil {
		log.Printf("Error c.err(): %s. Bytes written: %d", err.Error(), write)
		return
	}
}

func (c *client) msg(msg string) {
	write, err := c.conn.Write([]byte("> " + msg + "\n"))
	if err != nil {
		log.Printf("Error c.msg(): %s. Bytes written: %d", err.Error(), write)
		return
	}
}
