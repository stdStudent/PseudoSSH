package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/miracl/conflate"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const users_path string = "users/"
const db_path string = "db/"
const group_path = "group/"
const db_files = "files/files.json"
const audits_path = "audits/"

const iba = "isBeingAudited"
const aoa = "amountOfAudits"

const std_mark uint64 = 50

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

		// lab3
		case CmdAppend:
			s.append(cmd.client, cmd.args)

		case CmdChMark:
			s.chmark(cmd.client, cmd.args)

		case CmdGM:
			s.gm(cmd.client, cmd.args)

		// lab4
		case CmdWatch:
			s.watch(cmd.client, cmd.args)
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
		c.msg(`A nick and a password are required. Example: "reg [nick] [pswd] {cm}"`)
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

	c.cm, _ = getMark(args, "")

	db, _ := sjson.Set("", "nick", nick)
	db, _ = sjson.Set(db, "pswd", pswd)
	db, _ = sjson.Set(db, "cm", c.cm)

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

func appendGroups(c *client) {
	matches, _ := filepath.Glob(filepath.Join(group_path, "*.json"))
	for _, file := range matches {
		content, _ := os.ReadFile(file)
		db := string(content)
		isInGroup, _ := inGroup(db, c.nick)
		if isInGroup {
			c.groups = append(c.groups, gjson.Get(db, "name").String())
		}
	}
}

