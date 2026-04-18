package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"unsafe"
	"github.com/soda92/vpn-share-tool/mobile"
)

//export GetNextEvent
func GetNextEvent() *C.char {
	return C.CString(mobile.GetNextEvent())
}

//export FreeString
func FreeString(str *C.char) {
	C.free(unsafe.Pointer(str))
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
