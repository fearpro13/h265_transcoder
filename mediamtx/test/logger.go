package test

import "fearpro13/h265_transcoder/mediamtx/logger"

type nilLogger struct{}

func (nilLogger) Log(_ logger.Level, _ string, _ ...interface{}) {
}

// NilLogger is a logger to /dev/null
var NilLogger logger.Writer = &nilLogger{}
