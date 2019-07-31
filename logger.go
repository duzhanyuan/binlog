package binlog

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/onlyac0611/binlog/dump"
)

//Logger用于打印binlog包的调试信息
type Logger interface {
	Errorf(string, ...interface{}) //错误信息打印
	Infof(string, ...interface{})  //进程信息打印
	Debugf(string, ...interface{}) //调试信息打印
	Print(args ...interface{})     //打印dump包的错误信息
}

//日志级别
type LogLevel uint8

const (
	DebugLevel LogLevel = iota //调试
	InfoLevel                  //进程
	ErrorLevel                 //错误
)

type defaultLogger struct {
	level  LogLevel
	logger *log.Logger
}

func newNilLogger() Logger {
	d := &defaultLogger{
		level:  ErrorLevel,
		logger: log.New(os.Stderr, "[binlog]", log.Lmicroseconds|log.LstdFlags|log.Lshortfile),
	}
	return d
}

//生成一个日志打印Logger，level可以是DebugLevel，InfoLevel，ErrorLevel
func NewDefaultLogger(writer io.Writer, level LogLevel) Logger {
	d := &defaultLogger{
		level:  level,
		logger: log.New(writer, "[binlog]", log.Lmicroseconds|log.LstdFlags|log.Lshortfile),
	}
	return d
}

func (d *defaultLogger) Errorf(format string, args ...interface{}) {
	if d.level <= ErrorLevel {
		d.logger.Output(2, fmt.Sprintf(format, args...))
	}
}

func (d *defaultLogger) Infof(format string, args ...interface{}) {
	if d.level <= InfoLevel {
		d.logger.Output(2, fmt.Sprintf(format, args...))
	}
}

func (d *defaultLogger) Debugf(format string, args ...interface{}) {
	if d.level <= DebugLevel {
		d.logger.Output(2, fmt.Sprintf(format, args...))
	}
}

func (d *defaultLogger) Print(args ...interface{}) {
	d.logger.Output(2, fmt.Sprint(args...))
}

var logger Logger = newNilLogger()

//设置一个符合Logger日志来打印binlog包的调试信息
func SetLogger(other Logger) {
	logger = other
	dump.SetLogger(other)
}
