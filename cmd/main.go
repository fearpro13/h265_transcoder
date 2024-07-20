package main

import (
	"fearpro13/h265_transcoder"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	gpuArg := flag.Bool("gpu", false, "Will use gpu hw acceleration(NVIDIA only)")
	flag.Parse()

	argc := len(os.Args)

	if argc > 1 {
		argv1 := os.Args[1]

		if argv1 == "h" || argv1 == "help" || argv1 == "-help" || argv1 == "--help" {
			flag.PrintDefaults()
			os.Exit(0)
		}
	}

	if *gpuArg {
		log.Println("Using GPU HW Acceleration")
		h265_transcoder.TranscodeUseGPU = true

		panic("Currently not supported")
	}

	os.Exit(run())
}

func run() int {
	osig := make(chan os.Signal, 1)
	signal.Notify(osig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	instance := h265_transcoder.NewInstance(":9222", ":8222", 10)

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
