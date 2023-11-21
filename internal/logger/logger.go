/*
Level Numeric value

CRITICAL 50
ERROR 40
WARNING 30
INFO 20
DEBUG 10
NOTSET 0

https://github.com/codeskyblue/beelog
*/

package logger

import (
	"fmt"
	"log"
	"os"

	"spread_message/internal/config"
)

var (
	level  = LevelNotset
	Logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)
)

const (
	LevelNotset = iota
	LevelDebug  = iota * 10
	LevelInfo
	LevelWarning
	LevelError
	LevelCritical
)

func SetLevel(l int) {
	level = l
}

func SetLogger(l *log.Logger) {
	Logger = l
}

func logPrint(le int, format string, a ...interface{}) {
	if le < level {
		return
	}
	var s string
	switch le {
	case LevelDebug:
		s = "[Debug]"
	case LevelInfo:
		s = "[Info]"
	case LevelWarning:
		s = "[Warning]"
	case LevelError:
		s = "[Error]"
	case LevelCritical:
		s = "[Critical]"
	}

	Logger.Printf(fmt.Sprintf("%s - %s\n", s, format), a...)
}

func Debug(v ...interface{}) {
	logPrint(LevelDebug, "%v", v)
}

func Debugf(format string, v ...interface{}) {
	logPrint(LevelDebug, format, v...)
}

func Info(v ...interface{}) {
	logPrint(LevelInfo, "%v", v)
}

func Warn(v ...interface{}) {
	logPrint(LevelWarning, "%v", v)
}

func Error(v ...interface{}) {
	logPrint(LevelError, "%v", v)
}

func Critical(v ...interface{}) {
	logPrint(LevelCritical, "%v", v)
}

func NewLogger(c *config.Config) {
	lg := log.New(os.Stdout, "", log.Ldate|log.Ltime)
	SetLogger(lg)
	SetLevel(c.LogLevel)
}
