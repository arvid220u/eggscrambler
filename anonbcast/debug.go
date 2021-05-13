package anonbcast

import (
	"fmt"
	"log"
	"os"
)

// Debugging
// Dump may have race conditions
const debugEnvKey = "DEBUG"
const dumpEnvKey = "DUMP"

type logTopic string

const (
	dError   logTopic = "ERRO"
	dWarning logTopic = "WARN"
	dInfo    logTopic = "INFO"
	dDump    logTopic = "DUMP"
)

func logf(topic logTopic, header string, format string, a ...interface{}) {
	if IsDebug() {
		log.Printf(string(topic)+" ["+header+"] "+format, a...)
	}
}

func assertf(condition bool, header string, format string, a ...interface{}) {
	if IsDebug() && !condition {
		logf(dError, header, format, a...)
		panic("assertion error: " + fmt.Sprintf("(see above) "+format, a...))
	}
}

func SetDebug(debug bool) {
	envVal := ""
	if debug {
		envVal = "true"
	}
	os.Setenv(debugEnvKey, envVal)
}

func IsDebug() bool {
	envVal := os.Getenv(debugEnvKey)
	return envVal == "true"
}

func SetDump(dump bool) {
	envVal := ""
	if dump {
		envVal = "true"
	}
	os.Setenv(dumpEnvKey, envVal)
}

func IsDump() bool {
	envVal := os.Getenv(dumpEnvKey)
	return envVal == "true"
}
