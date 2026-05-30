/*
 * lurpic_android.c - JNI implementation and bridge between NativeActivity callbacks and Go.
 *
 * This file:
 * - Handles thread attachment for JNI
 * - Translates Android input events to a form Go can consume
 * - Manages the surface lifecycle
 * - Provides the ANativeActivity_onCreate entry point
 */

#include <android/choreographer.h>
#include <android/configuration.h>
#include <android/native_activity.h>
#include <android/native_window.h>
#include <android/input.h>
#include <android/keycodes.h>
#include <android/looper.h>
#include <android/log.h>
#include <jni.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <errno.h>
#include <time.h>

/* Logging macros */
#define LOGV(...) ((void)__android_log_print(ANDROID_LOG_VERBOSE, "LurpicBridge", __VA_ARGS__))
#define LOGI(...) ((void)__android_log_print(ANDROID_LOG_INFO, "LurpicBridge", __VA_ARGS__))
#define LOGW(...) ((void)__android_log_print(ANDROID_LOG_WARN, "LurpicBridge", __VA_ARGS__))
#define LOGE(...) ((void)__android_log_print(ANDROID_LOG_ERROR, "LurpicBridge", __VA_ARGS__))

/* Forward declarations for Go functions */
extern void goANativeActivityOnCreate(ANativeActivity* activity, void* savedState, size_t savedStateSize);
extern void goOnStart(ANativeActivity* activity);
extern void goOnResume(ANativeActivity* activity);
extern void goOnPause(ANativeActivity* activity);
extern void goOnStop(ANativeActivity* activity);
extern void goOnDestroy(ANativeActivity* activity);
extern void goOnWindowFocusChanged(ANativeActivity* activity, int focused);
extern void goOnNativeWindowCreated(ANativeActivity* activity, ANativeWindow* window);
extern void goOnNativeWindowDestroyed(ANativeActivity* activity, ANativeWindow* window);
extern void goOnNativeWindowResized(ANativeActivity* activity, ANativeWindow* window);
extern void goOnInputQueueCreated(ANativeActivity* activity, AInputQueue* queue);
extern void goOnInputQueueDestroyed(ANativeActivity* activity, AInputQueue* queue);
extern void goOnLowMemory(ANativeActivity* activity);
extern void goDeliverTouchEvent(int32_t pointerId, int32_t phase, float x, float y,
                                float pressure, float major, float minor,
                                int32_t source, int32_t deviceId, int32_t toolType,
                                int32_t buttonState, int64_t eventTime);
extern void goDeliverPointerEvent(int32_t pointerId, int32_t action, float x, float y,
                                  float pressure, float size,
                                  int32_t source, int32_t deviceId, int32_t toolType,
                                  int32_t buttonState, int64_t eventTime);
extern void goDeliverScrollEvent(float x, float y, float hScroll, float vScroll,
                                 int32_t source, int32_t deviceId, int64_t eventTime);
extern void goDeliverKeyEvent(int32_t keyCode, int32_t action, int32_t metaState,
                              int32_t source, int32_t deviceId, int64_t eventTime);
extern void goDeliverWindowInsets(int32_t top, int32_t bottom, int32_t left, int32_t right,
                                   int32_t cutoutLeft, int32_t cutoutTop,
                                   int32_t cutoutRight, int32_t cutoutBottom);
extern void goDeliverIMECompose(char* text, int32_t cursorPos);
extern void goDeliverIMECommit(char* text);
extern void goDeliverAudioFocusChange(int32_t focusChange);
extern void goDeliverVsync(int64_t frameTimeNanos);
extern void goDeliverConfigurationChanged(int32_t orientation, int32_t screenWidthDp,
                                           int32_t screenHeightDp, int32_t density,
                                           int32_t uiModeNight, float fontScale,
                                           const char* language, const char* country);
extern void goDeliverPermissionResult(int32_t requestCode, int32_t granted, int32_t permanent);

/* Thread-local storage for JNI environment */
static pthread_key_t jni_env_key;
static JavaVM* g_vm = NULL;
static ANativeActivity* g_activity = NULL;
static jobject g_activity_object = NULL;
static pthread_once_t jni_tls_once = PTHREAD_ONCE_INIT;

/* Destructor for thread-local JNI environment */
static void jni_env_destructor(void* env) {
    if (env != NULL && g_vm != NULL) {
        (*g_vm)->DetachCurrentThread(g_vm);
    }
}

/* Initialize thread-local storage once */
static void jni_tls_init(void) {
    pthread_key_create(&jni_env_key, jni_env_destructor);
}

