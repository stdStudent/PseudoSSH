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
	CmdRR
	CmdChMod

	//lab3
	CmdAppend
	CmdChMark
	CmdGM
	CmdWatch
)

type command struct {
	id     commandID
	client *client
	args   []string
}
