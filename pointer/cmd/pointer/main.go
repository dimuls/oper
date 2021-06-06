package main

import (
	"os"
	"os/signal"
	"syscall"

	"oper/pointer"
	"oper/pointer/memory"
)

func main() {
	p, err := pointer.NewServer(memory.NewStorage())
	if err != nil {
		panic(err)
	}

	exit := make(chan os.Signal)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	<-exit

	p.Close()
}
