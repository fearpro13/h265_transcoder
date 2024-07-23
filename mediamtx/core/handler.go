package core

import (
	"context"
	"errors"
	auth2 "fearpro13/h265_transcoder/mediamtx/auth"
	"fearpro13/h265_transcoder/mediamtx/conf"
	"fearpro13/h265_transcoder/mediamtx/logger"
	"fearpro13/h265_transcoder/mediamtx/rtsp"
	"fmt"
	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/auth"
	"github.com/pion/logging"
	"sync/atomic"
	"time"
)

type RtspHandler struct {
	ctx      context.Context
	ctxF     context.CancelFunc
	pm       *pathManager
	rts      *rtsp.Server
	running  atomic.Bool
	RtspAddr string
	useUdp   bool
}

func NewRtspHandler(ctx context.Context, rtspPort uint16, useUdp bool) *RtspHandler {
	handler := &RtspHandler{
		running:  atomic.Bool{},
		RtspAddr: fmt.Sprintf(":%d", rtspPort),
		useUdp:   useUdp,
	}

	handler.ctx, handler.ctxF = context.WithCancel(ctx)

	return handler
}

func (h *RtspHandler) Start() error {
	if h.running.Load() {
		return errors.New("already started")
	}

	l, err := logger.New(logger.Info, []logger.Destination{logger.DestinationStdout}, "")
	if err != nil {
		return err
	}

	pm := &pathManager{
		logLevel: conf.LogLevel(logging.LogLevelDebug),
		authManager: &auth2.Manager{
			Method: 0,
			InternalUsers: []conf.AuthInternalUser{
				{
					User: "any",
					Pass: "any",
					IPs:  nil,
					Permissions: []conf.AuthInternalUserPermission{
						{
							Action: conf.AuthActionRead,
							Path:   "",
						},
						{
							Action: conf.AuthActionPublish,
							Path:   "",
						},
					},
				},
			},
			ReadTimeout: 5 * time.Second,
		},
		rtspAddress:       h.RtspAddr,
		readTimeout:       conf.StringDuration(5 * time.Second),
		writeTimeout:      conf.StringDuration(5 * time.Second),
		writeQueueSize:    512,
		udpMaxPayloadSize: 2000,
		pathConfs:         map[string]*conf.Path{},
		parent:            l,
	}
	pm.initialize()

	h.pm = pm

	allowedProto := map[conf.Protocol]struct{}{
		conf.Protocol(gortsplib.TransportTCP): {},
	}

	if h.useUdp {
		allowedProto[conf.Protocol(gortsplib.TransportUDP)] = struct{}{}
	}

	rts := &rtsp.Server{
		Address: h.RtspAddr,
		AuthMethods: []auth.ValidateMethod{
			auth.ValidateMethodBasic,
		},
		ReadTimeout:       conf.StringDuration(5 * time.Second),
		WriteTimeout:      conf.StringDuration(5 * time.Second),
		WriteQueueSize:    512,
		UseUDP:            h.useUdp,
		UseMulticast:      false,
		RTPAddress:        "0.0.0.0:6512",
		RTCPAddress:       "0.0.0.0:6513",
		MulticastIPRange:  "",
		MulticastRTPPort:  0,
		MulticastRTCPPort: 0,
		IsTLS:             false,
		ServerCert:        "",
		ServerKey:         "",
		RTSPAddress:       h.RtspAddr,
		Protocols:         allowedProto,
		PathManager:       pm,
		Parent:            l,
	}

	err = rts.Initialize()

	if err != nil {
		return err
	}

	h.rts = rts

	h.running.Store(true)

	go func() {
		select {
		case <-h.ctx.Done():
		}

		h.Stop()
	}()

	return nil
}

func (h *RtspHandler) Stop() {
	if !h.running.Load() {
		return
	}

	h.running.Store(false)
	h.ctxF()

	h.pm.close()
	h.rts.Close()
}

func (h *RtspHandler) AddPath(path string) error {
	tTcp := gortsplib.TransportTCP

	pathConf := &conf.Path{
		Name:              path,
		Source:            "publisher",
		MaxReaders:        0,
		OverridePublisher: false,
		RTSPTransport: conf.RTSPTransport{
			Transport: &tTcp,
		},
		RTSPAnyPort:    false,
		RTSPRangeType:  0,
		RTSPRangeStart: "",
		SourceRedirect: "",
	}

	currentConfs := h.pm.pathConfs
	_, e := currentConfs[path]
	if e {
		return errors.New("path already exist")
	}

	currentConfs[path] = pathConf

	h.pm.ReloadPathConfs(currentConfs)

	return nil
}

func (h *RtspHandler) RemovePath(path string) error {
	currentConfs := h.pm.pathConfs
	_, e := currentConfs[path]
	if !e {
		return errors.New("path does not exist")
	}

	delete(currentConfs, path)

	h.pm.ReloadPathConfs(currentConfs)

	return nil
}
