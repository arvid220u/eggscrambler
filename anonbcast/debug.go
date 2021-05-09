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

func DPrintf(format string, a ...interface{}) {
	log.SetFlags(log.Lmicroseconds)
	if IsDebug() {
		log.Printf(format, a...)
	}
}

func assertf(condition bool, format string, a ...interface{}) {
	if IsDebug() && !condition {
		panic(fmt.Sprintf(format, a...))
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
