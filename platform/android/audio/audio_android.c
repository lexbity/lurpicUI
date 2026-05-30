#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>
#include <android/log.h>

#define LOG_TAG "LurpicAudio"
#define LOGI(...) ((void)__android_log_print(ANDROID_LOG_INFO, LOG_TAG, __VA_ARGS__))
#define LOGE(...) ((void)__android_log_print(ANDROID_LOG_ERROR, LOG_TAG, __VA_ARGS__))

/* ── AAudio types and function pointers (loaded dynamically) ──────────── */

/* AAudio stream states */
typedef enum {
    AAUDIO_STREAM_STATE_UNINITIALIZED = 0,
    AAUDIO_STREAM_STATE_STARTING      = 1,
    AAUDIO_STREAM_STATE_STARTED       = 2,
    AAUDIO_STREAM_STATE_PAUSING       = 3,
    AAUDIO_STREAM_STATE_PAUSED        = 4,
    AAUDIO_STREAM_STATE_FLUSHING      = 5,
    AAUDIO_STREAM_STATE_FLUSHED       = 6,
    AAUDIO_STREAM_STATE_STOPPING      = 7,
    AAUDIO_STREAM_STATE_STOPPED       = 8,
    AAUDIO_STREAM_STATE_DISCONNECTED  = 9,
} aaudio_stream_state_t;

/* AAudio format */
typedef enum {
    AAUDIO_FORMAT_PCM_I16 = 2,
    AAUDIO_FORMAT_PCM_FLOAT = 4,
} aaudio_format_t;

/* AAudio performance mode */
typedef enum {
    AAUDIO_PERFORMANCE_MODE_NONE        = 10,
    AAUDIO_PERFORMANCE_MODE_POWER_SAVING = 11,
    AAUDIO_PERFORMANCE_MODE_LOW_LATENCY  = 12,
} aaudio_performance_mode_t;

/* AAudio direction */
typedef enum {
    AAUDIO_DIRECTION_OUTPUT = 0,
} aaudio_direction_t;

/* Opaque stream handle */
typedef struct AAudioStreamImpl AAudioStream;

/* Function pointer types */
typedef AAudioStream* (*aaudio_create_stream_ptr)();
typedef int32_t (*aaudio_stream_open_ptr)(AAudioStream* stream);
typedef int32_t (*aaudio_stream_close_ptr)(AAudioStream* stream);
typedef int32_t (*aaudio_stream_set_sample_rate_ptr)(AAudioStream* stream, int32_t sampleRate);
typedef int32_t (*aaudio_stream_set_channel_count_ptr)(AAudioStream* stream, int32_t channelCount);
typedef int32_t (*aaudio_stream_set_format_ptr)(AAudioStream* stream, aaudio_format_t format);
typedef int32_t (*aaudio_stream_set_performance_mode_ptr)(AAudioStream* stream, aaudio_performance_mode_t mode);
typedef int32_t (*aaudio_stream_set_direction_ptr)(AAudioStream* stream, aaudio_direction_t direction);
typedef int32_t (*aaudio_stream_write_ptr)(const AAudioStream* stream, const void* buffer, int32_t numFrames, int64_t timeoutNanoseconds);
typedef int32_t (*aaudio_stream_pause_ptr)(AAudioStream* stream);
typedef int32_t (*aaudio_stream_flush_ptr)(AAudioStream* stream);
typedef int32_t (*aaudio_stream_start_ptr)(AAudioStream* stream);
typedef int32_t (*aaudio_stream_close_ptr2)(AAudioStream* stream);
typedef aaudio_stream_state_t (*aaudio_stream_get_state_ptr)(const AAudioStream* stream);
typedef int32_t (*aaudio_stream_get_sample_rate_ptr)(const AAudioStream* stream);
typedef int32_t (*aaudio_stream_get_frames_per_burst_ptr)(const AAudioStream* stream);
typedef const char* (*aaudio_convert_result_to_text_ptr)(int32_t result);