static void ensure_jni_tls_initialized(void) {
    pthread_once(&jni_tls_once, jni_tls_init);
}

/* Get or attach JNI environment for the current thread */
JNIEnv* get_jni_env(void) {
    ensure_jni_tls_initialized();

    JNIEnv* env = pthread_getspecific(jni_env_key);
    if (env == NULL && g_vm != NULL) {
        jint attachResult = (*g_vm)->AttachCurrentThread(g_vm, &env, NULL);
        if (attachResult == JNI_OK) {
            pthread_setspecific(jni_env_key, env);
        }
    }
    return env;
}

/* Activity callback implementations */

static void onDestroy(ANativeActivity* activity) {
    LOGI("C: onDestroy called");
    goOnDestroy(activity);

    JNIEnv* env = get_jni_env();
    if (env != NULL && g_activity_object != NULL) {
        (*env)->DeleteGlobalRef(env, g_activity_object);
        g_activity_object = NULL;
    }

    /* Detach the main thread before exiting */
    if (g_vm != NULL) {
        (*g_vm)->DetachCurrentThread(g_vm);
    }
}

static void onStart(ANativeActivity* activity) {
    LOGI("C: onStart called");
    goOnStart(activity);
}

static void onResume(ANativeActivity* activity) {
    LOGI("C: onResume called");
    goOnResume(activity);
}

static void onPause(ANativeActivity* activity) {
    LOGI("C: onPause called");
    goOnPause(activity);
}

static void onStop(ANativeActivity* activity) {
    LOGI("C: onStop called");
    goOnStop(activity);
}

static void onConfigurationChanged(ANativeActivity* activity) {
    LOGI("C: onConfigurationChanged called");

    if (activity == NULL || activity->assetManager == NULL) {
        LOGW("onConfigurationChanged: no asset manager available");
        return;
    }

    AConfiguration* config = AConfiguration_new();
    if (config == NULL) {
        LOGE("onConfigurationChanged: AConfiguration_new failed");
        return;
    }

    AConfiguration_fromAssetManager(config, activity->assetManager);

    int32_t orientation   = AConfiguration_getOrientation(config);
    int32_t screenWidthDp = AConfiguration_getScreenWidthDp(config);
    int32_t screenHeightDp = AConfiguration_getScreenHeightDp(config);
    int32_t density       = AConfiguration_getDensity(config);
    int32_t uiModeNight   = AConfiguration_getUiModeNight(config);
    float   fontScale     = 0.0f; /* AConfiguration does not provide fontScale; default to 0 (unknown) */

    char language[3] = {0, 0, 0};
    char country[3]  = {0, 0, 0};
    AConfiguration_getLanguage(config, language); /* fills up to 2 chars + NUL */
    AConfiguration_getCountry(config, country);

    AConfiguration_delete(config);

    LOGI("onConfigurationChanged: orientation=%d density=%d uiModeNight=%d "
         "screen=%dx%ddp lang=%s country=%s",
         orientation, density, uiModeNight,
         screenWidthDp, screenHeightDp,
         language[0] ? language : "(none)",
         country[0] ? country : "(none)");

    goDeliverConfigurationChanged(orientation, screenWidthDp, screenHeightDp,
                                  density, uiModeNight, fontScale,
                                  language, country);
}

static void onWindowFocusChanged(ANativeActivity* activity, int focused) {
    LOGI("C: onWindowFocusChanged: %d", focused);
    goOnWindowFocusChanged(activity, focused);
}

static void onNativeWindowCreated(ANativeActivity* activity, ANativeWindow* window) {
    LOGI("C: onNativeWindowCreated called");
    goOnNativeWindowCreated(activity, window);
}

static void onNativeWindowDestroyed(ANativeActivity* activity, ANativeWindow* window) {
    LOGI("C: onNativeWindowDestroyed called");
    goOnNativeWindowDestroyed(activity, window);
}

static void onNativeWindowResized(ANativeActivity* activity, ANativeWindow* window) {
    LOGI("C: onNativeWindowResized called");
    goOnNativeWindowResized(activity, window);
}

static void onNativeWindowRedrawNeeded(ANativeActivity* activity, ANativeWindow* window) {
    LOGI("C: onNativeWindowRedrawNeeded called");
    /* Surface recreation, swapchain rebuild, and redraw are driven by the
     * Vulkan backend's recreate path (lurpic_render_recreate_surface_android)
     * through the native window lifecycle callbacks. This callback serves as
     * an additional hint that the window content needs repainting. */
}

