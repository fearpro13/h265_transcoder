package main

import (
	"fearpro13/h265_transcoder"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	os.Exit(run())
}

func run() int {
	osig := make(chan os.Signal, 1)
	signal.Notify(osig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	instance := h265_transcoder.NewInstance(":9222", ":8222")

	err := instance.Start()
	if err != nil {
		println(err)

		return 1
	}

	<-osig

	err = instance.Stop()
	if err != nil {
		println(err)
		return 1
	}

	return 0
}
