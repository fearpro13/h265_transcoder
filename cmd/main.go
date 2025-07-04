package main

import (
	"context"
	"errors"
	"fearpro13/h265_transcoder"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	gpuArg := flag.Bool("gpu", false, "Will use gpu hw acceleration(NVIDIA only, NOT IMPLEMENTED)")
	rtspPort := flag.Uint64("rtsp_port", 9222, "Rtsp listening port")
	httpPort := flag.Uint64("http_port", 8222, "Http listening port")
	ffmpegPath := flag.String("ex", "", "ffmpeg executable path")
	udpFlag := flag.Bool("udp", false, "allow udp usage")

	flag.Parse()

	argc := len(os.Args)

	if argc > 1 {
		argv1 := os.Args[1]

		if argv1 == "h" || argv1 == "help" || argv1 == "-help" || argv1 == "--help" {
			flag.PrintDefaults()
			os.Exit(0)
		}
	}

	os.Exit(run(*rtspPort, *httpPort, *ffmpegPath, *gpuArg, *udpFlag))
}

func run(rtspPort uint64, httpPort uint64, ffmpegPath string, useGpu bool, allowUdp bool) int {
	if useGpu {
		log.Println("Using GPU HW Acceleration")
		h265_transcoder.TranscodeUseGPU = true

		log.Println("GPU HW acceleration currently not supported")
		return 1
	}

	if allowUdp {
		log.Println("Rtsp server UDP connections are enabled")
	} else {
		log.Println("Rtsp server UDP connections are disabled")
	}

	ffmpegPath = strings.TrimSpace(ffmpegPath)
	if ffmpegPath == "" {
		log.Println("ffmpeg executable path is required")
		return 1
	} else {
		h265_transcoder.FFMpegPath = ffmpegPath

		err := testRunFFMpeg(ffmpegPath)
		if err != nil {
			log.Println(err)
			return 1
		} else {
			log.Println(fmt.Sprintf("ffmpeg '%s' test run completed successfully", ffmpegPath))
		}
	}

	osig := make(chan os.Signal, 1)
	signal.Notify(osig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	ctx, ctxF := context.WithCancel(context.Background())
	defer ctxF()

	instance := h265_transcoder.NewInstance(ctx, uint16(rtspPort), uint16(httpPort), 10, allowUdp)

	err := instance.Start()
	if err != nil {
		log.Println(err)

		return 1
	}

	select {
	case <-osig:
	case <-instance.Done:
	}

	return 0
}

func testRunFFMpeg(path string) error {
	var err error
	testCmd := exec.Command(path, "-version")
	err = testCmd.Start()
	if err != nil {
		return err
	}

	timer := time.NewTimer(5 * time.Second)
	startRes := make(chan error, 1)

	go func() {
		startRes <- testCmd.Wait()
	}()

	go func() {
		select {
		case <-timer.C:
			startRes <- errors.New(fmt.Sprintf("could not start '%s' - timeout reached", path))
			_ = testCmd.Process.Kill()
		case startRes <- <-startRes:
			timer.Stop()
		}
	}()

	return <-startRes
}
