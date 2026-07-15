package main

import (
	"os"

	server "github.com/saveenergy/openbyte/cmd/server"
)

var version = "dev"

func main() {
	os.Exit(server.Run(os.Args[1:], version))
}
