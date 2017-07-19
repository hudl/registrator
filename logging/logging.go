package logging

import (
	stdlog "log"
	"os"

	golog "github.com/op/go-logging"
)

var logLevel golog.Level = golog.INFO

func Configure(enableVerbose bool) {
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

	if enableVerbose {
		logLevel = golog.DEBUG
	}

	golog.SetLevel(logLevel, "")
}
