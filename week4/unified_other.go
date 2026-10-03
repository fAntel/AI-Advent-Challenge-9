//go:build !darwin || !cgo

package agent

import "log"

func LogSchedule(message string, failed bool) { log.Print(message) }
