//go:build darwin && cgo

package agent

/*
#include <os/log.h>
static void scheduleLog(const char *message, int failed) {
  os_log_t log=os_log_create("dev.aiadvent.advent-agent","scheduler");
  os_log_with_type(log,failed ? OS_LOG_TYPE_ERROR : OS_LOG_TYPE_DEFAULT,"%{public}s",message);
}
*/
import "C"
import "unsafe"

func LogSchedule(message string, failed bool) {
	p := C.CString(message)
	defer C.free(unsafe.Pointer(p))
	f := C.int(0)
	if failed {
		f = 1
	}
	C.scheduleLog(p, f)
}
