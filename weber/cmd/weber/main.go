package main

import (
	"oper/weber"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ws, err := weber.NewServer()
	if err != nil {
		panic(err)
	}

	exit := make(chan os.Signal)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	<-exit

	ws.Close()
}
