package logger

import (
	log "github.com/sirupsen/logrus"
)

type Logger interface {
	Info(msg string, kv ...interface{})
	Warn(msg string, kv ...interface{})
	Error(msg string, kv ...interface{})
	Fatal(msg string, kv ...interface{})
}

type defaultLogger struct{}

func kvs(msg string, kv ...interface{}) []interface{} {
	ret := make([]interface{}, 0, len(kv)+1)
	ret = append(ret, msg)
	ret = append(ret, kv...)
	return ret
}

func (dl *defaultLogger) Info(msg string, kv ...interface{}) {
	log.Info(kvs(msg, kv...))
}

func (dl *defaultLogger) Warn(msg string, kv ...interface{}) {
	log.Warn(kvs(msg, kv...))
}

func (dl *defaultLogger) Error(msg string, kv ...interface{}) {
	log.Error(kvs(msg, kv...))
}

func (dl *defaultLogger) Fatal(msg string, kv ...interface{}) {
	log.Fatal(kvs(msg, kv...))
}

func NewDebugLogger() Logger {
	return &defaultLogger{}
}