func (s *server) login(c *client, args []string) {
	if len(args) < 3 {
		c.msg(`A nick and a password are required. Example: "login [nick] [pswd] {cm}"`)
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
	c.isBeingAudited = gjson.Get(db, iba).Bool()

	isActive := gjson.Get(db, "isActive")
	if isActive.Bool() {
		c.msg("This user is already logged in.")

		c.loginAttempts++
		writeAudit(c, db, fmt.Sprintf("Failed relogin from '%s'. Attempt #%d", getIP(c), c.loginAttempts))

		return
	}

	pswd := gjson.Get(db, "pswd")
	if pswd.String() != c.pswd {
		c.msg("Wrong password.")

		c.loginAttempts++
		writeAudit(c, db, fmt.Sprintf("Failed login from '%s'. Attempt #%d", getIP(c), c.loginAttempts))

		return
	} else {
		c.isLoggedIn = true
		c.isAdmin = gjson.Get(db, "isAdmin").Bool()
		c.isAudit = gjson.Get(db, "isAudit").Bool()

		mark, mErr := getMark(args, db)
		if mErr != nil {
			c.err(mErr)
			return
		}
		c.cm = mark

		db, _ = sjson.Set(db, "isActive", true)

		err := os.WriteFile(pathToFile, []byte(db), 0755)
		if err != nil {
			log.Printf("Could NOU open file '%s'", db_path+c.nick+".json")
		}

		c.actDir = users_path + c.nick + "/home"
		c.homeDir = "/home"
		c.currDir = c.homeDir

		appendGroups(c)

		c.msg("You have successfully logged in.")
		log.Printf("A user '%s' has connected.", c.nick)

		c.loginAttempts++
		writeAudit(c, db, fmt.Sprintf("Success login from '%s'. Attempt #%d", getIP(c), c.loginAttempts))

		c.loginAttempts = 0 // success login
	}
}

func removeLines(fn string, start, n int64) (err error) {
	if start < 1 {
		return errors.New("invalid request.  line numbers start at 1.")
	}
	if n < 0 {
		return errors.New("invalid request.  negative number to remove.")
	}
	var f *os.File
	if f, err = os.OpenFile(fn, os.O_RDWR, 0); err != nil {
		return
	}
	defer func() {
		if cErr := f.Close(); err == nil {
			err = cErr
		}
	}()
	var b []byte
	if b, err = ioutil.ReadAll(f); err != nil {
		return
	}
	cut, ok := skip(b, start-1)
	if !ok {
		return fmt.Errorf("less than %d lines", start)
	}
	if n == 0 {
		return nil
	}
	tail, ok := skip(cut, n)
	if !ok {
		return fmt.Errorf("less than %d lines after line %d", n, start)
	}
	t := int64(len(b) - len(cut))
	if err = f.Truncate(t); err != nil {
		return
	}
	if len(tail) > 0 {
		_, err = f.WriteAt(tail, t)
	}
	return
}

func skip(b []byte, n int64) ([]byte, bool) {
	for ; n > 0; n-- {
		if len(b) == 0 {
			return nil, false
		}
		x := bytes.IndexByte(b, '\n')
		if x < 0 {
			x = len(b)
		} else {
			x++
		}
		b = b[x:]
	}
	return b, true
}

func writeAudit(c *client, db string, msg string) {
	if c.isBeingAudited {
		auditFile := audits_path + c.nick
		f, _ := os.OpenFile(auditFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)

		aoa := gjson.Get(db, aoa).Int()
		if aoa != 0 {
			file, _ := os.Open(auditFile)
			var amount int64 = 0
			fileScanner := bufio.NewScanner(file)
			for fileScanner.Scan() {
				amount++
			}
			_ = file.Close()

			if amount > aoa {
				_ = removeLines(auditFile, 1, amount-aoa+1)
			} else if amount == aoa {
				_ = removeLines(auditFile, 1, 1)
			}
		}

		_, _ = f.WriteString(fmt.Sprintf("%s: %s: %s.\n", time.Now(), c.nick, msg))
		_ = f.Close()
	}
}

func getIP(c *client) string {
	if addr, ok := c.conn.RemoteAddr().(*net.TCPAddr); ok {
		return addr.IP.String()
	} else {
		return "invalid_ip"
	}
}

func getMark(args []string, db string) (uint64, error) {
	if db == "" {
		if len(args) > 3 {
			cm, err := strconv.ParseUint(args[3], 10, 32)
			if err != nil {
				return 0, errors.New("mark must be >= 0")
			}

			return cm, nil
		}

		/* default */
		return std_mark, nil
	}

	var mark uint64 = gjson.Get(db, "cm").Uint()
	isMarkExists := gjson.Get(db, "cm").Exists()
	if len(args) > 3 && isMarkExists {
		cm, err := strconv.ParseUint(args[3], 10, 32)
		if err != nil {
			return 0, errors.New("mark must be >= 0")
		}

		if cm > mark {
			return 0, errors.New(fmt.Sprintf("mark cannot be larger than '%d'", mark))
		}
		mark = cm
	}
	return mark, nil
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

	pathToFile, err := getPathToFile(c, args[1])
	if err != nil {
		c.err(err)
		return
	}

	content, _ := os.ReadFile(db_files)
	old_db := string(content)

	c.groups = c.groups[:0]
	appendGroups(c)

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

			markOfFile := gjson.Get(old_db, pathToFile+".cm").Uint()
			contentOfGroup, _ := os.ReadFile(group_path + fileGroup + ".json")
			db_group := string(contentOfGroup)
			markOfGroup := gjson.Get(db_group, "cm").Uint()

			if !(markOfGroup == markOfFile) {
				c.msg(fmt.Sprintf("MS: '%s':'%d' must be == '%d' of the file.", fileGroup, markOfGroup, markOfFile))
				return
			}

			if !(c.cm == markOfFile) {
				c.msg(fmt.Sprintf("MS: Your mark '%d' must equal to the file's mark '%d'", c.cm, markOfFile))
				return
			}

			/* Success */
			err := os.WriteFile(pathToFile, []byte(strings.Join(args[2:], " ")), 0755)
			if err != nil {
				c.err(err)
				return
			}

		default:
			c.msg("DS: NOT allowed to write to this file due to the rights.")
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
	new_db, _ = sjson.Set(new_db, pathToFile+".cm", std_mark)

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

	c.groups = c.groups[:0]
	appendGroups(c)

	pathToFile, fErr := getPathToFile(c, args[1])
	if fErr != nil {
		c.err(fErr)
		return
	}

	var err error
	var text []byte

	content, _ := os.ReadFile(db_files)
	db := string(content)

	if !gjson.Get(db, pathToFile).Exists() {
		c.msg("DB: There is no such file in the database.")
		return
	}

	fileRights := gjson.Get(db, pathToFile+".rights").Int()
	switch fileRights & 0b1010 {
	case 0b1010:
		isAllowedToRead := false
		fileGroup := gjson.Get(db, pathToFile+".group").String()
		for _, group := range c.groups {
			if group == fileGroup {
				isAllowedToRead = true
			}
		}

		if isAllowedToRead == false {
			c.msg(fmt.Sprintf("DS: You are NOT in the group '%s'", fileGroup))
			return
		}

		markOfFile := gjson.Get(db, pathToFile+".cm").Uint()
		contentOfGroup, _ := os.ReadFile(group_path + fileGroup + ".json")
		db_group := string(contentOfGroup)
		markOfGroup := gjson.Get(db_group, "cm").Uint()

		if !(markOfGroup >= markOfFile) {
			c.msg(fmt.Sprintf("MS: '%s':'%d' must be >= '%d' of the file.", fileGroup, markOfGroup, markOfFile))
			return
		}

		if !(c.cm >= markOfFile) {
			c.msg(fmt.Sprintf("MS: Your mark '%d' must be >= the mark '%d' of the file.", c.cm, markOfFile))
			return
		}

		/* Success */
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

	path, err := getPathToFile(c, args[1])
	if err != nil {
		c.err(err)
		return
	}

	files, err := os.ReadDir(path)
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
		c.msg(`Wrong usage. Example: "addgroup [group] "mark" {mark}"`)
		return
	}

	group := args[1]
	pathToFile := group_path + group + ".json"
	if _, err := os.Stat(pathToFile); err == nil {
		c.msg(fmt.Sprintf("Group '%s' already exists.", group))
		return
	}

	mark, mErr := getMark(args, "")
	if mErr != nil {
		c.err(mErr)
		return
	}

	db, _ := sjson.Set("", "name", group)
	db, _ = sjson.Set(db, "cm", mark)
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

	pathToFile, err := getPathToFile(c, args[1])
	if err != nil {
		c.err(err)
		return
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
	pathToFile, err := getPathToFile(c, args[1])
	if err != nil {
		c.err(err)
		return
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

func getPathToFile(c *client, arg string) (string, error) {
	isFullPath := strings.HasPrefix(arg, users_path)
	if isFullPath {
		return arg, nil
	} else {
		if strings.HasPrefix(arg, "../../..") {
			return "", errors.New("cannot go higher than the root directory")
		}
		return filepath.Join(c.actDir, arg), nil
	}
}

// lab3
func (s *server) append(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if len(args) < 3 {
		c.msg(`Wrong usage. Example: "append [filename] [text]"`)
		return
	}

	pathToFile, err := getPathToFile(c, args[1])
	if err != nil {
		c.err(err)
		return
	}

	content, _ := os.ReadFile(db_files)
	old_db := string(content)

	c.groups = c.groups[:0]
	appendGroups(c)

	isExists := false
	if _, err := os.Stat(pathToFile); err == nil {
		isExists = true
	}

	f, err := os.OpenFile(pathToFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		c.err(err)
		return
	}
	defer f.Close()

	if !isExists {
		c.msg(fmt.Sprintf("File '%s' does NOT exists.", pathToFile))
		return
	} else {
		fileRights := gjson.Get(old_db, pathToFile+".rights").Int()
		switch fileRights & 0b0101 {
		case 0b0101:
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

			markOfFile := gjson.Get(old_db, pathToFile+".cm").Uint()
			contentOfGroup, _ := os.ReadFile(group_path + fileGroup + ".json")
			db_group := string(contentOfGroup)
			markOfGroup := gjson.Get(db_group, "cm").Uint()

			if !(markOfGroup <= markOfFile) {
				c.msg(fmt.Sprintf("MS: '%s':'%d' must be <= '%d' of the file.", fileGroup, markOfGroup, markOfFile))
				return
			}

			if !(c.cm <= markOfFile) {
				c.msg(fmt.Sprintf("MS: Your mark '%d' must be <= the mark '%d' of the file.", c.cm, markOfFile))
				return
			}

			/* Success */
			_, err := f.WriteString(strings.Join(args[2:], " "))
			if err != nil {
				c.err(err)
				return
			}

		default:
			c.msg("DS: NOT allowed to read this file due to the rights.")
			return
		}
	}

	c.msg(fmt.Sprintf("You have successfully appended text to '%s'", pathToFile))
}

func (s *server) chmark(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if len(args) < 4 {
		c.msg(`Wrong usage. Example: "chmark (f|u|g) [object] [mark]"`)
		return
	}

	mod := args[1]
	object := args[2]
	mark, mErr := getMark(args, "")
	if mErr != nil {
		c.err(mErr)
		return
	}

	switch mod {
	case "f":
		if mark > c.cm {
			c.msg(fmt.Sprintf("New mark '%d' can't be higher than your current mark: '%d'", mark, c.cm))
			return
		}

		content, _ := os.ReadFile(db_files)
		old_db := string(content)

		pathToFile, err := getPathToFile(c, object)
		if err != nil {
			c.err(err)
			return
		}

		if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
			c.msg(fmt.Sprintf("File '%s' does NOT exists.", pathToFile))
			return
		}

		owner := gjson.Get(old_db, pathToFile+".owner").String()
		if c.nick != owner {
			c.msg("You are not the owner of this file.")
			return
		}

		new_db, _ := sjson.Set("", pathToFile+".cm", mark)
		result, _ := conflate.FromData([]byte(old_db), []byte(new_db))
		merged, _ := result.MarshalJSON()
		_ = os.WriteFile(db_files, []byte(merged), 0755)

	case "u":
		if c.isAdmin {
			if c.nick == object {
				pathToFile := db_path + c.nick + ".json"
				content, _ := os.ReadFile(pathToFile)
				db := string(content)

				c.cm, mErr = getMark(args, db)
				if mErr != nil {
					c.err(mErr)
					return
				}
			} else {
				pathToFile := db_path + object + ".json"
				if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
					c.msg(fmt.Sprintf("User '%s' does NOT exists.", object))
					return
				}

				content, _ := os.ReadFile(pathToFile)
				db := string(content)

				db, _ = sjson.Set(db, "cm", mark)
				_ = os.WriteFile(pathToFile, []byte(db), 0755)
			}
		} else {
			if c.nick != object {
				c.msg(fmt.Sprintf("You are NOT '%s'", object))
				return
			}

			pathToFile := db_path + c.nick + ".json"
			if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
				c.msg(fmt.Sprintf("User '%s' does NOT exists.", c.nick))
				return
			}

			content, _ := os.ReadFile(pathToFile)
			db := string(content)

			c.cm, mErr = getMark(args, db)
			if mErr != nil {
				c.err(mErr)
				return
			}
		}

	case "g":
		if !c.isAdmin {
			c.msg("Only admin can change mark for groups.")
			return
		}

		pathToFile := group_path + object + ".json"
		if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
			c.msg(fmt.Sprintf("Group '%s' does NOT exists.", object))
			return
		}

		content, _ := os.ReadFile(pathToFile)
		db := string(content)

		db, _ = sjson.Set(db, "cm", mark)
		_ = os.WriteFile(pathToFile, []byte(db), 0755)

	default:
		c.msg("First option must be either of 'f', 'u', 'g'")
		return
	}

	c.msg(fmt.Sprintf("You have successfully changed mark for '%s'", object))
}

