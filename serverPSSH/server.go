package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/miracl/conflate"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const users_path string = "users/"
const db_path string = "db/"
const group_path = "group/"
const db_files = "files/files.json"

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

		// lab2
		case CmdAddGroup:
			s.addgroup(cmd.client, cmd.args)

		case CmdU2G:
			s.u2g(cmd.client, cmd.args)

		case CmdTrimGroup:
			s.trimgroup(cmd.client, cmd.args)

		case CmdRmGroup:
			s.rmgroup(cmd.client, cmd.args)

		case CmdRR:
			s.rr(cmd.client, cmd.args)

		case CmdChMod:
			s.chmod(cmd.client, cmd.args)
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

		matches, _ := filepath.Glob(filepath.Join(group_path, "*.json"))
		for _, file := range matches {
			content, _ := os.ReadFile(file)
			db := string(content)
			isInGroup, _ := inGroup(db, c.nick)
			if isInGroup {
				c.groups = append(c.groups, gjson.Get(db, "name").String())
			}
		}

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

	if strings.HasPrefix(args[1], "../../..") {
		c.msg("Cannot go higher than the root directory to write a file.")
		return
	}

	pathToFile := filepath.Join(c.actDir, args[1])
	content, _ := os.ReadFile(db_files)
	old_db := string(content)

	isExists := false
	if _, err := os.Stat(pathToFile); err == nil {
		isExists = true
	}

	if !isExists {
		err := os.WriteFile(pathToFile, []byte(strings.Join(args[2:], " ")), 0755)
		if err != nil {
			c.err(err)
			return
		}
	} else {
		fileRights := gjson.Get(old_db, pathToFile+".rights").Int()
		switch fileRights & 0b0101 {
		case 0b0101:
			if c.nick != gjson.Get(old_db, pathToFile+".owner").String() {
				c.msg("DS: You are not the owner of this file.")
				return
			}

			isAllowedToWrite := false
			fileGroup := gjson.Get(old_db, pathToFile+".group").String()
			for _, group := range c.groups {
				if group == fileGroup {
					isAllowedToWrite = true
				}
			}

			if isAllowedToWrite == false {
				c.msg(fmt.Sprintf("DS: You are NOT in the group '%s'", fileGroup))
				return
			}

			err := os.WriteFile(pathToFile, []byte(strings.Join(args[2:], " ")), 0755)
			if err != nil {
				c.err(err)
				return
			}

		default:
			c.msg("DS: NOT allowed to read this file due to the rights.")
			return
		}
	}

	if strings.Contains(pathToFile, ".") {
		pathToFile = strings.ReplaceAll(pathToFile, ".", "\\.")
	}

	new_db, _ := sjson.Set("", pathToFile+".owner", c.nick)
	if c.isAdmin {
		new_db, _ = sjson.Set(new_db, pathToFile+".group", "admins")
	} else {
		new_db, _ = sjson.Set(new_db, pathToFile+".group", "users")
	}
	if !isExists {
		new_db, _ = sjson.Set(new_db, pathToFile+".rights", 0b1110) // rwr_
	}

	if old_db != "" { // files.json is NOT empty
		result, _ := conflate.FromData([]byte(old_db), []byte(new_db))
		merged, _ := result.MarshalJSON()
		_ = os.WriteFile(db_files, []byte(merged), 0755)
	} else {
		_ = os.WriteFile(db_files, []byte(new_db), 0755)
	}

	c.msg(fmt.Sprintf("You have successfully written text to '%s'", args[1]))
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

	var pathToFile string
	if isFullPath {
		pathToFile = args[1]
	} else {
		pathToFile = filepath.Join(c.actDir, args[1])
	}

	content, _ := os.ReadFile(db_files)
	db := string(content)
	fileRights := gjson.Get(db, pathToFile+".rights").Int()
	switch fileRights & 0b1010 {
	case 0b1010:
		if c.nick != gjson.Get(db, pathToFile+".owner").String() {
			c.msg("DS: You are not the owner of this file.")
			return
		}

		isAllowedToRead := false
		fileGroup := gjson.Get(db, pathToFile+".group").String()
		c.msg(fileGroup)
		for _, group := range c.groups {
			if group == fileGroup {
				isAllowedToRead = true
			}
		}

		if isAllowedToRead == false {
			c.msg(fmt.Sprintf("DS: You are NOT in the group '%s'", fileGroup))
			return
		}

		text, err = os.ReadFile(pathToFile)
		if err != nil { // Couldn't read from file
			c.err(err)
			return
		}

		c.msg(fmt.Sprintf("Text from file '%s':\n%s", args[1], text))

	default:
		c.msg("DS: NOT allowed to read this file due to the rights.")
		return
	}
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
	c.groups = c.groups[:0]

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

	content, _ = os.ReadFile(db_files)
	db = string(content)

	files := gjson.Parse(db)

	for k, v := range files.Map() {
		r := gjson.Get(v.Raw, "owner").String()
		if r == nick {
			db, _ = sjson.Delete(db, k)
		}
	}

	_ = os.WriteFile(db_files, []byte(db), 0755)

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

// lab2

func (s *server) addgroup(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if !c.isAdmin {
		c.msg("Only admin can create new groups.")
		return
	}

	if len(args) < 2 {
		c.msg(`Wrong usage. Example: "addgroup [group]"`)
		return
	}

	group := args[1]
	pathToFile := group_path + group + ".json"
	if _, err := os.Stat(pathToFile); err == nil {
		c.msg(fmt.Sprintf("Group '%s' already exists.", group))
		return
	}

	db, _ := sjson.Set("", "name", group)
	_ = os.MkdirAll(group_path, os.ModePerm)
	_ = os.WriteFile(pathToFile, []byte(db), 0755)

	c.msg(fmt.Sprintf("You have successfully created group '%s'", group))
}

