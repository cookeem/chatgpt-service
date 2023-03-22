package chat

import (
	"fmt"
	"github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"
	"math/rand"
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

func RandomString(n int) string {
	var letter []rune
	lowerChars := "abcdefghijklmnopqrstuvwxyz"
	numberChars := "0123456789"
	chars := fmt.Sprintf("%s%s", lowerChars, numberChars)
	letter = []rune(chars)
	var str string
	b := make([]rune, n)
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range b {
		b[i] = letter[seededRand.Intn(len(letter))]
	}
	str = string(b)
	return str
}

const (
	StatusFail string = "FAIL"

	PingPeriod = time.Second * 50
	PingWait   = time.Second * 60
)

var (
	GPTModels = []string{
		openai.GPT432K0314,
		openai.GPT432K,
		openai.GPT40314,
		openai.GPT4,
		openai.GPT3Dot5Turbo0301,
		openai.GPT3Dot5Turbo,
		openai.GPT3TextDavinci003,
		openai.GPT3TextDavinci002,
		openai.GPT3TextCurie001,
		openai.GPT3TextBabbage001,
		openai.GPT3TextAda001,
		openai.GPT3TextDavinci001,
		openai.GPT3DavinciInstructBeta,
		openai.GPT3Davinci,
		openai.GPT3CurieInstructBeta,
		openai.GPT3Curie,
		openai.GPT3Ada,
		openai.GPT3Babbage,
	}
)
