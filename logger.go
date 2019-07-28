package binlog

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/onlyac0611/binlog/dump"
)

type Logger interface {
	Errorf(string, ...interface{})
	Infof(string, ...interface{})
	Debugf(string, ...interface{})
	Print(args ...interface{})
}

type logLevel uint8

const (
	DebugLevel logLevel = iota
	InfoLevel
	ErrorLevel
)

type DefaultLogger struct {
	Level  logLevel
	Logger *log.Logger
}

func newNilLogger() Logger {
	d := &DefaultLogger{
		Level:  ErrorLevel,
		Logger: log.New(os.Stderr, "[binlog]", log.Lmicroseconds|log.LstdFlags|log.Lshortfile),
	}
	return d
}

func NewDefaultLogger(writer io.Writer, level logLevel) Logger {
	d := &DefaultLogger{
		Level:  level,
		Logger: log.New(writer, "[binlog]", log.Lmicroseconds|log.LstdFlags|log.Lshortfile),
	}
	return d
}

func (d *DefaultLogger) Errorf(format string, args ...interface{}) {
	if d.Level <= ErrorLevel {
		d.Logger.Output(2, fmt.Sprintf(format, args...))
	}
}

func (d *DefaultLogger) Infof(format string, args ...interface{}) {
	if d.Level <= InfoLevel {
		d.Logger.Output(2, fmt.Sprintf(format, args...))
	}
}

func (d *DefaultLogger) Debugf(format string, args ...interface{}) {
	if d.Level <= DebugLevel {
		d.Logger.Output(2, fmt.Sprintf(format, args...))
	}
}

func (d *DefaultLogger) Print(args ...interface{}) {
	d.Logger.Output(2, fmt.Sprint(args...))
}

var logger Logger = newNilLogger()

func SetLogger(other Logger) {
	logger = other
	dump.SetLogger(other)
}
