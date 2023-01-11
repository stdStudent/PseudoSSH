package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const users_path string = "users/"
const db_path string = "db/"

type server struct {
	rooms    map[string]*room
	commands chan command
}

func newServer() *server {
	return &server{
		rooms:    make(map[string]*room),
		commands: make(chan command),
	}
}

func (s *server) run() {
	for cmd := range s.commands {
		switch cmd.id {
		case CmdNick:
			s.reg(cmd.client, cmd.args)

		case CmdLogin:
			s.login(cmd.client, cmd.args)

		case CmdPwd:
			s.pwd(cmd.client)

		case CmdWrite:
			s.write(cmd.client, cmd.args)

		case CmdRead:
			s.read(cmd.client, cmd.args)

		case CmdLs:
			s.ls(cmd.client, cmd.args)

		case CmdJoin:
			s.join(cmd.client, cmd.args)

		case CmdRooms:
			s.listRooms(cmd.client)

		case CmdMsg:
			s.msg(cmd.client, cmd.args)

		case CmdQuit:
			s.quit(cmd.client)
		}
	}
}

func (s *server) newClient(conn net.Conn) *client {
	log.Printf(`A new client has joined from %s`, conn.RemoteAddr().String())

	return &client{
		conn:     conn,
		nick:     "anonymous",
		commands: s.commands,
	}
}

func (s *server) reg(c *client, args []string) {
	if len(args) < 3 {
		c.msg(`A nick and a password are required. Example: "/reg NICK PASSWORD"`)
		return
	}

	c.nick = args[1]
	if _, err := os.Stat(db_path + c.nick + ".json"); err == nil {
		c.msg(fmt.Sprintf("User %s already exists.", c.nick))
		return
	}

	h := sha1.New()
	h.Write([]byte(args[2]))
	c.pswd = hex.EncodeToString(h.Sum(nil))

	value, _ := sjson.Set("", "nick", c.nick)
	value, _ = sjson.Set(value, "pswd", c.pswd)

	_ = os.WriteFile(db_path+c.nick+".json", []byte(value), 0755)

	c.msg("You have successfully registered.")
}

func (s *server) login(c *client, args []string) {
	if len(args) < 3 {
		c.msg(`A nick and a password are required. Example: "/login NICK PASSWORD"`)
		return
	}

	c.nick = args[1]
	if _, err := os.Stat(db_path + c.nick + ".json"); errors.Is(err, os.ErrNotExist) {
		c.msg(fmt.Sprintf("User %s does NOT exists.", c.nick))
		return
	}

	h := sha1.New()
	h.Write([]byte(args[2]))
	c.pswd = hex.EncodeToString(h.Sum(nil))

	content, _ := os.ReadFile(db_path + c.nick + ".json")
	db := string(content)
	pswd := gjson.Get(db, "pswd")

	if pswd.String() != c.pswd {
		c.msg("Wrong password.")
		return
	} else {
		c.isLoggedIn = true
		c.actDir = users_path + c.nick + "/home"
		c.homeDir = "/home"
		c.currDir = c.homeDir
		_ = os.MkdirAll(c.actDir, os.ModePerm)
		c.msg("You successfully logged in.")
	}
}

func (s *server) pwd(c *client) {
	if !c.isLoggedIn {
		c.msg("You must log in first with /login")
		return
	}

	c.msg(c.currDir)
}

func (s *server) write(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first with /login")
		return
	}

	if len(args) < 3 {
		c.msg(`Wrong usage. Example: "write [filename] [text]"`)
		return
	}

	err := os.WriteFile(filepath.Join(c.actDir, args[1]), []byte(strings.Join(args[2:], " ")), 0755)
	if err != nil {
		c.err(err)
	} else {
		c.msg(fmt.Sprintf("You have successfully written text to '%s'", args[1]))
	}
}

func (s *server) read(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first with /login")
		return
	}

	if len(args) < 2 {
		c.msg(`Wrong usage. Example: "read [filename]"`)
		return
	}

	isFullPath := strings.HasPrefix(args[1], users_path)
	var err error
	var text []byte

	if isFullPath {
		text, err = os.ReadFile(args[1])
	} else {
		text, err = os.ReadFile(filepath.Join(c.actDir, args[1]))
	}

	if err != nil { // Couldn't read from file
		c.err(err)
		return
	}

	c.msg(fmt.Sprintf("Text from file '%s':\n%s", args[1], text))
}

func (s *server) ls(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first with /login")
		return
	}

	if len(args) < 2 {
		c.msg(`Wrong usage. Example: "ls [dir]". Use "ls ." for the current directory.`)
		return
	}

	isFullPath := strings.HasPrefix(args[1], users_path)
	var err error
	var files []fs.DirEntry

	if isFullPath {
		files, err = os.ReadDir(args[1])
	} else {
		if strings.HasPrefix(args[1], "../../..") {
			c.msg("Cannot go higher that root directory.")
			return
		}
		files, err = os.ReadDir(filepath.Join(c.actDir, args[1]))
	}

	if err != nil { // Couldn't read from dir
		c.err(err)
		return
	}

	var listOfFiles string
	for _, file := range files {
		listOfFiles += file.Name()
		listOfFiles += " "
	}

	c.msg(fmt.Sprintf("Files from directory '%s':\n%s", args[1], listOfFiles))
}

func (s *server) join(c *client, args []string) {
	if len(args) < 2 {
		c.msg(`A room name is required. Example: "/join AwesomeRoom"`)
		return
	}

	roomName := args[1]

	r, ok := s.rooms[roomName]
	if !ok {
		r = &room{
			name:    roomName,
			members: make(map[net.Addr]*client),
		}
		s.rooms[roomName] = r
	}
	r.members[c.conn.RemoteAddr()] = c

	s.quitCurrentRoom(c)
	c.room = r

	r.broadcast(c, fmt.Sprintf("%s has just joined the room.", c.nick))

	c.msg(fmt.Sprintf("Welcome to %s.", roomName))
}

func (s *server) listRooms(c *client) {
	var rooms []string
	for name := range s.rooms {
		rooms = append(rooms, name)
	}

	c.msg(fmt.Sprintf("Available rooms: %s", strings.Join(rooms, ", ")))
}

func (s *server) msg(c *client, args []string) {
	if len(args) < 2 {
		c.msg(`A message is required. Example: "/msg Hello"`)
		return
	}

	msg := strings.Join(args[1:], " ")
	c.room.broadcast(c, c.nick+": "+msg)
}

func (s *server) quit(c *client) {
	log.Printf("The client has left the chat: %s", c.conn.RemoteAddr().String())

	s.quitCurrentRoom(c)

	c.msg("You have successfully quited.")
	c.conn.Close()
}

func (s *server) quitCurrentRoom(c *client) {
	if c.room != nil {
		oldRoom := s.rooms[c.room.name]
		delete(s.rooms[c.room.name].members, c.conn.RemoteAddr())
		oldRoom.broadcast(c, fmt.Sprintf("%s has just left the room", c.nick))
	}
}