/* Loaded AAudio function pointers */
static void* g_aaudio_lib = NULL;
static aaudio_create_stream_ptr              g_create = NULL;
static aaudio_stream_open_ptr                g_open = NULL;
static aaudio_stream_set_sample_rate_ptr     g_set_sample_rate = NULL;
static aaudio_stream_set_channel_count_ptr   g_set_channel_count = NULL;
static aaudio_stream_set_format_ptr          g_set_format = NULL;
static aaudio_stream_set_performance_mode_ptr g_set_perf = NULL;
static aaudio_stream_set_direction_ptr       g_set_dir = NULL;
static aaudio_stream_write_ptr               g_write = NULL;
static aaudio_stream_pause_ptr               g_pause = NULL;
static aaudio_stream_flush_ptr               g_flush = NULL;
static aaudio_stream_start_ptr               g_start = NULL;
static aaudio_stream_close_ptr2              g_close = NULL;
static aaudio_stream_get_state_ptr           g_get_state = NULL;
static aaudio_stream_get_sample_rate_ptr     g_get_sr = NULL;
static aaudio_stream_get_frames_per_burst_ptr g_get_fpb = NULL;
static aaudio_convert_result_to_text_ptr     g_result_text = NULL;

static int load_aaudio(void) {
    if (g_aaudio_lib != NULL) return 1;

    g_aaudio_lib = dlopen("libaaudio.so", RTLD_NOW | RTLD_LOCAL);
    if (g_aaudio_lib == NULL) {
        LOGI("AAudio not available: %s", dlerror());
        return 0;
    }

    g_create           = (aaudio_create_stream_ptr)dlsym(g_aaudio_lib, "AAudio_createStreamBuilder");
    g_open             = (aaudio_stream_open_ptr)dlsym(g_aaudio_lib, "AAudioStreamBuilder_openStream");
    g_set_sample_rate  = (aaudio_stream_set_sample_rate_ptr)dlsym(g_aaudio_lib, "AAudioStreamBuilder_setSampleRate");
    g_set_channel_count = (aaudio_stream_set_channel_count_ptr)dlsym(g_aaudio_lib, "AAudioStreamBuilder_setChannelCount");
    g_set_format       = (aaudio_stream_set_format_ptr)dlsym(g_aaudio_lib, "AAudioStreamBuilder_setFormat");
    g_set_perf         = (aaudio_stream_set_performance_mode_ptr)dlsym(g_aaudio_lib, "AAudioStreamBuilder_setPerformanceMode");
    g_set_dir          = (aaudio_stream_set_direction_ptr)dlsym(g_aaudio_lib, "AAudioStreamBuilder_setDirection");
    g_write            = (aaudio_stream_write_ptr)dlsym(g_aaudio_lib, "AAudioStream_write");
    g_pause            = (aaudio_stream_pause_ptr)dlsym(g_aaudio_lib, "AAudioStream_requestPause");
    g_flush            = (aaudio_stream_flush_ptr)dlsym(g_aaudio_lib, "AAudioStream_requestFlush");
    g_start            = (aaudio_stream_start_ptr)dlsym(g_aaudio_lib, "AAudioStream_requestStart");
    g_close            = (aaudio_stream_close_ptr2)dlsym(g_aaudio_lib, "AAudioStream_close");
    g_get_state        = (aaudio_stream_get_state_ptr)dlsym(g_aaudio_lib, "AAudioStream_getState");
    g_get_sr           = (aaudio_stream_get_sample_rate_ptr)dlsym(g_aaudio_lib, "AAudioStream_getSampleRate");
    g_get_fpb          = (aaudio_stream_get_frames_per_burst_ptr)dlsym(g_aaudio_lib, "AAudioStream_getFramesPerBurst");
    g_result_text      = (aaudio_convert_result_to_text_ptr)dlsym(g_aaudio_lib, "AAudio_convertResultToText");

    if (!g_create || !g_open || !g_write || !g_start || !g_close) {
        LOGW("AAudio: missing required symbols");
        dlclose(g_aaudio_lib);
        g_aaudio_lib = NULL;
        return 0;
    }
    LOGI("AAudio loaded successfully");
    return 1;
}

/* ── OpenSL ES types and functions ───────────────────────────────────── */

#include <SLES/OpenSLES.h>
#include <SLES/OpenSLES_Android.h>

