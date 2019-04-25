package main

import (
	"github.com/k93ndy/logbook/cmd"
)

func main() {
	// cooperate with cobra, command flags have a higher priority
	cmd.Execute()
}