static void onInputQueueCreated(ANativeActivity* activity, AInputQueue* queue) {
    LOGI("C: onInputQueueCreated called (queue=%p)", (void*)queue);
    goOnInputQueueCreated(activity, queue);
    /* Start the dedicated input thread that polls this queue via ALooper. */
    start_input_thread(queue);
}

static void onInputQueueDestroyed(ANativeActivity* activity, AInputQueue* queue) {
    LOGI("C: onInputQueueDestroyed called (queue=%p)", (void*)queue);
    goOnInputQueueDestroyed(activity, queue);
    /* Stop the input thread and detach the queue from its looper. */
    stop_input_thread();
}

/* ── Choreographer / vsync ──────────────────────────────────────────── */
/*
 * AChoreographer provides vsync-aligned frame callbacks for smooth
 * frame pacing. The callback fires on the Choreographer's dedicated
 * thread approximately once per vsync period (typically 16.6ms at 60 Hz,
 * 8.3ms at 120 Hz, etc.).
 */

static int g_vsync_registered = 0;

static void vsync_frame_callback(long frameTimeNanos, void* data) {
    (void)data;
    goDeliverVsync((int64_t)frameTimeNanos);

    /* Re-register for the next frame so we get continuous vsync events.
     * AChoreographer_postFrameCallback is thread-safe and can be called
     * from any thread. */
    if (g_vsync_registered) {
        AChoreographer* choreographer = AChoreographer_getInstance();
        if (choreographer != NULL) {
            AChoreographer_postFrameCallback(choreographer, vsync_frame_callback, NULL);
        }
    }
}

/* Start receiving vsync callbacks. Should be called when the app is
 * visible and actively rendering (e.g., onResume / onWindowFocusChanged). */
static void start_vsync(void) {
    if (g_vsync_registered) return;
    AChoreographer* choreographer = AChoreographer_getInstance();
    if (choreographer == NULL) {
        LOGW("Choreographer not available (pre-API 24 or no main looper)");
        return;
    }
    g_vsync_registered = 1;
    AChoreographer_postFrameCallback(choreographer, vsync_frame_callback, NULL);
    LOGI("Vsync callbacks started");
}

/* Stop receiving vsync callbacks. Should be called when the app is
 * paused or no longer visible. */
static void stop_vsync(void) {
    g_vsync_registered = 0;
    LOGI("Vsync callbacks stopped");
}

/* ── Input thread ────────────────────────────────────────────────────── */
/*
 * Android delivers input events through AInputQueue, which requires an
 * ALooper to poll. Rather than attaching to an arbitrary thread's looper
 * (the Go runtime thread does not have one), we run a dedicated input
 * thread that keeps its own ALooper for the lifetime of the input queue.
 */

#define LOOPER_ID_INPUT 1

/* State shared between the main (callback) thread and the input thread. */
static pthread_t       g_input_thread        = 0;
static volatile int    g_input_thread_running = 0;
static AInputQueue*    g_input_queue          = NULL;
static pthread_mutex_t g_input_mutex          = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t  g_input_cond           = PTHREAD_COND_INITIALIZER;

/* Signal the input thread to shut down and wait for it. Called on the main
 * thread from onInputQueueDestroyed.
 */
static void stop_input_thread(void) {
    pthread_mutex_lock(&g_input_mutex);
    if (!g_input_thread_running || g_input_thread == 0) {
        pthread_mutex_unlock(&g_input_mutex);
        return;
    }
    g_input_thread_running = 0;
    pthread_mutex_unlock(&g_input_mutex);

    /* Wake the looper by sending a dummy event. AInputQueue_detachLooper
     * will also break the poll, but we must ensure the thread sees the flag.
     */
    pthread_join(g_input_thread, NULL);
    g_input_thread = 0;
    LOGI("Input thread stopped");
}

