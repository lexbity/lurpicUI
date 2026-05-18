// Package android provides the Android platform backend for lurpicUI.
//
// The Android backend implements the platform.App interface and provides:
// - Native bridge via NativeActivity (C + Java)
// - Event queue for lifecycle and input events
// - Touch input handling
// - Surface lifecycle management plus Vulkan surface creation hooks
//
// The backend uses the internal/bridge package for JNI communication with Android
// and the apiNN subpackages for API-level-specific behavior registration.
//
// Build tags:
//   - android: Builds the real Android implementation with CGO
//   - !android: Builds the non-Android fallback that returns platform errors
//
// Architecture:
//
//	Android UI Thread → C Bridge → Go Event Queue → lurpicUI Runtime
//
// The Android UI thread receives lifecycle callbacks from the OS, which are
// captured in the C bridge and pushed into a thread-safe Go event queue.
// The runtime drains this queue on its own thread.
//
// API-level support is registered via blank-imported platform/android/apiNN
// subpackages. The parent package resolves the best implementation for the
// target SDK at build/runtime.
package android
