package logging

import (
	stdlog "log"
	"os"
	"strings"

	golog "github.com/op/go-logging"
)

// Due to go-logging not having a "Verbose" level, we're
// bumping the lower levels down by 1
// Debug -> Verbose
// Info -> Debug
// Notice -> Info
// Warn, Error, and Critical stay the same
var defaultLevel golog.Level = golog.INFO

func Configure() {
	golog.SetFormatter(golog.MustStringFormatter("[path=%{shortfile}] [0x%{id:x}] [level=%{level}] [module=%{module}] [app=alyx3] [func=%{shortfunc}] %{message}"))
	stdoutLogBackend := golog.NewLogBackend(os.Stdout, "", stdlog.LstdFlags|stdlog.Lmicroseconds)
	stdoutLogBackend.Color = true

	// NOTE these file permissions are restricted by umask, so they probably won't work right.
	err := os.MkdirAll("./log", 0775)
	if err != nil {
		panic(err)
	}
	logFile, err := os.OpenFile("./log/registrator.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
	if err != nil {
		panic(err)
	}

	fileLogBackend := golog.NewLogBackend(logFile, "", stdlog.LstdFlags|stdlog.Lmicroseconds)
	fileLogBackend.Color = false

	golog.SetBackend(stdoutLogBackend, fileLogBackend)
}

func SetLevels(levels map[string]string) {
	if levelString, ok := levels["default"]; ok {
		level, err := golog.LogLevel(levelString)
		if err != nil {
			level = defaultLevel
		}
		golog.SetLevel(level, "")
		delete(levels, "default")
	} else {
		golog.SetLevel(defaultLevel, "")
	}

	for module, levelString := range levels {
		level, err := golog.LogLevel(levelString)
		if err != nil {
			continue
		}
		module = strings.Replace(module, "##", ".", -1)
		golog.SetLevel(level, module)
	}
}