/* The input thread body. Runs its own ALooper and drains AInputQueue. */
static void* input_thread_func(void* arg) {
    (void)arg;

    ALooper* looper = ALooper_prepare(ALOOPER_PREPARE_ALLOW_NON_CALLBACKS);
    if (looper == NULL) {
        LOGE("input thread: ALooper_prepare failed");
        return NULL;
    }

    pthread_mutex_lock(&g_input_mutex);
    AInputQueue* queue = g_input_queue;
    if (queue != NULL) {
        AInputQueue_attachLooper(queue, looper, LOOPER_ID_INPUT, NULL, NULL);
        LOGI("Input thread: attached looper to queue %p", (void*)queue);
    }
    pthread_mutex_unlock(&g_input_mutex);

    while (g_input_thread_running) {
        int events;
        struct ALooper_pollResult result;
        /* android-35 NDK has ALooper_pollOnce; we use ALooper_pollOnce
         * for compatibility. It returns the number of file descriptors
         * that had events, or ALOOPER_POLL_TIMEOUT (-1), or ALOOPER_POLL_ERROR.
         */
        int rc = ALooper_pollOnce(-1 /* timeout: infinite */,
                                  NULL /* outFd */,
                                  &events,
                                  NULL /* outData */);

        if (!g_input_thread_running) break;

        if (rc == LOOPER_ID_INPUT && queue != NULL) {
            /* Drain all available events from the input queue. */
            AInputEvent* inputEvent = NULL;
            while (AInputQueue_getEvent(queue, &inputEvent) >= 0) {
                if (inputEvent == NULL) break;

                int32_t handled = 0;
                if (AInputQueue_preDispatchEvent(queue, inputEvent)) {
                    /* preDispatchEvent returned true → the event was consumed
                     * (e.g. by IME). Do not call finishEvent yet; it will be
                     * finished by the IME thread. */
                    continue;
                }

                handled = handle_input_event(inputEvent);
                AInputQueue_finishEvent(queue, inputEvent, handled);
            }
        } else if (rc == ALOOPER_POLL_ERROR) {
            LOGE("input thread: ALooper_pollOnce returned ALOOPER_POLL_ERROR");
            break;
        }
    }

    /* Detach the input queue from the looper before exiting. */
    pthread_mutex_lock(&g_input_mutex);
    if (queue != NULL) {
        AInputQueue_detachLooper(queue);
        LOGI("Input thread: detached looper from queue");
    }
    pthread_mutex_unlock(&g_input_mutex);

    LOGI("Input thread: exiting");
    return NULL;
}

/* Start the input thread. Called from onInputQueueCreated on the main thread.
 * The thread is responsible for attaching the queue to its ALooper.
 */
static void start_input_thread(AInputQueue* queue) {
    pthread_mutex_lock(&g_input_mutex);
    if (g_input_thread_running) {
        LOGW("Input thread already running; detaching old queue first");
        pthread_mutex_unlock(&g_input_mutex);
        stop_input_thread();
        pthread_mutex_lock(&g_input_mutex);
    }

    g_input_queue    = queue;
    g_input_thread_running = 1;
    pthread_mutex_unlock(&g_input_mutex);

    if (pthread_create(&g_input_thread, NULL, input_thread_func, NULL) != 0) {
        LOGE("Failed to create input thread: %s", strerror(errno));
        pthread_mutex_lock(&g_input_mutex);
        g_input_thread_running = 0;
        g_input_queue = NULL;
        pthread_mutex_unlock(&g_input_mutex);
        return;
    }
    LOGI("Input thread started for queue %p", (void*)queue);
}

/* ── Source classifier ───────────────────────────────────────────────── */
/* Classify the motion event source to determine routing. Touchscreen events
 * go through the touch pipeline (multi-touch, gesture recognition) while
 * mouse, stylus, and trackpad events go through the pointer pipeline (click,
 * right-click, hover, scroll, pressure/tilt). */
#define SOURCE_CLASS_TOUCH     0  /* AINPUT_SOURCE_TOUCHSCREEN */
#define SOURCE_CLASS_POINTER   1  /* AINPUT_SOURCE_MOUSE, _STYLUS, _TOUCHPAD */
#define SOURCE_CLASS_UNKNOWN   2

static int source_class(int32_t source) {
    if (source & 0x00001002) return SOURCE_CLASS_TOUCH;   /* AINPUT_SOURCE_TOUCHSCREEN (0x1002) */
    if (source & 0x00002002) return SOURCE_CLASS_POINTER; /* AINPUT_SOURCE_MOUSE (0x2002) */
    if (source & 0x00004002) return SOURCE_CLASS_POINTER; /* AINPUT_SOURCE_STYLUS (0x4002) */
    if (source == 0x100008)  return SOURCE_CLASS_POINTER; /* AINPUT_SOURCE_TOUCHPAD */
    if (source & 0x00000001) return SOURCE_CLASS_POINTER; /* AINPUT_SOURCE_CLASS_POINTER */
    return SOURCE_CLASS_UNKNOWN;
}

/* ── Button helper ────────────────────────────────────────────────────── */
static int pointer_button_from_bstate(int32_t buttonState) {
    /* Android button state bits map directly to AMOTION_EVENT_BUTTON_*
     * constants: PRIMARY (1), SECONDARY (2), TERTIARY (4), BACK (8), FORWARD (16). */
    if (buttonState & 1) return 0;  /* Left/Primary */
    if (buttonState & 2) return 1;  /* Right/Secondary */
    if (buttonState & 4) return 2;  /* Middle/Tertiary */
    return 0;
}

