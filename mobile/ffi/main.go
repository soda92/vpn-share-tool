package main

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
	"github.com/soda92/vpn-share-tool/mobile"
)

// cEventCallback implements the mobile.EventCallback interface for C/FFI.
type cEventCallback struct {
	ptr unsafe.Pointer
}

func (c *cEventCallback) OnEvent(eventJSON string) {
	cStr := C.CString(eventJSON)
	defer C.free(unsafe.Pointer(cStr))
	C.call_event_callback(c.ptr, cStr)
}

//export SetEventCallback
func SetEventCallback(cb unsafe.Pointer) {
	mobile.SetEventCallback(&cEventCallback{ptr: cb})
}

//export StartApiServerWithPort
func StartApiServerWithPort(apiPort C.int) {
	mobile.StartApiServerWithPort(int(apiPort))
}

//export StartGoBackendWithPort
func StartGoBackendWithPort(apiPort C.int) {
	mobile.StartGoBackendWithPort(int(apiPort))
}

//export ShareURL
func ShareURL(url *C.char) {
	mobile.ShareURL(C.GoString(url))
}

//export GetProxies
func GetProxies() *C.char {
	return C.CString(mobile.GetProxies())
}

//export GetIP
func GetIP() *C.char {
	return C.CString(mobile.GetIP())
}

//export SetDeviceIP
func SetDeviceIP(ip *C.char) {
	mobile.SetDeviceIP(C.GoString(ip))
}

//export SetStoragePath
func SetStoragePath(path *C.char) {
	mobile.SetStoragePath(C.GoString(path))
}

func main() {}
