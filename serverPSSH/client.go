package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

type client struct {
	conn       net.Conn
	isLoggedIn bool
	nick       string
	pswd       string
	actDir     string
	homeDir    string
	currDir    string
	room       *room
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
		case "/reg":
			c.commands <- command{
				id:     CmdNick,
				client: c,
				args:   args,
			}

		case "/login":
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

		case "/join":
			c.commands <- command{
				id:     CmdJoin,
				client: c,
				args:   args,
			}

		case "/rooms":
			c.commands <- command{
				id:     CmdRooms,
				client: c,
			}

		case "/msg":
			c.commands <- command{
				id:     CmdMsg,
				client: c,
				args:   args,
			}

		case "/quit":
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
	c.conn.Write([]byte("Error: " + err.Error() + "\n"))
}

func (c *client) msg(msg string) {
	c.conn.Write([]byte("> " + msg + "\n"))
}