/* ── Shared event-time helper ─────────────────────────────────────────── */
static int64_t get_event_time_ns(AInputEvent* event) {
#if defined(__ANDROID_API__) && __ANDROID_API__ >= 9
    return (int64_t)AInputEvent_getEventTime(event);
#else
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (int64_t)ts.tv_sec * 1000000000LL + (int64_t)ts.tv_nsec;
#endif
}

/* ── Historical sample delivery for touch events ──────────────────────── */
static void deliver_touch_historical(AInputEvent* event, int32_t source,
                                     int32_t deviceId, int32_t buttonState) {
    size_t historySize = AMotionEvent_getHistorySize(event);
    int32_t pointerCount = AMotionEvent_getPointerCount(event);
    for (size_t h = 0; h < historySize; h++) {
        float histEventTime = AMotionEvent_getHistoricalEventTime(event, h);
        for (int32_t i = 0; i < pointerCount; i++) {
            int32_t pointerId = AMotionEvent_getPointerId(event, i);
            goDeliverTouchEvent(
                pointerId, 1 /* TouchMove */,
                AMotionEvent_getHistoricalX(event, i, h),
                AMotionEvent_getHistoricalY(event, i, h),
                AMotionEvent_getHistoricalPressure(event, i, h),
                AMotionEvent_getHistoricalTouchMajor(event, i, h),
                AMotionEvent_getHistoricalTouchMinor(event, i, h),
                source, deviceId,
                AMotionEvent_getHistoricalToolType(event, i, h),
                buttonState, (int64_t)histEventTime);
        }
    }
}

/* ── Historical sample delivery for pointer events (mouse/stylus) ─────── */
static void deliver_pointer_historical(AInputEvent* event, int32_t action,
                                       int32_t source, int32_t deviceId,
                                       int32_t buttonState) {
    size_t historySize = AMotionEvent_getHistorySize(event);
    int32_t pointerCount = AMotionEvent_getPointerCount(event);
    for (size_t h = 0; h < historySize; h++) {
        float histEventTime = AMotionEvent_getHistoricalEventTime(event, h);
        for (int32_t i = 0; i < pointerCount; i++) {
            int32_t pointerId = AMotionEvent_getPointerId(event, i);
            goDeliverPointerEvent(
                pointerId, action,
                AMotionEvent_getHistoricalX(event, i, h),
                AMotionEvent_getHistoricalY(event, i, h),
                AMotionEvent_getHistoricalPressure(event, i, h),
                AMotionEvent_getHistoricalSize(event, i, h),
                source, deviceId,
                AMotionEvent_getHistoricalToolType(event, i, h),
                buttonState, (int64_t)histEventTime);
        }
    }
}

