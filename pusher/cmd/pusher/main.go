package main

import (
	"os"
	"os/signal"
	"syscall"

	"oper/pusher"
	"oper/pusher/dummy"
)

func main() {
	p, err := pusher.NewServer(dummy.NewMessagePusher())
	if err != nil {
		panic(err)
	}

	exit := make(chan os.Signal)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	<-exit

	p.Close()
}
