package main

type commandID int

const (
	CmdReg commandID = iota
	CmdChPswd
	CmdLogin
	CmdPwd
	CmdWrite
	CmdRead
	CmdLs
	CmdLogout
	CmdHelp
	CmdRmUser
	CmdQuit
)

type command struct {
	id     commandID
	client *client
	args   []string
}
