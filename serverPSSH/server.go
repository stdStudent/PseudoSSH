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
	commands chan command
}

func newServer() *server {
	return &server{
		commands: make(chan command),
	}
}

func (s *server) run() {
	for cmd := range s.commands {
		switch cmd.id {
		case CmdReg:
			s.reg(cmd.client, cmd.args)

		case CmdChPswd:
			s.chpswd(cmd.client, cmd.args)

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

		case CmdLogout:
			s.logout(cmd.client)

		case CmdHelp:
			s.help(cmd.client, cmd.args)

		case CmdRmUser:
			s.rmuser(cmd.client, cmd.args)

		case CmdLsUsers:
			s.lsusers(cmd.client)

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
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if !c.isAdmin {
		c.msg("Only admin can register users.")
		return
	}

	if len(args) < 3 {
		c.msg(`A nick and a password are required. Example: "reg [nick] [pswd]"`)
		return
	}

	nick := args[1]
	if _, err := os.Stat(db_path + nick + ".json"); err == nil {
		c.msg(fmt.Sprintf("User %s already exists. Use 'chpswd' to change password for a user.", nick))
		return
	}

	h := sha1.New()
	h.Write([]byte(args[2]))
	pswd := hex.EncodeToString(h.Sum(nil))

	db, _ := sjson.Set("", "nick", nick)
	db, _ = sjson.Set(db, "pswd", pswd)

	_ = os.WriteFile(db_path+nick+".json", []byte(db), 0755)
	_ = os.MkdirAll(users_path+nick+"/home", os.ModePerm)

	c.msg(fmt.Sprintf("You have successfully registered '%s'.", nick))
}

func (s *server) chpswd(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if !c.isAdmin {
		c.msg("Only admin can register users.")
		return
	}

	if len(args) < 3 {
		c.msg(`A nick and a password are required. Example: "chpswd [nick] [pswd]"`)
		return
	}

	nick := args[1]
	if _, err := os.Stat(db_path + nick + ".json"); errors.Is(err, os.ErrNotExist) {
		c.msg(fmt.Sprintf("User %s does NOT exists.", c.nick))
		return
	}

	h := sha1.New()
	h.Write([]byte(args[2]))
	new_pswd := hex.EncodeToString(h.Sum(nil))

	pathToFile := db_path + nick + ".json"
	content, _ := os.ReadFile(pathToFile)
	db := string(content)

	old_pswd := gjson.Get(db, "pswd").String()
	if old_pswd == new_pswd {
		c.msg("Current password and new passwords are the same. Proceeding nothing.")
		return
	}

	db, _ = sjson.Set(db, "pswd", new_pswd)
	_ = os.WriteFile(db_path+nick+".json", []byte(db), 0755)

	c.msg(fmt.Sprintf("You have successfully changed password for '%s'.", nick))
}

func (s *server) login(c *client, args []string) {
	if len(args) < 3 {
		c.msg(`A nick and a password are required. Example: "login [nick] [pswd]"`)
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

	pathToFile := db_path + c.nick + ".json"
	content, _ := os.ReadFile(pathToFile)
	db := string(content)

	isActive := gjson.Get(db, "isActive")
	if isActive.Bool() {
		c.msg("This user is already logged in.")
		return
	}

	pswd := gjson.Get(db, "pswd")
	if pswd.String() != c.pswd {
		c.msg("Wrong password.")
		return
	} else {
		c.isLoggedIn = true
		c.isAdmin = gjson.Get(db, "isAdmin").Bool()

		db, _ = sjson.Set(db, "isActive", true)
		err := os.WriteFile(pathToFile, []byte(db), 0755)
		if err != nil {
			log.Printf("Could NOU open file '%s'", db_path+c.nick+".json")
		}

		c.actDir = users_path + c.nick + "/home"
		c.homeDir = "/home"
		c.currDir = c.homeDir

		c.msg("You have successfully logged in.")
		log.Printf("A user '%s' has connected.", c.nick)
	}
}

func (s *server) pwd(c *client) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	c.msg(c.currDir)
}

func (s *server) write(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
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
		c.msg("You must log in first.")
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
		c.msg("You must log in first.")
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
			c.msg("Cannot go higher than the root directory.")
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

func (s *server) logout(c *client) {
	if !c.isLoggedIn {
		c.msg("Checking if you're logged in. Proceeding nothing.")
		return
	}

	pathToFile := db_path + c.nick + ".json"
	content, _ := os.ReadFile(pathToFile)
	db := string(content)
	db, _ = sjson.Set(db, "isActive", false)
	err := os.WriteFile(pathToFile, []byte(db), 0755)
	if err != nil {
		log.Printf(err.Error())
	}

	if c.isConnErr {
		log.Printf("A user '%s' has UNEXPECTEDLY disconnected.", c.nick)
	} else {
		log.Printf("A user '%s' has disconnected.", c.nick)
	}

	c.nick = ""
	c.pswd = ""
	c.actDir = ""
	c.homeDir = ""
	c.currDir = ""
	c.isLoggedIn = false
	c.isAdmin = false

	c.msg("You have successfully logged out.")
}

func (s *server) help(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if len(args) < 2 {
		c.msg(`Wrong usage. Example: "help [cmd]"`)
		return
	}

	switch args[1] {
	case "help":
		c.msg("'help' prints help. Usage: help [cmd]")
		break

	case "ls":
		c.msg("'ls' lists files of directory. Usage: ls [dir]")
		break

	case "write":
		c.msg("'write' inputs text to a file. Usage: write [file] [text]")
		break

	case "read":
		c.msg("'read' outputs text from a file. Usage: read [file]")
		break

	case "logout":
		c.msg("'logout' shut a user down. Usage: guess it yourself.")
		break

	case "pwd":
		c.msg("'pwd' prints current directory. Usage: guess it yourself.")
		break
	}
}

func (s *server) rmuser(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if !c.isAdmin {
		c.msg("Only admin can remove users.")
		return
	}

	if len(args) < 2 {
		c.msg(`Wrong usage. Example: "rmuser [nick]"`)
		return
	}

	nick := args[1]
	pathToFile := db_path + nick + ".json"
	if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
		c.msg(fmt.Sprintf("User '%s' does NOT exists.", nick))
		return
	}

	content, _ := os.ReadFile(pathToFile)
	db := string(content)
	isActive := gjson.Get(db, "isActive").Bool()
	if isActive {
		c.msg("The user '%s' is logged in. Proceeding nothing.")
		return
	}

	err := os.Remove(pathToFile)
	if err != nil {
		log.Printf(err.Error())
		c.err(err)
		return
	}

	if _, err := os.Stat(users_path + nick); err == nil {
		err := os.RemoveAll(users_path + nick)
		if err != nil {
			log.Printf(err.Error())
			c.err(err)
			return
		}
	}

	c.msg(fmt.Sprintf("You have successfully removed '%s'", nick))
}

func (s *server) lsusers(c *client) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if !c.isAdmin {
		c.msg("Only admin can list users.")
		return
	}

	var total_info string
	matches, _ := filepath.Glob(filepath.Join(db_path, "*.json"))
	for _, file := range matches {
		content, _ := os.ReadFile(file)
		db := string(content)
		total_info += "\n"
		total_info += db
	}

	total_info = strings.ReplaceAll(total_info, "\n\n", "\n") // Might delete later?
	c.msg(fmt.Sprintf("Users info: %s", total_info))
}

func (s *server) quit(c *client) {
	leftClient := c.conn.RemoteAddr().String()

	c.msg("You have successfully quited.")
	err := c.conn.Close()
	if err != nil {
		log.Printf("The client could NOT left the chat: %s", leftClient)
		return
	}

	log.Printf("The client has left the chat: %s", leftClient)
}
