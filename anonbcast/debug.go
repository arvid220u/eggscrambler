package anonbcast

import (
	"fmt"
	"log"
)

// Debugging
// TODO: make this into an environment variable or something
const Debug = false

func DPrintf(format string, a ...interface{}) {
	log.SetFlags(log.Lmicroseconds)
	if Debug {
		log.Printf(format, a...)
	}
	return
}

func assertf(condition bool, format string, a ...interface{}) {
	if Debug && !condition {
		panic(fmt.Sprintf(format, a...))
	}
}
