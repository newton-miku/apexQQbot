package tools

import (
	"fmt"
	"os"
)

var (
	Version   = "undefined"
	BuildTime = "undefined"
)

func PrintVersion() {
	fmt.Printf("Current version: %s\n", Version)
	fmt.Printf("Build time: %s\n", BuildTime)
	os.Exit(0)
}
