package logger

import (
	"log"
	"os"
)

type Logger struct {
	Debug   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
}

func New() *Logger {
	flags := log.Ldate | log.Ltime | log.Lshortfile

	return &Logger{
		Debug:   log.New(os.Stdout, "DEBUG: ", flags),
		Info:    log.New(os.Stdout, "INFO: ", flags),
		Warning: log.New(os.Stdout, "WARNING: ", flags),
		Error:   log.New(os.Stderr, "ERROR: ", flags),
	}
}