func (s *server) gm(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if len(args) < 3 {
		c.msg(`Wrong usage. Example: "gm (f|u|g) {object}"`)
		return
	}

	mod := args[1]
	object := args[2]

	switch mod {
	case "f":
		content, _ := os.ReadFile(db_files)
		db := string(content)

		pathToFile, err := getPathToFile(c, object)
		if err != nil {
			c.err(err)
			return
		}

		if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
			c.msg(fmt.Sprintf("File '%s' does NOT exists.", pathToFile))
			return
		}

		/*owner := gjson.Get(old_db, pathToFile+".owner").String()
		if c.nick != owner {
			c.msg("You are not the owner of this file.")
			return
		}*/

		markOfFile := gjson.Get(db, pathToFile+".cm").Uint()
		c.msg(fmt.Sprintf("Mark of file '%s' is '%d'", pathToFile, markOfFile))

	case "u":
		if c.nick == object {
			c.msg(fmt.Sprintf("Your current mark is '%d'", c.cm))
			return
		}

		if !c.isAdmin {
			c.msg("Only admin can see other's max mark")
			return
		}

		pathToFile := db_path + object + ".json"
		if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
			c.msg(fmt.Sprintf("User '%s' does NOT exists.", object))
			return
		}

		content, _ := os.ReadFile(pathToFile)
		db := string(content)

		markOfUser := gjson.Get(db, "cm").Uint()
		c.msg(fmt.Sprintf("Max mark of user '%s' is '%d'", object, markOfUser))

	case "g":
		pathToFile := group_path + object + ".json"
		if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
			c.msg(fmt.Sprintf("Group '%s' does NOT exists.", object))
			return
		}

		content, _ := os.ReadFile(pathToFile)
		db := string(content)

		markOfGroup := gjson.Get(db, "cm").Uint()
		c.msg(fmt.Sprintf("Mark of group '%s' is '%d'", object, markOfGroup))

	default:
		c.msg("First option must be either of 'f', 'u', 'g'")
		return
	}
}

