//go:build android

package android

/*
#cgo LDFLAGS: -landroid
#include "/opt/android-ndk/toolchains/llvm/prebuilt/linux-x86_64/sysroot/usr/include/jni.h"
#include <stdlib.h>
*/
import "C"
import (
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/mobile"
)

func GetIPAddress() string {
	var ip string
	driver := fyne.CurrentApp().Driver()
	if mobileDriver, ok := driver.(mobile.Driver); ok {
		mobileDriver.RunOnJVM(func(vm, env, ctx uintptr) {
			// Get the class
			class := C.JNI_FindClass(C.uintptr_t(env), C.CString("com/example/vpnsharetool/GetIP"))
			if class == 0 {
				return
			}

			// Get the method
			method := C.JNI_GetStaticMethodID(C.uintptr_t(env), class, C.CString("getIPAddress"), C.CString("(Landroid/content/Context;)Ljava/lang/String;"))
			if method == 0 {
				return
			}

			// Call the method
			jIP := C.JNI_CallStaticObjectMethod(C.uintptr_t(env), class, method, C.jobject(ctx))
			if jIP == 0 {
				return
			}

			// Convert the result to a Go string
			ip = C.GoString(C.JNI_GetStringUTFChars(C.uintptr_t(env), C.jstring(jIP), nil))
		})
	}
	return ip
}
