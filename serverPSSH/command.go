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
	CmdLsUsers
	CmdQuit

	//lab2
	CmdAddGroup
	CmdU2G
	CmdTrimGroup
	CmdRmGroup
)

type command struct {
	id     commandID
	client *client
	args   []string
}
