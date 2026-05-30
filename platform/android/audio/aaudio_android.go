//go:build android && cgo

package audio

/*
#include "audio_android.c"
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
