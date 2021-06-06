package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"oper/operator"
	"oper/operator/memory"
)

func main() {
	o, err := operator.NewServer(memory.NewStorage())
	if err != nil {
		logrus.WithError(err).Fatal("failed to start")
	}

	exit := make(chan os.Signal)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	<-exit

	o.Close()
}
