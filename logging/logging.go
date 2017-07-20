package logging

import (
	stdlog "log"
	"os"
	"strings"

	golog "github.com/op/go-logging"
)

func Configure() {
	golog.SetFormatter(golog.MustStringFormatter("[path=%{shortfile}] [level=%{level}] %{message}"))
	stdoutLogBackend := golog.NewLogBackend(os.Stdout, "", stdlog.LstdFlags|stdlog.Lmicroseconds)
	golog.SetBackend(stdoutLogBackend)

	golog.SetLevel(getEnvLogLevel("REGISTRATOR_LOG_LEVEL",golog.INFO), "")
	golog.SetLevel(getEnvLogLevel("FARGO_LOG_LEVEL",golog.NOTICE), "fargo")
}

func getEnvLogLevel(logLevelEnvVarName string, defaultLevel golog.Level) golog.Level {
		logLevel := defaultLevel
		switch levelOverride := strings.ToUpper(os.Getenv(logLevelEnvVarName)); levelOverride {
		case "DEBUG":
			logLevel = golog.DEBUG
		case "INFO":
			logLevel = golog.INFO
		case "NOTICE":
			logLevel = golog.NOTICE
		case "WARNING":
			logLevel = golog.WARNING
		case "ERROR":
			logLevel = golog.ERROR
		case "CRITICAL":
			logLevel = golog.CRITICAL
 	}
 	return logLevel
}