package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"oper/accounter"
	"oper/accounter/memory"
)

func main() {
	p, err := accounter.NewServer(memory.NewStorage())
	if err != nil {
		logrus.WithError(err).Fatal("failed to start")
	}

	exit := make(chan os.Signal)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	<-exit

	p.Close()
}
