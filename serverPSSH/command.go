package main

type commandID int

const (
	CmdNick commandID = iota
	CmdLogin
	CmdPwd
	CmdWrite
	CmdRead
	CmdJoin
	CmdRooms
	CmdMsg
	CmdQuit
)

type command struct {
	id     commandID
	client *client
	args   []string
}
