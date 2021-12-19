package main

import (
	"os"
)

func main() {
	app := NewApp()
	if len(os.Args) == 2 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		os.Args = []string{"App", "version"}
	}
	app.Run()
}
