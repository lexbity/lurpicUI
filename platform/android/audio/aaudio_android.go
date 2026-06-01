//go:build android && cgo

package audio

/*
// Link the OpenSL ES and Android logging libraries. audio_android.c
// references OpenSL ES symbols (SL_IID_BUFFERQUEUE, slCreateEngine, ...)
// and __android_log_print; without these the symbols are unresolved and
// dlopen of libgo.so fails at runtime with UnsatisfiedLinkError.
#cgo LDFLAGS: -lOpenSLES -llog
// audio_android.c is compiled automatically by cgo as a standalone
// translation unit; declare only the prototypes here so the symbols are
// not defined twice (which would cause duplicate-symbol link errors).
#include <stdint.h>

int   aaudio_available(void);
void* aaudio_stream_open(int sampleRate, int channels, int bitsPerSample, int lowLatency);
int   aaudio_stream_write(void* handle, const int16_t* samples, int frameCount);
int   aaudio_stream_pause(void* handle);
int   aaudio_stream_resume(void* handle);
int   aaudio_stream_close(void* handle);
int   aaudio_stream_latency(void* handle);

void* opensl_stream_open(int sampleRate, int channels, int bitsPerSample);
int   opensl_stream_write(void* handle, const int16_t* samples, int frameCount);
int   opensl_stream_pause(void* handle);
int   opensl_stream_resume(void* handle);
int   opensl_stream_close(void* handle);
*/
import "C"

import (
	"errors"
	"unsafe"
)

func cAAudioAvailable() bool {
	return C.aaudio_available() != 0
}

func cAAudioStreamOpen(sampleRate, channels, bitsPerSample int, lowLatency bool) (unsafe.Pointer, error) {
	ll := 0
	if lowLatency {
		ll = 1
	}
	handle := C.aaudio_stream_open(C.int(sampleRate), C.int(channels), C.int(bitsPerSample), C.int(ll))
	if handle == nil {
		return nil, ErrAudioNotAvailable
	}
	return handle, nil
}

func cAAudioStreamWrite(handle unsafe.Pointer, samples []int16) (int, error) {
	if len(samples) == 0 {
		return 0, nil
	}
	if handle == nil {
		return 0, errors.New("aaudio: nil stream handle")
	}
	frames := C.aaudio_stream_write(handle, (*C.int16_t)(unsafe.Pointer(&samples[0])), C.int(len(samples)))
	if frames < 0 {
		return 0, errors.New("aaudio: write failed")
	}
	return int(frames), nil
}

func cAAudioStreamPause(handle unsafe.Pointer) error {
	if C.aaudio_stream_pause(handle) != 0 {
		return errors.New("aaudio: pause failed")
	}
	return nil
}

func cAAudioStreamResume(handle unsafe.Pointer) error {
	if C.aaudio_stream_resume(handle) != 0 {
		return errors.New("aaudio: resume failed")
	}
	return nil
}

func cAAudioStreamClose(handle unsafe.Pointer) error {
	C.aaudio_stream_close(handle)
	return nil
}

func cAAudioStreamLatency(handle unsafe.Pointer) int {
	return int(C.aaudio_stream_latency(handle))
}

// OpenSL ES wrappers

func cOpenSLStreamOpen(sampleRate, channels, bitsPerSample int) (unsafe.Pointer, error) {
	handle := C.opensl_stream_open(C.int(sampleRate), C.int(channels), C.int(bitsPerSample))
	if handle == nil {
		return nil, ErrAudioNotAvailable
	}
	return handle, nil
}

func cOpenSLStreamWrite(handle unsafe.Pointer, samples []int16) (int, error) {
	if len(samples) == 0 || handle == nil {
		return 0, nil
	}
	frames := C.opensl_stream_write(handle, (*C.int16_t)(unsafe.Pointer(&samples[0])), C.int(len(samples)))
	if frames < 0 {
		return 0, errors.New("opensl: write failed")
	}
	return int(frames), nil
}

func cOpenSLStreamPause(handle unsafe.Pointer) error {
	if C.opensl_stream_pause(handle) != 0 {
		return errors.New("opensl: pause failed")
	}
	return nil
}

func cOpenSLStreamResume(handle unsafe.Pointer) error {
	if C.opensl_stream_resume(handle) != 0 {
		return errors.New("opensl: resume failed")
	}
	return nil
}

func cOpenSLStreamClose(handle unsafe.Pointer) error {
	C.opensl_stream_close(handle)
	return nil
}
