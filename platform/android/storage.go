//go:build android && cgo

package android

/*
#cgo LDFLAGS: -landroid
#include <android/native_activity.h>
#include <stdlib.h>
#include <string.h>

// Return the activity storage path without going through JNI. Using the
// direct ANativeActivity fields avoids any Java-side stack walking during
// bootstrap and is sufficient for the app-specific directories we need.
static char* android_storage_path_from_field(const char* path) {
    return path == NULL ? NULL : strdup(path);
}

static char* android_cache_path_from_internal(const char* internalPath) {
    if (internalPath == NULL) return NULL;

    const char* filesSuffix = "/files";
    const char* cacheSuffix = "/cache";
    size_t internalLen = strlen(internalPath);
    size_t filesSuffixLen = strlen(filesSuffix);
    size_t cacheSuffixLen = strlen(cacheSuffix);

    if (internalLen >= filesSuffixLen &&
        strcmp(internalPath + internalLen - filesSuffixLen, filesSuffix) == 0) {
        size_t prefixLen = internalLen - filesSuffixLen;
        char* result = (char*)malloc(prefixLen + cacheSuffixLen + 1);
        if (result == NULL) return NULL;
        memcpy(result, internalPath, prefixLen);
        memcpy(result + prefixLen, cacheSuffix, cacheSuffixLen + 1);
        return result;
    }

    return strdup(internalPath);
}

// Call getFilesDir/getCacheDir/getExternalFilesDir on the activity and
// return the absolute path. Returns NULL on failure (caller must free).
char* android_storage_path(ANativeActivity* activity, int which) {
    if (activity == NULL)
        return NULL;

    switch (which) {
        case 0:
            return android_storage_path_from_field(activity->internalDataPath);
        case 1:
            return android_cache_path_from_internal(activity->internalDataPath);
        case 2:
            return android_storage_path_from_field(activity->externalDataPath);
        default:
            return NULL;
    }
}
*/
import "C"

import (
	"unsafe"
)

// AppStorage provides access to Android app-specific storage directories.
type AppStorage struct {
	activity *C.ANativeActivity
}

// newStorage wraps the ANativeActivity pointer for storage path access.
func newStorage(activity unsafe.Pointer) *AppStorage {
	return &AppStorage{activity: (*C.ANativeActivity)(activity)}
}

// NewStorage creates a Storage from the ANativeActivity pointer obtained
// via bridge.GetActivity(). Callers should verify the pointer is non-nil.
func NewStorage(activity unsafe.Pointer) *AppStorage {
	return newStorage(activity)
}

const (
	_storageFilesDir       = 0
	_storageCacheDir       = 1
	_storageExternalFiles  = 2
)

func (s *AppStorage) getPath(which int) string {
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
func (s *AppStorage) FilesDir() string       { return s.getPath(_storageFilesDir) }

// CacheDir returns the internal cache directory path, or "" if unavailable.
func (s *AppStorage) CacheDir() string       { return s.getPath(_storageCacheDir) }

// ExternalFilesDir returns the external files directory path, or "" if unavailable.
func (s *AppStorage) ExternalFilesDir() string { return s.getPath(_storageExternalFiles) }
