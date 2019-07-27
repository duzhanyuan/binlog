package binlog

import (
	"fmt"
	"log"
	"os"
)

type Logger interface {
	Errorf(string, ...interface{})
	Infof(string, ...interface{})
	Debugf(string, ...interface{})
}

type logLevel uint8

const (
	debugLevel logLevel = iota
	infoLevel
	errorLevel
)

type defaultLogger struct {
	level  logLevel
	logger *log.Logger
}

func newDefaultLogger() *defaultLogger {
	d := &defaultLogger{
		level:  infoLevel,
		logger: log.New(os.Stdout, "", log.Lmicroseconds|log.LstdFlags),
	}
	return d
}

func (d *defaultLogger) Errorf(format string, args ...interface{}) {
	if d.level <= errorLevel {
		log.Output(2, fmt.Sprintf(format, args...))
	}
}

func (d *defaultLogger) Infof(format string, args ...interface{}) {
	if d.level <= infoLevel {
		log.Output(2, fmt.Sprintf(format, args...))
	}
}

func (d *defaultLogger) Debugf(format string, args ...interface{}) {
	if d.level <= debugLevel {
		log.Output(2, fmt.Sprintf(format, args...))
	}
}

var logger Logger = newDefaultLogger()

func SetLogger(other Logger) {
	logger = other
}
