package log

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Print(v any) {
	log.Print(v)
}

func Error(err error) {
	log.Error().Msg(err.Error())
}

func Errorf(format string, v ...any) {
	log.Error().Msg(fmt.Errorf(format, v...).Error())
}

func Debug(msg string) {
	log.Debug().Msg(msg)
}

func Debugf(format string, v ...any) {
	log.Debug().Msgf(format, v...)
}

func Info(msg string) {
	log.Info().Msg(msg)
}

func Infof(format string, v ...any) {
	log.Info().Msgf(format, v...)
}

func Trace(msg string) {
	log.Trace().Msg(msg)
}

func Tracef(format string, v ...any) {
	log.Trace().Msgf(format, v...)
}

func Panic(err error) {
	log.Panic().Msg(err.Error())
}

func Panicf(format string, v ...any) {
	log.Panic().Msgf(format, v...)
}

func Init(level string) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "3:04:05PM"})
	if l, err := zerolog.ParseLevel(level); err == nil {
		zerolog.SetGlobalLevel(l)
	}
}
