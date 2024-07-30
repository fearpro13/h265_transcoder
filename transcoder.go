package h265_transcoder

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
)

const (
	StatusStopped = "stopped"
	StatusOk      = "ok"
	StatusError   = "error"
)

var FFMpegPath = "/usr/bin/ffmpeg"

// IdrInterval Deprecated
var IdrInterval uint64 = 60
var TranscodeUseGPU = false

type Source struct {
	id   string
	from url.URL
	to   url.URL
}

type Transcoder struct {
	source  Source
	proc    *exec.Cmd
	status  string
	running atomic.Bool
	ctx     context.Context
	ctxF    context.CancelFunc
	stdErr  io.ReadCloser
}

func NewSource(id string, from string, to string) Source {
	fromParsed, err := url.Parse(from)
	if err != nil {
		log.Fatal(err)
	}

	toParsed, err := url.Parse(to)
	if err != nil {
		log.Fatal(err)
	}

	return Source{
		id:   id,
		from: *fromParsed,
		to:   *toParsed,
	}
}

func NewTranscoder(source Source) *Transcoder {
	return &Transcoder{
		source:  source,
		status:  StatusStopped,
		running: atomic.Bool{},
	}
}

func (t *Transcoder) Start(ctx context.Context) error {
	if t.running.Load() {
		return errors.New("already started")
	}

	// start ffmpeg
	argsStr := fmt.Sprintf("-y -fflags +igndts -rtsp_transport tcp -i %s -c:a copy -c:v libx264 -crf 20 -b:v 500k -max_muxing_queue_size 1024 -bf 0 -f rtsp -rtsp_transport tcp %s", t.source.from.String(), t.source.to.String())

	argsSplit := strings.Split(argsStr, " ")
	cmd := exec.Command(FFMpegPath, argsSplit...)

	stdErr, err := cmd.StderrPipe()
	if err != nil {
		t.status = StatusError

		return err
	}

	t.stdErr = stdErr

	err = cmd.Start()
	if err != nil {
		t.status = StatusError
		return err
	}

	t.proc = cmd
	t.running.Store(true)

	go t.run()

	t.ctx, t.ctxF = context.WithCancel(ctx)

	go func() {
		err := cmd.Wait()

		if err != nil {
			t.status = StatusError
			log.Println(fmt.Sprintf("transcoder #%s(%s): %s", t.source.id, t.source.from.String(), err))
		} else {
			t.status = StatusStopped
		}

		t.ctxF()
	}()

	go func() {
		select {
		case <-t.ctx.Done():
			_ = t.Stop()
		}
	}()

	return nil
}

func (t *Transcoder) run() {
	var line []byte
	var pErr, err error

	t.status = StatusOk

	reader := bufio.NewReader(t.stdErr)

	for t.running.Load() {
		line, _, err = reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				log.Println(fmt.Sprintf("transcoder #%s(%s): %s", t.source.id, t.source.from.String(), err))
			}

			pErr = t.proc.Process.Signal(syscall.Signal(0))
			if pErr != nil && !errors.Is(pErr, syscall.EPERM) {
				log.Println(pErr)
				t.ctxF()
				return
			}
		} else {
			log.Println(fmt.Sprintf("transcoder #%s(%s): %s", t.source.id, t.source.from.String(), line))
		}
	}
}

func (t *Transcoder) Stop() error {
	if !t.running.Load() {
		return errors.New("not running")
	}

	t.running.Store(false)

	t.ctxF()
	_ = t.proc.Process.Release()
	_ = t.proc.Process.Kill()

	return nil
}

func (t *Transcoder) Status() string {
	return t.status
}
