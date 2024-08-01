package h265_transcoder

import (
	"context"
	"errors"
	"fearpro13/h265_transcoder/mediamtx/core"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type Unit struct {
	id   string
	path Source
}

type Instance struct {
	rtspHandler       *core.RtspHandler
	httpHandler       *ControlServer
	transcoders       map[string]*Transcoder
	units             map[string]Unit
	running           atomic.Bool
	ctx               context.Context
	ctxF              context.CancelFunc
	Done              <-chan struct{}
	retryAfterSeconds int
	allowUdp          bool
	m                 sync.Mutex
}

func NewInstance(pCtx context.Context, rtspPort uint16, httpPort uint16, retryAfterSeconds int, allowUdp bool) *Instance {
	ctx, ctxF := context.WithCancel(pCtx)
	return &Instance{
		rtspHandler:       core.NewRtspHandler(ctx, rtspPort, allowUdp),
		httpHandler:       NewControlServer(ctx, httpPort),
		transcoders:       map[string]*Transcoder{},
		units:             map[string]Unit{},
		running:           atomic.Bool{},
		ctx:               ctx,
		ctxF:              ctxF,
		Done:              ctx.Done(),
		retryAfterSeconds: retryAfterSeconds,
		allowUdp:          allowUdp,
		m:                 sync.Mutex{},
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

		t, te := instance.transcoders[id]

		var status string
		if !te || t == nil {
			return nil
		} else {
			status = t.Status()
		}

		return map[string]any{
			"original": u.path.from.String(),
			"source":   u.path.to.String(),
			"status":   status,
		}
	}

	instance.httpHandler.OnStatusAll = func() map[string]any {
		res := make(map[string]any, len(instance.units))

		for id, u := range instance.units {
			t, te := instance.transcoders[id]

			var status string
			if !te || t == nil {
				return nil
			} else {
				status = t.Status()
			}

			res[id] = map[string]any{
				"original": u.path.from.String(),
				"source":   u.path.to.String(),
				"status":   status,
			}
		}

		return res
	}

	err = instance.httpHandler.Start()
	if err != nil {
		return err
	}

	instance.running.Store(true)

	go func() {
		select {
		case <-instance.ctx.Done():
			_ = instance.Stop()
		}
	}()

	go instance.run()

	return nil
}

func (instance *Instance) run() {
	if instance.retryAfterSeconds > 0 {
		ticker := time.NewTicker(time.Duration(instance.retryAfterSeconds) * time.Second)
		defer func() {
			ticker.Stop()
			instance.ctxF()
		}()

		for instance.running.Load() {
			select {
			case <-ticker.C:
				if !instance.running.Load() {
					return
				}
			case <-instance.ctx.Done():
				return
			case <-instance.httpHandler.Done:
				return
			case <-instance.rtspHandler.Done:
				return
			}

			for _, u := range instance.units {
				t, te := instance.transcoders[u.id]

				if !te || t == nil || t.status != StatusOk || !instance.rtspHandler.PathExist(u.id) {
					var us string
					if !te || t == nil {
						us = "transcoder is stopped, missing or broken"
					} else if !instance.rtspHandler.PathExist(u.id) {
						us = fmt.Sprintf("receiving rtsp path(%s) is stoppped, missing or broken", u.path.to.String())
					} else if t.status != StatusOk {
						us = "transcoder is stopped"
					}

					log.Printf("unit #%s: %s, restarting unit\n", u.id, us)

					err := instance.RestartUnit(u)

					if err != nil {
						log.Printf("unit #%s: unit restart failed: %s\n", u.id, err.Error())
					}
					continue
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
			t, te := instance.transcoders[unit.id]
			if te && t != nil {
				_ = t.Stop()
			}
			wg.Done()
		}()
	}

	wg.Wait()

	instance.rtspHandler.Stop()

	instance.ctxF()

	return nil
}

func (instance *Instance) GetUnit(id string) *Unit {
	u, exist := instance.units[id]
	if !exist {
		return nil
	}

	return &u
}

func (instance *Instance) AddUnit(id string, fromSource string) (Source, error) {
	path := NewSource(id, fromSource, fmt.Sprintf("rtsp://0.0.0.0%s/%s", instance.rtspHandler.RtspAddr, id))

	_, exist := instance.units[id]
	if exist {
		return path, errors.New("unit already exists")
	}

	err := instance.rtspHandler.AddPath(id)
	if err != nil {
		return path, err
	}

	ptc, e := instance.transcoders[id]
	if e && ptc != nil {
		_ = ptc.Stop()
	}

	tc := NewTranscoder(path)

	err = tc.Start(instance.ctx)
	if err != nil {
		return path, err
	}

	instance.m.Lock()
	instance.transcoders[id] = tc

	instance.units[id] = Unit{
		id:   id,
		path: path,
	}

	instance.m.Unlock()

	return path, nil
}

func (instance *Instance) RemoveUnit(id string) error {
	_, exist := instance.units[id]
	if !exist {
		return errors.New("unit does not exist")
	}

	t, e := instance.transcoders[id]
	if e && t != nil {
		_ = t.Stop()
	}

	_ = instance.rtspHandler.RemovePath(id)

	instance.m.Lock()

	delete(instance.transcoders, id)
	delete(instance.units, id)

	instance.m.Unlock()

	return nil
}

func (instance *Instance) RestartUnit(unit Unit) error {
	_ = instance.RemoveUnit(unit.id)
	_, err := instance.AddUnit(unit.id, unit.path.from.String())
	if err != nil {
		return err
	}

	return nil
}
