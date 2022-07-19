package main

/*

#include <stdlib.h>
#include <jni.h>

static jstring jni_NewStringUTF(JNIEnv *env, const char *bytes) {
	return (*env)->NewStringUTF(env, bytes);
}

*/
import "C"
import "unsafe"

//export Java_com_github_rajveermalviya_tsukuru_androidnodeps_MainActivity_greeter
func Java_com_github_rajveermalviya_tsukuru_androidnodeps_MainActivity_greeter(env *C.JNIEnv, obj C.jobject) C.jstring {
	str := C.CString("Hello from Go!")
	defer C.free(unsafe.Pointer(str))

	return C.jni_NewStringUTF(env, str)
}

func main() {}