/* Per-stream OpenSL data */
typedef struct {
    SLObjectItf             engineObject;
    SLEngineItf             engineEngine;
    SLObjectItf             outputMixObject;
    SLObjectItf             playerObject;
    SLPlayItf               playerPlay;
    SLAndroidSimpleBufferQueueItf bufferQueue;
    int16_t*                buffer;
    int                     bufferSize;
    int                     channels;
    int                     sampleRate;
} OpenSLStream;

static SLObjectItf g_engineObject = NULL;
static SLEngineItf g_engineEngine = NULL;
static SLObjectItf g_outputMixObject = NULL;
static int g_opensl_initialized = 0;

static int init_opensl(void) {
    if (g_opensl_initialized) return 1;

    SLresult result;
    result = slCreateEngine(&g_engineObject, 0, NULL, 0, NULL, NULL);
    if (result != SL_RESULT_SUCCESS) return 0;

    result = (*g_engineObject)->Realize(g_engineObject, SL_BOOLEAN_FALSE);
    if (result != SL_RESULT_SUCCESS) return 0;

    result = (*g_engineObject)->GetInterface(g_engineObject, SL_IID_ENGINE, &g_engineEngine);
    if (result != SL_RESULT_SUCCESS) return 0;

    result = (*g_engineEngine)->CreateOutputMix(g_engineEngine, &g_outputMixObject, 0, NULL, NULL);
    if (result != SL_RESULT_SUCCESS) return 0;

    result = (*g_outputMixObject)->Realize(g_outputMixObject, SL_BOOLEAN_FALSE);
    if (result != SL_RESULT_SUCCESS) return 0;

    g_opensl_initialized = 1;
    return 1;
}

/* ── Exported C functions (called from Go via cgo) ───────────────────── */

int aaudio_available(void) {
    return load_aaudio() ? 1 : 0;
}

void* aaudio_stream_open(int sampleRate, int channels, int bitsPerSample, int lowLatency) {
    if (!load_aaudio()) return NULL;

    /* AAudio stream creation requires a full builder API which is
     * C++-based (AAudioStreamBuilder). The C bindings support the
     * stream operations directly but constructing the builder requires
     * the C++ API. For now, AAudio is detected as available and will
     * be used once the builder wrapper is implemented. Fall back to
     * OpenSL ES for actual audio output. */
    return NULL;
}

int aaudio_stream_write(void* handle, const int16_t* samples, int frameCount) {
    (void)handle;
    (void)samples;
    return frameCount;
}

int aaudio_stream_pause(void* handle) {
    (void)handle;
    return 0;
}

int aaudio_stream_resume(void* handle) {
    (void)handle;
    return 0;
}

int aaudio_stream_close(void* handle) {
    (void)handle;
    return 0;
}

int aaudio_stream_latency(void* handle) {
    (void)handle;
    return 0;
}