/* Input event processing */
static int32_t handle_input_event(AInputEvent* event) {
    int32_t type = AInputEvent_getType(event);
    int32_t source = AInputEvent_getSource(event);
    int32_t deviceId = AInputEvent_getDeviceId(event);
    int64_t eventTime = get_event_time_ns(event);

    LOGV("handle_input_event: type=%d source=0x%x deviceId=%d",
         type, source, deviceId);

    if (type == AINPUT_EVENT_TYPE_MOTION) {
        int32_t action = AMotionEvent_getAction(event);
        int32_t actionMasked = action & AMOTION_EVENT_ACTION_MASK;
        int32_t actionPointerIndex = (action & AMOTION_EVENT_ACTION_POINTER_INDEX_MASK) >>
                                     AMOTION_EVENT_ACTION_POINTER_INDEX_SHIFT;
        int32_t pointerCount = AMotionEvent_getPointerCount(event);
        int32_t buttonState = AMotionEvent_getButtonState(event);
        int cls = source_class(source);

        /* ── SCROLL (mouse wheel / trackpad two-finger) ────────────── */
        if (actionMasked == AMOTION_EVENT_ACTION_SCROLL) {
            float vScroll = AMotionEvent_getAxisValue(event, AMOTION_EVENT_AXIS_VSCROLL, 0);
            float hScroll = AMotionEvent_getAxisValue(event, AMOTION_EVENT_AXIS_HSCROLL, 0);
            float x = AMotionEvent_getX(event, 0);
            float y = AMotionEvent_getY(event, 0);
            goDeliverScrollEvent(x, y, hScroll, vScroll, source, deviceId, eventTime);
            return 1;
        }

        /* ── HOVER (stylus hover, mouse move without button) ───────── */
        if (actionMasked == AMOTION_EVENT_ACTION_HOVER_MOVE ||
            actionMasked == AMOTION_EVENT_ACTION_HOVER_ENTER ||
            actionMasked == AMOTION_EVENT_ACTION_HOVER_EXIT) {
            for (int32_t i = 0; i < pointerCount && i < 1; i++) {
                int32_t pointerId = AMotionEvent_getPointerId(event, i);
                float x = AMotionEvent_getX(event, i);
                float y = AMotionEvent_getY(event, i);
                float pressure = AMotionEvent_getPressure(event, i);
                float size = AMotionEvent_getSize(event, i);
                int32_t toolType = AMotionEvent_getToolType(event, i);
                /* Hover events carry 0 for buttonState; use ACTION_MOVE
                 * as the hover action kind and let Go distinguish hover
                 * from drag via the source class. */
                goDeliverPointerEvent(pointerId, actionMasked,
                                      x, y, pressure, size,
                                      source, deviceId, toolType,
                                      0, eventTime);
            }
            return 1;
        }

        /* ── TOUCH / POINTER (down, move, up, cancel) ─────────────── */
        if (cls == SOURCE_CLASS_POINTER) {
            /* Mouse and stylus events → pointer pipeline. */
            for (int32_t i = 0; i < pointerCount && i < 1; i++) {
                int32_t pointerId = AMotionEvent_getPointerId(event, i);
                float x = AMotionEvent_getX(event, i);
                float y = AMotionEvent_getY(event, i);
                float pressure = AMotionEvent_getPressure(event, i);
                float size = AMotionEvent_getSize(event, i);
                int32_t toolType = AMotionEvent_getToolType(event, i);

                /* Determine the pointer action: for POINTER_DOWN/UP the
                 * action encodes which pointer index; for our single-pointer
                 * mouse/stylus we map DOWN→PRESS, UP→RELEASE, MOVE→MOVE. */
                int ptrAction;
                switch (actionMasked) {
                    case AMOTION_EVENT_ACTION_DOWN:
                    case AMOTION_EVENT_ACTION_POINTER_DOWN:
                        ptrAction = 0; /* PointerPress */
                        break;
                    case AMOTION_EVENT_ACTION_UP:
                    case AMOTION_EVENT_ACTION_POINTER_UP:
                        ptrAction = 1; /* PointerRelease */
                        break;
                    case AMOTION_EVENT_ACTION_MOVE:
                        ptrAction = 2; /* PointerMove */
                        break;
                    case AMOTION_EVENT_ACTION_CANCEL:
                        ptrAction = 3; /* PointerCancel */
                        break;
                    default:
                        ptrAction = 2; /* PointerMove fallback */
                        break;
                }

                goDeliverPointerEvent(pointerId, ptrAction,
                                      x, y, pressure, size,
                                      source, deviceId, toolType,
                                      buttonState, eventTime);
            }

            /* Historical samples for pointer events. */
            if (actionMasked == AMOTION_EVENT_ACTION_MOVE) {
                deliver_pointer_historical(event, 2 /* PointerMove */,
                                           source, deviceId, buttonState);
            }
        } else {
            /* Touchscreen → touch pipeline (multi-touch with pointer IDs). */
            for (int32_t i = 0; i < pointerCount; i++) {
                int32_t pointerId = AMotionEvent_getPointerId(event, i);
                float x = AMotionEvent_getX(event, i);
                float y = AMotionEvent_getY(event, i);
                float pressure = AMotionEvent_getPressure(event, i);
                float major = AMotionEvent_getTouchMajor(event, i);
                float minor = AMotionEvent_getTouchMinor(event, i);
                int32_t toolType = AMotionEvent_getToolType(event, i);

                int phase;
                switch (actionMasked) {
                    case AMOTION_EVENT_ACTION_DOWN:
                    case AMOTION_EVENT_ACTION_POINTER_DOWN:
                        phase = (i == actionPointerIndex) ? 0 : 1;
                        break;
                    case AMOTION_EVENT_ACTION_MOVE:
                        phase = 1;
                        break;
                    case AMOTION_EVENT_ACTION_UP:
                    case AMOTION_EVENT_ACTION_POINTER_UP:
                        phase = (i == actionPointerIndex) ? 2 : 1;
                        break;
                    case AMOTION_EVENT_ACTION_CANCEL:
                        phase = 3;
                        break;
                    default:
                        phase = 1;
                        break;
                }

                goDeliverTouchEvent(pointerId, phase, x, y, pressure, major, minor,
                                    source, deviceId, toolType, buttonState, eventTime);
            }

            /* Historical samples for touch events. */
            if (actionMasked == AMOTION_EVENT_ACTION_MOVE) {
                deliver_touch_historical(event, source, deviceId, buttonState);
            }
        }
        return 1;

    } else if (type == AINPUT_EVENT_TYPE_KEY) {
        int32_t keyCode = AKeyEvent_getKeyCode(event);
        int32_t action = AKeyEvent_getAction(event);
        int32_t metaState = AKeyEvent_getMetaState(event);

        goDeliverKeyEvent(keyCode, action, metaState, source, deviceId, eventTime);
        return 1;
    }

    return 0; /* Event not handled */
}

