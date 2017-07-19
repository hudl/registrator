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

	golog.SetBackend(stdoutLogBackend)

	if enableVerbose {
		logLevel = golog.DEBUG
	}

	golog.SetLevel(logLevel, "")
	golog.SetLevel(logLevel, "fargo")
}
