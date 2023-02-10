package chat

import (
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

type Logger struct {
}

func (logger *Logger) LoggerInit() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func (logger Logger) LogDebug(args ...interface{}) {
	log.Debug(args...)
}

func (logger Logger) LogInfo(args ...interface{}) {
	log.Info(args...)
}

func (logger Logger) LogWarn(args ...interface{}) {
	log.Warn(args...)
}

func (logger Logger) LogError(args ...interface{}) {
	log.Error(args...)
}

func (logger Logger) LogPanic(args ...interface{}) {
	log.Panic(args...)
}

const (
	StatusFail string = "FAIL"

	pingPeriod = time.Second * 50
	pingWait   = time.Second * 60
)
