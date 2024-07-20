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
	"sync/atomic"
	"syscall"
)

const (
	StatusStopped = "stopped"
	StatusOk      = "ok"
	StatusError   = "error"
)

var ffmpegPath = "/usr/bin/ffmpeg"

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

	t.ctx, t.ctxF = context.WithCancel(ctx)
	defer func() {
		t.ctxF()
	}()

	// start ffmpeg
	cmd := exec.Command(ffmpegPath, "-i", t.source.from.String(), "-c:v", "h264", "-f", "rtsp", t.source.to.String())

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

	go func() {
		select {
		case <-ctx.Done():
			_ = t.Stop()
		}
	}()

	go func() {
		_ = cmd.Wait()
		_ = t.Stop()
	}()

	return nil
}

func (t *Transcoder) run() {
	defer func() {
		_ = t.Stop()
	}()

	var line []byte
	var pErr, err error

	t.status = StatusOk

	reader := bufio.NewReader(t.stdErr)

	for t.running.Load() {
		line, _, err = reader.ReadLine()
		if err != nil {
			log.Println(fmt.Sprintf("transcoder #%d(%s): %s", t.source.id, t.source.from.String(), err))

			pErr = t.proc.Process.Signal(syscall.Signal(0))
			if pErr != nil && !errors.Is(pErr, syscall.EPERM) {
				return
			}
		} else {
			log.Println(fmt.Sprintf("transcoder #%d(%s): %s", t.source.id, t.source.from.String(), line))
		}
	}
}

func (t *Transcoder) Stop() error {
	if !t.running.Load() {
		return errors.New("not running")
	}

	t.running.Store(false)

	_ = t.proc.Process.Kill()

	err := t.proc.Wait()

	if err != nil {
		t.status = StatusError
		log.Println(fmt.Sprintf("transcoder #%d(%s): %s", t.source.id, t.source.from.String(), err))
	} else {
		t.status = StatusStopped
	}

	_ = t.proc.Process.Release()

	return err
}

func (t *Transcoder) Status() string {
	return t.status
}