func inGroup(db string, user string) (bool, int) {
	userarr := gjson.Get(db, "users").Array()

	isInGroup := false
	index := -1
	for i := range userarr {
		if userarr[i].String() == user {
			isInGroup = true
			index = i
			break
		}
	}

	return isInGroup, index
}

func (s *server) u2g(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if !c.isAdmin {
		c.msg("Only admin can create new groups.")
		return
	}

	if len(args) < 3 {
		c.msg(`Wrong usage. Example: "u2g [group] [user]"`)
		return
	}

	group := args[1]
	pathToFile := group_path + group + ".json"
	if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
		c.msg(fmt.Sprintf("Group '%s' does NOT exists.", group))
		return
	}

	user := args[2]
	pathToUser := db_path + user + ".json"
	if _, err := os.Stat(pathToUser); errors.Is(err, os.ErrNotExist) {
		c.msg(fmt.Sprintf("User '%s' does NOT exists.", user))
		return
	}

	content, _ := os.ReadFile(pathToFile)
	db := string(content)

	isInGroup, _ := inGroup(db, user)
	if isInGroup {
		c.msg(fmt.Sprintf("User '%s' is already in '%s'. Proceeding nothing", user, group))
		return
	}

	db, _ = sjson.Set(db, "users.-1", user)
	_ = os.WriteFile(pathToFile, []byte(db), 0755)

	c.msg(fmt.Sprintf("You have successfully added '%s' to group '%s'", user, group))
}

func (s *server) trimgroup(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if !c.isAdmin {
		c.msg("Only admin can create new groups.")
		return
	}

	if len(args) < 3 {
		c.msg(`Wrong usage. Example: "trimgroup [group] [user]"`)
		return
	}

	group := args[1]
	pathToFile := group_path + group + ".json"
	if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
		c.msg(fmt.Sprintf("Group '%s' does NOT exists.", group))
		return
	}

	user := args[2]
	pathToUser := db_path + user + ".json"
	if _, err := os.Stat(pathToUser); errors.Is(err, os.ErrNotExist) {
		c.msg(fmt.Sprintf("User '%s' does NOT exists.", user))
		return
	}

	content, _ := os.ReadFile(pathToFile)
	db := string(content)

	isInGroup, index := inGroup(db, user)
	if !isInGroup {
		c.msg(fmt.Sprintf("There's no '%s' in group '%s'. Proceeding nothing", user, group))
		return
	}

	db, _ = sjson.Delete(db, fmt.Sprintf("users.%d", index))
	_ = os.WriteFile(pathToFile, []byte(db), 0755)

	c.msg(fmt.Sprintf("You have successfully removed '%s' from group '%s'", user, group))
}

func (s *server) rmgroup(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if !c.isAdmin {
		c.msg("Only admin can remove users.")
		return
	}

	if len(args) < 2 {
		c.msg(`Wrong usage. Example: "rmgroup [group]"`)
		return
	}

	group := args[1]
	pathToFile := group_path + group + ".json"
	if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
		c.msg(fmt.Sprintf("Group '%s' does NOT exists.", group))
		return
	}

	err := os.Remove(pathToFile)
	if err != nil {
		log.Printf(err.Error())
		c.err(err)
		return
	}

	c.msg(fmt.Sprintf("You have successfully removed the group '%s'", group))
}

func (s *server) rr(c *client, args []string) {
	if len(args) < 2 {
		c.msg(`Wrong usage. Example: "rr [file]"`)
		return
	}

	isFullPath := strings.HasPrefix(args[1], users_path)

	var pathToFile string
	if isFullPath {
		pathToFile = args[1]
	} else {
		pathToFile = filepath.Join(c.actDir, args[1])
	}

	content, err := os.ReadFile(db_files)
	if err != nil {
		c.msg("Couldn't load database of files.")
		log.Printf(err.Error())
		return
	}
	db := string(content)
	info := gjson.Get(db, pathToFile).String()

	if info == "" {
		c.msg("No such file in the database")
		return
	}

	c.msg(fmt.Sprintf("File info of '%s':\n%s", pathToFile, info))
}

func (s *server) chmod(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if len(args) < 3 {
		c.msg(`Wrong usage. Example: "chmod [file] [rwrw]" for user and group.`)
		return
	}

	rights := args[2]
	isFullPath := strings.HasPrefix(args[1], users_path)

	var pathToFile string
	if isFullPath {
		pathToFile = args[1]
	} else {
		pathToFile = filepath.Join(c.actDir, args[1])
	}

	content, _ := os.ReadFile(db_files)
	db := string(content)
	if !gjson.Get(db, pathToFile).Exists() {
		c.msg("DB: There is no such file in the database.")
		return
	}

	owner := gjson.Get(db, pathToFile+".owner").String()
	if c.nick != owner {
		c.msg("You are not the owner of this file.")
		return
	}

	irights, _ := strconv.ParseInt(rights, 2, 5)
	db, _ = sjson.Set(db, pathToFile+".rights", irights)

	_ = os.WriteFile(db_files, []byte(db), 0755)

	c.msg(fmt.Sprintf("You have successfully changed rights for '%s'", pathToFile))
}
