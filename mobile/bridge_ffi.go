// +build !android,!ios

package mobile

/*
#include <stdlib.h>
typedef void (*event_callback)(const char* event_json);
static void call_event_callback(void* cb, const char* event_json) {
    ((event_callback)cb)(event_json);
}
*/
import "C"
import (
	"unsafe"
)

// cEventCallback implements the EventCallback interface for C/FFI.
type cEventCallback struct {
	ptr unsafe.Pointer
}

func (c *cEventCallback) OnEvent(eventJSON string) {
	cStr := C.CString(eventJSON)
	defer C.free(unsafe.Pointer(cStr))
	C.call_event_callback(c.ptr, cStr)
}

//export SetEventCallback
func SetEventCallbackFFI(cb unsafe.Pointer) {
	SetEventCallback(&cEventCallback{ptr: cb})
}

//export StartApiServerWithPort
func StartApiServerWithPortFFI(apiPort C.int) {
	StartApiServerWithPort(int(apiPort))
}

//export StartGoBackendWithPort
func StartGoBackendWithPortFFI(apiPort C.int) {
	StartGoBackendWithPort(int(apiPort))
}

//export ShareURL
func ShareURLFFI(url *C.char) {
	ShareURL(C.GoString(url))
}

//export GetProxies
func GetProxiesFFI() *C.char {
	return C.CString(GetProxies())
}

//export GetIP
func GetIPFFI() *C.char {
	return C.CString(GetIP())
}

//export SetDeviceIP
func SetDeviceIPFFI(ip *C.char) {
	SetDeviceIP(C.GoString(ip))
}

//export SetStoragePath
func SetStoragePathFFI(path *C.char) {
	SetStoragePath(C.GoString(path))
}
