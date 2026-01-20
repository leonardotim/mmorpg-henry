package main

import (
	"henry/pkg/server"
)

func main() {
	gameServer := server.NewGameServer()
	gameServer.Run(":8080")
}
