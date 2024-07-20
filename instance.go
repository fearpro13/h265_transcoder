package h265_transcoder

import (
	"context"
	"errors"
	"fearpro13/h265_transcoder/mediamtx/core"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Unit struct {
	id         string
	transcoder *Transcoder
	path       Source
}

type Instance struct {
	rtspHandler       *core.RtspHandler
	httpHandler       *ControlServer
	units             map[string]Unit
	running           atomic.Bool
	ctx               context.Context
	retryAfterSeconds int
}

func NewInstance(rtspAddr string, httpAddr string, retryAfterSeconds int) *Instance {
	ctx := context.Background()
	return &Instance{
		rtspHandler:       core.NewRtspHandler(ctx, rtspAddr),
		httpHandler:       NewControlServer(httpAddr),
		units:             map[string]Unit{},
		running:           atomic.Bool{},
		ctx:               ctx,
		retryAfterSeconds: retryAfterSeconds,
	}
}

func (instance *Instance) Start() error {
	if instance.running.Load() {
		return errors.New("instance already running")
	}

	err := instance.rtspHandler.Start()

	if err != nil {
		return err
	}

	instance.httpHandler.OnCreate = instance.AddUnit
	instance.httpHandler.OnStop = instance.RemoveUnit

	instance.httpHandler.OnStatus = func(id string) map[string]any {
		u := instance.GetUnit(id)
		if u == nil {
			return nil
		}

		return map[string]any{
			"original": u.path.from.String(),
			"source":   u.path.to.String(),
			"status":   u.transcoder.status,
		}
	}

	instance.httpHandler.OnStatusAll = func() map[string]any {
		res := make(map[string]any, len(instance.units))

		for id, u := range instance.units {
			res[id] = map[string]any{
				"original": u.path.from.String(),
				"source":   u.path.to.String(),
				"status":   u.transcoder.status,
			}
		}

		return res
	}

	err = instance.httpHandler.Start()
	if err != nil {
		return err
	}

	instance.running.Store(true)

	go instance.run()

	return nil
}

func (instance *Instance) run() {
	if instance.retryAfterSeconds > 0 {
		ticker := time.NewTicker(time.Duration(instance.retryAfterSeconds) * time.Second)
		defer func() {
			ticker.Stop()
		}()

		for instance.running.Load() {
			select {
			case <-ticker.C:
				if !instance.running.Load() {
					return
				}
			case <-instance.ctx.Done():
				return
			}

			for _, u := range instance.units {
				t := u.transcoder
				if t == nil {
					continue
				}

				if t.status != StatusOk {
					_ = t.Start(instance.ctx)
				}
			}
		}
	}
}

func (instance *Instance) Stop() error {
	if !instance.running.Load() {
		return errors.New("instance not running")
	}

	instance.running.Store(false)

	_ = instance.httpHandler.Stop()

	wg := sync.WaitGroup{}
	wg.Add(len(instance.units))

	for _, unit := range instance.units {
		go func() {
			_ = unit.transcoder.Stop()
			wg.Done()
		}()
	}

	wg.Wait()

	instance.rtspHandler.Stop()

	return nil
}

func (instance *Instance) GetUnit(id string) *Unit {
	u, exist := instance.units[id]
	if !exist {
		return nil
	}

	return &u
}

func (instance *Instance) AddUnit(id string, fromSource string) error {
	_, exist := instance.units[id]
	if exist {
		return errors.New("unit already exists")
	}

	path := NewSource(id, fromSource, fmt.Sprintf("rtsp://0.0.0.0:9222/%s", id))

	err := instance.rtspHandler.AddPath(id)
	if err != nil {
		return err
	}

	tc := NewTranscoder(path)

	err = tc.Start(instance.ctx)
	if err != nil {
		return err
	}

	instance.units[id] = Unit{
		id:         id,
		transcoder: tc,
		path:       path,
	}

	return nil
}

func (instance *Instance) RemoveUnit(id string) error {
	u, exist := instance.units[id]
	if !exist {
		return errors.New("unit does not exist")
	}

	_ = u.transcoder.Stop()

	_ = instance.rtspHandler.RemovePath(id)

	delete(instance.units, id)

	return nil
}