static void call_activity_method(const char* name, const char* sig) {
    JNIEnv* env = get_jni_env();
    if (env == NULL || g_activity == NULL || g_activity_object == NULL) {
        return;
    }
    jclass cls = (*env)->GetObjectClass(env, g_activity_object);
    if (cls == NULL) {
        return;
    }
    jmethodID method = (*env)->GetMethodID(env, cls, name, sig);
    if (method != NULL) {
        (*env)->CallVoidMethod(env, g_activity_object, method);
    }
    (*env)->DeleteLocalRef(env, cls);
}

static void call_activity_request_permission(const char* permission, jint requestCode) {
    JNIEnv* env = get_jni_env();
    if (env == NULL || g_activity == NULL || g_activity_object == NULL) {
        return;
    }
    jclass cls = (*env)->GetObjectClass(env, g_activity_object);
    if (cls == NULL) {
        return;
    }
    jmethodID method = (*env)->GetMethodID(env, cls, "requestPermission", "(Ljava/lang/String;I)V");
    if (method != NULL) {
        jstring jpermission = (*env)->NewStringUTF(env, permission);
        if (jpermission != NULL) {
            (*env)->CallVoidMethod(env, g_activity_object, method, jpermission, requestCode);
            (*env)->DeleteLocalRef(env, jpermission);
        }
    }
    (*env)->DeleteLocalRef(env, cls);
}

static jint call_activity_check_permission(const char* permission) {
    JNIEnv* env = get_jni_env();
    if (env == NULL || g_activity == NULL || g_activity_object == NULL) {
        return 0;
    }
    jclass cls = (*env)->GetObjectClass(env, g_activity_object);
    if (cls == NULL) {
        return 0;
    }
    jmethodID method = (*env)->GetMethodID(env, cls, "checkPermission", "(Ljava/lang/String;)I");
    jint result = 0;
    if (method != NULL) {
        jstring jpermission = (*env)->NewStringUTF(env, permission);
        if (jpermission != NULL) {
            result = (*env)->CallIntMethod(env, g_activity_object, method, jpermission);
            (*env)->DeleteLocalRef(env, jpermission);
        }
    }
    (*env)->DeleteLocalRef(env, cls);
    return result;
}

static jint call_activity_has_declared_permission(const char* permission) {
    JNIEnv* env = get_jni_env();
    if (env == NULL || g_activity == NULL || g_activity_object == NULL) {
        return 0;
    }
    jclass cls = (*env)->GetObjectClass(env, g_activity_object);
    if (cls == NULL) {
        return 0;
    }
    jmethodID method = (*env)->GetMethodID(env, cls, "hasDeclaredPermission", "(Ljava/lang/String;)Z");
    jint result = 0;
    if (method != NULL) {
        jstring jpermission = (*env)->NewStringUTF(env, permission);
        if (jpermission != NULL) {
            result = (*env)->CallBooleanMethod(env, g_activity_object, method, jpermission) ? 1 : 0;
            (*env)->DeleteLocalRef(env, jpermission);
        }
    }
    (*env)->DeleteLocalRef(env, cls);
    return result;
}

void bridgeShowSoftKeyboard(void) {
    call_activity_method("showSoftKeyboard", "()V");
}

void bridgeHideSoftKeyboard(void) {
    call_activity_method("hideSoftKeyboard", "()V");
}

void bridgeRequestPermission(const char* permission, jint requestCode) {
    call_activity_request_permission(permission, requestCode);
}

jint bridgeCheckPermission(const char* permission) {
    return call_activity_check_permission(permission);
}

jint bridgeIsPermissionDeclared(const char* permission) {
    return call_activity_has_declared_permission(permission);
}

void bridgeVsyncStart(void) {
    start_vsync();
}

void bridgeVsyncStop(void) {
    stop_vsync();
}

