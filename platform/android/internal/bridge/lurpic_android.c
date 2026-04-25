/*
 * lurpic_android.c - JNI implementation and bridge between NativeActivity callbacks and Go.
 *
 * This file:
 * - Handles thread attachment for JNI
 * - Translates Android input events to a form Go can consume
 * - Manages the surface lifecycle
 * - Provides the ANativeActivity_onCreate entry point
 */

#include <android/native_activity.h>
#include <android/native_window.h>
#include <android/input.h>
#include <android/keycodes.h>
#include <android/log.h>
#include <jni.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

/* Logging macros */
#define LOGI(...) ((void)__android_log_print(ANDROID_LOG_INFO, "LurpicBridge", __VA_ARGS__))
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
                                float pressure, float major, float minor);
extern void goDeliverKeyEvent(int32_t keyCode, int32_t action, int32_t metaState);
extern void goDeliverIMECompose(char* text, int32_t cursorPos);
extern void goDeliverIMECommit(char* text);
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
    /* TODO: Handle configuration changes */
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
    /* TODO: Trigger redraw */
}

static void onInputQueueCreated(ANativeActivity* activity, AInputQueue* queue) {
    LOGI("C: onInputQueueCreated called");
    goOnInputQueueCreated(activity, queue);
}

static void onInputQueueDestroyed(ANativeActivity* activity, AInputQueue* queue) {
    LOGI("C: onInputQueueDestroyed called");
    goOnInputQueueDestroyed(activity, queue);
}

/* Input event processing */
static int32_t handle_input_event(AInputEvent* event) {
    int32_t type = AInputEvent_getType(event);

    if (type == AINPUT_EVENT_TYPE_MOTION) {
        int32_t action = AMotionEvent_getAction(event);
        int32_t pointerCount = AMotionEvent_getPointerCount(event);

        for (int32_t i = 0; i < pointerCount; i++) {
            int32_t pointerId = AMotionEvent_getPointerId(event, i);
            float x = AMotionEvent_getX(event, i);
            float y = AMotionEvent_getY(event, i);
            float pressure = AMotionEvent_getPressure(event, i);
            float major = AMotionEvent_getTouchMajor(event, i);
            float minor = AMotionEvent_getTouchMinor(event, i);

            /* Determine phase from action */
            int32_t actionMasked = action & AMOTION_EVENT_ACTION_MASK;
            int32_t actionPointerIndex = (action & AMOTION_EVENT_ACTION_POINTER_INDEX_MASK) >>
                                         AMOTION_EVENT_ACTION_POINTER_INDEX_SHIFT;

            int phase = 0; /* TouchDown */
            switch (actionMasked) {
                case AMOTION_EVENT_ACTION_DOWN:
                case AMOTION_EVENT_ACTION_POINTER_DOWN:
                    if (i == actionPointerIndex) {
                        phase = 0; /* TouchDown */
                    } else {
                        phase = 1; /* TouchMove */
                    }
                    break;
                case AMOTION_EVENT_ACTION_MOVE:
                    phase = 1; /* TouchMove */
                    break;
                case AMOTION_EVENT_ACTION_UP:
                case AMOTION_EVENT_ACTION_POINTER_UP:
                    if (i == actionPointerIndex) {
                        phase = 2; /* TouchUp */
                    } else {
                        phase = 1; /* TouchMove */
                    }
                    break;
                case AMOTION_EVENT_ACTION_CANCEL:
                    phase = 3; /* TouchCancel */
                    break;
            }

            /* Call Go function to deliver touch event */
            goDeliverTouchEvent(pointerId, phase, x, y, pressure, major, minor);
        }
        return 1; /* Event handled */
    } else if (type == AINPUT_EVENT_TYPE_KEY) {
        int32_t keyCode = AKeyEvent_getKeyCode(event);
        int32_t action = AKeyEvent_getAction(event);
        int32_t metaState = AKeyEvent_getMetaState(event);

        goDeliverKeyEvent(keyCode, action, metaState);
        return 1; /* Event handled */
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
    goDeliverKeyEvent(keyCode, action, metaState);
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
