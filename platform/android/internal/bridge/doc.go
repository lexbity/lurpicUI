// Package bridge provides the JNI bridge between Android and Go.
//
// This package contains:
// - lurpic_android.c: C bridge with JNI glue
// - bridge_android.go: Go implementation (android build tag)
// - bridge_unavailable.go: Non-Android fallback (!android build tag)
//
// The matching Java NativeActivity extension (LurpicNativeActivity.java) is
// owned by the build tool, which compiles and dexes it into the APK; see
// cmd/lurpic/androidtmpl/LurpicNativeActivity.java. Its package and class name
// (org.lurpicui.bridge.LurpicNativeActivity) are a contract shared with the JNI
// symbols defined here.
//
// The bridge follows the Android threading model:
//   - Android UI thread: Receives OS callbacks, pushes events to queue
//   - Runtime thread: Drains event queue, processes events
//   - JNI thread attachment: Managed via pthread thread-local storage
//
// Event flow:
//  1. Android OS calls ANativeActivity callback (UI thread)
//  2. C bridge function pushes event to Go queue
//  3. Go runtime drains queue and dispatches events
//
// This is an internal package; external code should not import it.
package bridge
