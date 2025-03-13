// src/commands/main/ping.go
package commands

import (
	"fmt"
	"hisoka/src/libs"
	"time"
)

func ping(conn *libs.IClient, m *libs.IMessage) bool {
	start := time.Now()
	m.Reply("Pong!")

	elapsed := time.Since(start)
	m.Reply(fmt.Sprintf("Ping: %d ms", elapsed.Milliseconds()))

	return true
}

func init() {
	libs.NewCommands(&libs.ICommand{
		Name:     "ping",
		As:       []string{"ping"},
		Tags:     "main",
		IsPrefix: true,
		Execute:  ping,
	})
}
