//go:build android

package android

/*
#cgo LDFLAGS: -landroid
#include <jni.h>
#include <android/native_activity.h>
#include <stdlib.h>
#include <string.h>

// Call getFilesDir/getCacheDir/getExternalFilesDir on the activity and
// return the absolute path. Returns NULL on failure (caller must free).
char* android_storage_path(ANativeActivity* activity, int which) {
    if (activity == NULL || activity->env == NULL || activity->clazz == NULL)
        return NULL;

    JNIEnv* env = activity->env;
    jclass cls = (*env)->GetObjectClass(env, activity->clazz);
    if (cls == NULL) return NULL;

    const char* methodName;
    const char* methodSig;
    switch (which) {
        case 0: methodName = "getFilesDir";        methodSig = "()Ljava/io/File;"; break;
        case 1: methodName = "getCacheDir";        methodSig = "()Ljava/io/File;"; break;
        case 2: methodName = "getExternalFilesDir"; methodSig = "(Ljava/lang/String;)Ljava/io/File;"; break;
        default: (*env)->DeleteLocalRef(env, cls); return NULL;
    }

    jmethodID mid = (*env)->GetMethodID(env, cls, methodName, methodSig);
    if (mid == NULL) {
        (*env)->DeleteLocalRef(env, cls);
        return NULL;
    }

    jobject fileObj;
    if (which == 2) {
        fileObj = (*env)->CallObjectMethod(env, activity->clazz, mid, NULL);
    } else {
        fileObj = (*env)->CallObjectMethod(env, activity->clazz, mid);
    }
    if (fileObj == NULL) {
        (*env)->DeleteLocalRef(env, cls);
        return NULL;
    }

    jclass fileCls = (*env)->GetObjectClass(env, fileObj);
    jmethodID absMid = (*env)->GetMethodID(env, fileCls, "getAbsolutePath", "()Ljava/lang/String;");
    jstring pathStr = (*env)->CallObjectMethod(env, fileObj, absMid);

    const char* utf = (*env)->GetStringUTFChars(env, pathStr, NULL);
    char* result = strdup(utf);
    (*env)->ReleaseStringUTFChars(env, pathStr, utf);

    (*env)->DeleteLocalRef(env, fileCls);
    (*env)->DeleteLocalRef(env, fileObj);
    (*env)->DeleteLocalRef(env, cls);
    return result;
}
*/
import "C"

import (
	"unsafe"
)

// Storage provides access to Android app-specific storage directories.
type Storage struct {
	activity *C.ANativeActivity
}

// newStorage wraps the ANativeActivity pointer for storage path access.
func newStorage(activity unsafe.Pointer) *Storage {
	return &Storage{activity: (*C.ANativeActivity)(activity)}
}

// NewStorage creates a Storage from the ANativeActivity pointer obtained
// via bridge.GetActivity(). Callers should verify the pointer is non-nil.
func NewStorage(activity unsafe.Pointer) *Storage {
	return newStorage(activity)
}

const (
	_storageFilesDir       = 0
	_storageCacheDir       = 1
	_storageExternalFiles  = 2
)

func (s *Storage) getPath(which int) string {
	if s == nil || s.activity == nil {
		return ""
	}
	cPath := C.android_storage_path(s.activity, C.int(which))
	if cPath == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cPath))
	return C.GoString(cPath)
}

// FilesDir returns the internal files directory path, or "" if unavailable.
func (s *Storage) FilesDir() string       { return s.getPath(_storageFilesDir) }

// CacheDir returns the internal cache directory path, or "" if unavailable.
func (s *Storage) CacheDir() string       { return s.getPath(_storageCacheDir) }

// ExternalFilesDir returns the external files directory path, or "" if unavailable.
func (s *Storage) ExternalFilesDir() string { return s.getPath(_storageExternalFiles) }