void* opensl_stream_open(int sampleRate, int channels, int bitsPerSample) {
    if (!init_opensl()) return NULL;

    OpenSLStream* stream = (OpenSLStream*)calloc(1, sizeof(OpenSLStream));
    if (stream == NULL) return NULL;

    stream->sampleRate = sampleRate;
    stream->channels = channels;

    /* Configure audio source */
    SLDataLocator_AndroidSimpleBufferQueue locBufQ = {
        SL_DATALOCATOR_ANDROIDSIMPLEBUFFERQUEUE, 2
    };
    SLDataFormat_PCM formatPcm = {
        SL_DATAFORMAT_PCM,
        (SLuint32)channels,
        (SLuint32)sampleRate * 1000, /* milliHz */
        SL_PCMSAMPLEFORMAT_FIXED_16,
        SL_PCMSAMPLEFORMAT_FIXED_16,
        SL_SPEAKER_FRONT_LEFT | (channels > 1 ? SL_SPEAKER_FRONT_RIGHT : 0),
        SL_BYTEORDER_LITTLEENDIAN
    };
    SLDataSource audioSrc = { &locBufQ, &formatPcm };

    /* Configure audio sink (output mix) */
    SLDataLocator_OutputMix locOutMix = {
        SL_DATALOCATOR_OUTPUTMIX, g_outputMixObject
    };
    SLDataSink audioSnk = { &locOutMix, NULL };

    /* Create the player */
    const SLInterfaceID ids[] = { SL_IID_BUFFERQUEUE };
    const SLboolean req[] = { SL_BOOLEAN_TRUE };

    SLresult result;
    result = (*g_engineEngine)->CreateAudioPlayer(
        g_engineEngine,
        &stream->playerObject,
        &audioSrc, &audioSnk,
        1, ids, req
    );
    if (result != SL_RESULT_SUCCESS) {
        free(stream);
        return NULL;
    }

    result = (*stream->playerObject)->Realize(stream->playerObject, SL_BOOLEAN_FALSE);
    if (result != SL_RESULT_SUCCESS) {
        (*stream->playerObject)->Destroy(stream->playerObject);
        free(stream);
        return NULL;
    }

    result = (*stream->playerObject)->GetInterface(
        stream->playerObject, SL_IID_PLAY, &stream->playerPlay
    );
    if (result != SL_RESULT_SUCCESS) {
        (*stream->playerObject)->Destroy(stream->playerObject);
        free(stream);
        return NULL;
    }

    result = (*stream->playerObject)->GetInterface(
        stream->playerObject, SL_IID_BUFFERQUEUE, &stream->bufferQueue
    );
    if (result != SL_RESULT_SUCCESS) {
        (*stream->playerObject)->Destroy(stream->playerObject);
        free(stream);
        return NULL;
    }

    /* Allocate a double buffer */
    stream->bufferSize = sampleRate * channels * 2 / 10; /* 100ms buffer */
    stream->buffer = (int16_t*)malloc(stream->bufferSize * sizeof(int16_t));
    if (stream->buffer == NULL) {
        (*stream->playerObject)->Destroy(stream->playerObject);
        free(stream);
        return NULL;
    }

    /* Start playing */
    result = (*stream->playerPlay)->SetPlayState(stream->playerPlay, SL_PLAYSTATE_PLAYING);
    if (result != SL_RESULT_SUCCESS) {
        free(stream->buffer);
        (*stream->playerObject)->Destroy(stream->playerObject);
        free(stream);
        return NULL;
    }

    return (void*)stream;
}

int opensl_stream_write(void* handle, const int16_t* samples, int frameCount) {
    OpenSLStream* stream = (OpenSLStream*)handle;
    if (stream == NULL) return 0;

    int bytesToWrite = frameCount * stream->channels * sizeof(int16_t);
    if (bytesToWrite > stream->bufferSize * (int)sizeof(int16_t)) {
        bytesToWrite = stream->bufferSize * (int)sizeof(int16_t);
    }

    memcpy(stream->buffer, samples, bytesToWrite);

    SLresult result = (*stream->bufferQueue)->Enqueue(
        stream->bufferQueue, stream->buffer, bytesToWrite
    );
    if (result != SL_RESULT_SUCCESS) {
        return 0;
    }

    return frameCount;
}

int opensl_stream_pause(void* handle) {
    OpenSLStream* stream = (OpenSLStream*)handle;
    if (stream == NULL) return -1;
    SLresult result = (*stream->playerPlay)->SetPlayState(
        stream->playerPlay, SL_PLAYSTATE_PAUSED
    );
    return (result == SL_RESULT_SUCCESS) ? 0 : -1;
}

int opensl_stream_resume(void* handle) {
    OpenSLStream* stream = (OpenSLStream*)handle;
    if (stream == NULL) return -1;
    SLresult result = (*stream->playerPlay)->SetPlayState(
        stream->playerPlay, SL_PLAYSTATE_PLAYING
    );
    return (result == SL_RESULT_SUCCESS) ? 0 : -1;
}

int opensl_stream_close(void* handle) {
    OpenSLStream* stream = (OpenSLStream*)handle;
    if (stream == NULL) return 0;

    if (stream->playerObject != NULL) {
        (*stream->playerPlay)->SetPlayState(stream->playerPlay, SL_PLAYSTATE_STOPPED);
        (*stream->playerObject)->Destroy(stream->playerObject);
    }
    if (stream->buffer != NULL) free(stream->buffer);
    free(stream);
    return 0;
}