JNIEXPORT void JNICALL Java_org_lurpicui_bridge_LurpicNativeActivity_nativeImeCompose(
    JNIEnv* env, jobject thiz, jstring text, jint cursorPos) {
    (void)env;
    (void)thiz;
    const char* utf = text != NULL ? (*env)->GetStringUTFChars(env, text, NULL) : NULL;
    char* copy = NULL;
    if (utf != NULL) {
        copy = strdup(utf);
        (*env)->ReleaseStringUTFChars(env, text, utf);
    }
    goDeliverIMECompose(copy != NULL ? copy : "", cursorPos);
    free(copy);
}

JNIEXPORT void JNICALL Java_org_lurpicui_bridge_LurpicNativeActivity_nativeImeCommit(
    JNIEnv* env, jobject thiz, jstring text) {
    (void)thiz;
    const char* utf = text != NULL ? (*env)->GetStringUTFChars(env, text, NULL) : NULL;
    char* copy = NULL;
    if (utf != NULL) {
        copy = strdup(utf);
        (*env)->ReleaseStringUTFChars(env, text, utf);
    }
    goDeliverIMECommit(copy != NULL ? copy : "");
    free(copy);
}

JNIEXPORT void JNICALL Java_org_lurpicui_bridge_LurpicNativeActivity_nativeImeKeyEvent(
    JNIEnv* env, jobject thiz, jint keyCode, jint action, jint metaState) {
    (void)env;
    (void)thiz;
    /* IME key events have source AINPUT_SOURCE_KEYBOARD, device 0, and use
     * the current monotonic time as eventTime since the Java layer does not
     * provide an Android eventTime. */
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    int64_t now = (int64_t)ts.tv_sec * 1000000000LL + (int64_t)ts.tv_nsec;
    goDeliverKeyEvent(keyCode, action, metaState,
                      0x101 /* AINPUT_SOURCE_KEYBOARD = 0x101 */,
                      0, now);
}

JNIEXPORT void JNICALL Java_org_lurpicui_bridge_LurpicNativeActivity_nativeOnAudioFocusChange(
    JNIEnv* env, jobject thiz, jint focusChange) {
    (void)env;
    (void)thiz;
    goDeliverAudioFocusChange(focusChange);
}

JNIEXPORT void JNICALL Java_org_lurpicui_bridge_LurpicNativeActivity_nativeOnWindowInsets(
    JNIEnv* env, jobject thiz,
    jint top, jint bottom, jint left, jint right,
    jint cutoutLeft, jint cutoutTop, jint cutoutRight, jint cutoutBottom) {
    (void)env;
    (void)thiz;
    goDeliverWindowInsets(top, bottom, left, right,
                          cutoutLeft, cutoutTop, cutoutRight, cutoutBottom);
}

JNIEXPORT void JNICALL Java_org_lurpicui_bridge_LurpicNativeActivity_nativePermissionResult(
    JNIEnv* env, jobject thiz, jint requestCode, jboolean granted, jboolean permanent) {
    (void)env;
    (void)thiz;
    goDeliverPermissionResult(requestCode, granted ? 1 : 0, permanent ? 1 : 0);
}

/* Main entry point - called by Android when the activity starts */
void ANativeActivity_onCreate(ANativeActivity* activity, void* savedState, size_t savedStateSize) {
    LOGI("C: ANativeActivity_onCreate called");
    g_activity = activity;

    /* Cache the JVM for JNI calls */
    JNIEnv* env = activity->env;
    (*env)->GetJavaVM(env, &g_vm);
    g_activity_object = (*env)->NewGlobalRef(env, activity->clazz);

    /* Initialize thread-local storage */
    ensure_jni_tls_initialized();
    pthread_setspecific(jni_env_key, env);

    /* Set up all the callbacks */
    activity->callbacks->onDestroy = onDestroy;
    activity->callbacks->onStart = onStart;
    activity->callbacks->onResume = onResume;
    activity->callbacks->onPause = onPause;
    activity->callbacks->onStop = onStop;
    activity->callbacks->onConfigurationChanged = onConfigurationChanged;
    activity->callbacks->onWindowFocusChanged = onWindowFocusChanged;
    activity->callbacks->onNativeWindowCreated = onNativeWindowCreated;
    activity->callbacks->onNativeWindowDestroyed = onNativeWindowDestroyed;
    activity->callbacks->onNativeWindowResized = onNativeWindowResized;
    activity->callbacks->onNativeWindowRedrawNeeded = onNativeWindowRedrawNeeded;
    activity->callbacks->onInputQueueCreated = onInputQueueCreated;
    activity->callbacks->onInputQueueDestroyed = onInputQueueDestroyed;

    /* Call into Go to initialize the runtime */
    goANativeActivityOnCreate(activity, savedState, savedStateSize);
}