// lab4
func (s *server) watch(c *client, args []string) {
	if !c.isLoggedIn {
		c.msg("You must log in first.")
		return
	}

	if !c.isAudit {
		c.msg("Only audit can watch.")
		return
	}

	if len(args) < 3 {
		c.msg(`Wrong usage. Example: "audit (f|u|g) {object} [amount]"`)
		return
	}

	mod := args[1]
	object := args[2]
	var amount uint64 = 0
	var err error
	if len(args) > 3 {
		amount, err = strconv.ParseUint(args[3], 10, 32)
		if err != nil {
			c.msg("(amount) must be >= 0")
			return
		}
	}

	switch mod {
	case "u":
		pathToFile := db_path + object + ".json"
		if _, err := os.Stat(pathToFile); errors.Is(err, os.ErrNotExist) {
			c.msg(fmt.Sprintf("User '%s' does NOT exists.", object))
			return
		}

		content, _ := os.ReadFile(pathToFile)
		db := string(content)

		boolAudit := !gjson.Get(db, iba).Bool()
		db, _ = sjson.Set(db, iba, boolAudit)
		db, _ = sjson.Set(db, aoa, amount)
		_ = os.WriteFile(pathToFile, []byte(db), 0755)

		c.msg(fmt.Sprintf("Changed audit to '%t' for user '%s'", boolAudit, object))

	default:
		c.msg("First option must be either of 'f', 'u', 'g'")
		return
	}
}
