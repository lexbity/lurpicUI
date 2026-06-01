package org.lurpicui.bridge;

import android.app.NativeActivity;
import android.content.Context;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.content.res.Configuration;
import android.media.AudioAttributes;
import android.media.AudioFocusRequest;
import android.media.AudioManager;
import android.os.Build;
import android.os.Bundle;
import android.util.Log;
import android.text.InputType;
import android.view.KeyEvent;
import android.view.Menu;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.view.inputmethod.BaseInputConnection;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputConnection;
import android.view.inputmethod.InputMethodManager;
import android.window.OnBackInvokedCallback;
import android.window.OnBackInvokedDispatcher;
import android.view.WindowMetrics;
import android.graphics.Point;
import android.graphics.Rect;
import android.view.Display;
import android.view.WindowManager;
import android.widget.FrameLayout;
import android.widget.TextView;

/**
 * LurpicNativeActivity extends NativeActivity to provide the entry point
 * for lurpicUI applications on Android.
 */
public class LurpicNativeActivity extends NativeActivity implements AudioManager.OnAudioFocusChangeListener {
    private static final String TAG = "LurpicNativeActivity";

    private FrameLayout imeRoot;
    private ImeInputView imeView;
    private FrameLayout splashRoot;
    private TextView splashText;
    private AudioManager audioManager;
    private AudioFocusRequest audioFocusRequest;

    // --- DEBUG instrumentation (recursion hunt) ---
    private int dbgInsetsDepth = 0;
    private int dbgPanelDepth = 0;

    static {
        try {
            System.loadLibrary("go");
            Log.i(TAG, "Loaded Go shared library");
        } catch (UnsatisfiedLinkError e) {
            Log.e(TAG, "Failed to load Go shared library", e);
            throw e;
        }
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        Log.i(TAG, "onCreate called");
        super.onCreate(savedInstanceState);

        // Edge-to-edge: draw under system bars so the framework can manage
        // insets ourselves. This is mandatory for targetSdk >= 35.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            getWindow().setDecorFitsSystemWindows(false);
        } else {
            getWindow()
                .getDecorView()
                .setSystemUiVisibility(
                    View.SYSTEM_UI_FLAG_LAYOUT_STABLE
                        | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
                );
        }

        // Listen for window insets (status bar, nav bar, cutout, IME) and
        // forward them to the native bridge for the runtime's safe-area store.
        getWindow().getDecorView().setOnApplyWindowInsetsListener(
            (View v, WindowInsets insets) -> {
                dbgInsetsDepth++;
                if (dbgInsetsDepth <= 3 || dbgInsetsDepth % 50 == 0) {
                    Log.w(TAG, "DBG insets listener depth=" + dbgInsetsDepth
                            + " thread=" + Thread.currentThread().getName());
                }
                if (dbgInsetsDepth == 4) {
                    Log.w(TAG, "DBG insets recursion stack:\n"
                            + Log.getStackTraceString(new Throwable()));
                }
                int inTop    = insets.getSystemWindowInsetTop();
                int inBottom = insets.getSystemWindowInsetBottom();
                int inLeft   = insets.getSystemWindowInsetLeft();
                int inRight  = insets.getSystemWindowInsetRight();
                int cutoutLeft = 0, cutoutTop = 0, cutoutRight = 0, cutoutBottom = 0;
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                    android.view.DisplayCutout cutout = insets.getDisplayCutout();
                    if (cutout != null) {
                        cutoutLeft   = cutout.getSafeInsetLeft();
                        cutoutTop    = cutout.getSafeInsetTop();
                        cutoutRight  = cutout.getSafeInsetRight();
                        cutoutBottom = cutout.getSafeInsetBottom();
                    }
                }
                nativeOnWindowInsets(inTop, inBottom, inLeft, inRight,
                                     cutoutLeft, cutoutTop, cutoutRight, cutoutBottom);
                WindowInsets dbgResult = v.onApplyWindowInsets(insets);
                dbgInsetsDepth--;
                return dbgResult;
            }
        );

        // Register predictive back callback (API 33+). The system shows an
        // animated back arrow/gesture and calls onBackInvoked when the user
        // completes the back gesture.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                OnBackInvokedDispatcher.PRIORITY_DEFAULT,
                () -> {
                    Log.i(TAG, "Predictive back invoked");
                    nativeOnBackInvoked();
                }
            );
        }

        // Set up audio focus listener for interruption handling.
        audioManager = (AudioManager) getSystemService(Context.AUDIO_SERVICE);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            AudioAttributes attrs = new AudioAttributes.Builder()
                .setUsage(AudioAttributes.USAGE_GAME)
                .setContentType(AudioAttributes.CONTENT_TYPE_MUSIC)
                .build();
            audioFocusRequest = new AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
                .setAudioAttributes(attrs)
                .setOnAudioFocusChangeListener(this)
                .build();
        }

        splashText = new TextView(this);
        splashText.setText("Loading...");
        splashText.setTextSize(18);
        splashText.setTextColor(0xFFFFFFFF);
        splashText.setGravity(android.view.Gravity.CENTER);
        splashText.setVisibility(android.view.View.GONE);

        splashRoot = new FrameLayout(this);
        splashRoot.setBackgroundColor(0xFF1A1A2E);
        splashRoot.addView(splashText, new FrameLayout.LayoutParams(
            FrameLayout.LayoutParams.WRAP_CONTENT,
            FrameLayout.LayoutParams.WRAP_CONTENT,
            android.view.Gravity.CENTER
        ));

        imeView = new ImeInputView(this);
        imeRoot = new FrameLayout(this);
        imeRoot.setFocusable(false);
        imeRoot.setFocusableInTouchMode(false);
        imeRoot.addView(imeView, new FrameLayout.LayoutParams(1, 1));
        imeRoot.addView(splashRoot, new FrameLayout.LayoutParams(
            FrameLayout.LayoutParams.MATCH_PARENT,
            FrameLayout.LayoutParams.MATCH_PARENT
        ));
        setContentView(imeRoot);
    }

    // --- DEBUG: panel/menu callbacks (recursion hunt) ---
    @Override
    public boolean onCreatePanelMenu(int featureId, Menu menu) {
        dbgPanelDepth++;
        if (dbgPanelDepth <= 3 || dbgPanelDepth % 50 == 0) {
            Log.w(TAG, "DBG onCreatePanelMenu feature=" + featureId + " depth=" + dbgPanelDepth);
        }
        boolean r = super.onCreatePanelMenu(featureId, menu);
        dbgPanelDepth--;
        return r;
    }

    @Override
    public boolean onPreparePanel(int featureId, View view, Menu menu) {
        dbgPanelDepth++;
        Log.w(TAG, "DBG onPreparePanel feature=" + featureId + " depth=" + dbgPanelDepth
                + " javaFrames=" + Thread.currentThread().getStackTrace().length);
        if (dbgPanelDepth == 4) {
            Log.w(TAG, "DBG panel recursion stack:\n" + Log.getStackTraceString(new Throwable()));
        }
        boolean r = super.onPreparePanel(featureId, view, menu);
        dbgPanelDepth--;
        return r;
    }

    @Override
    public void onContentChanged() {
        Log.w(TAG, "DBG onContentChanged thread=" + Thread.currentThread().getName());
        super.onContentChanged();
    }

    @Override
    protected void onSaveInstanceState(Bundle outState) {
        Log.i(TAG, "onSaveInstanceState called");
        super.onSaveInstanceState(outState);
        // Fetch serialized view state from the Go runtime and store it
        // in the bundle so it survives process death.
        byte[] state = nativeGetSavedState();
        if (state != null && state.length > 0) {
            outState.putByteArray("lurpic_view_state", state);
        }
    }

    @Override
    protected void onRestoreInstanceState(Bundle savedInstanceState) {
        Log.i(TAG, "onRestoreInstanceState called");
        super.onRestoreInstanceState(savedInstanceState);
        if (savedInstanceState != null) {
            byte[] state = savedInstanceState.getByteArray("lurpic_view_state");
            if (state != null) {
                nativeSetSavedState(state);
            }
        }
    }

    @Override
    protected void onStart() {
        Log.i(TAG, "onStart called");
        super.onStart();
    }

    @Override
    protected void onResume() {
        Log.i(TAG, "onResume called");
        super.onResume();
    }

    @Override
    protected void onPause() {
        Log.i(TAG, "onPause called");
        super.onPause();
    }

    @Override
    protected void onStop() {
        Log.i(TAG, "onStop called");
        super.onStop();
    }

    @Override
    protected void onDestroy() {
        Log.i(TAG, "onDestroy called");
        super.onDestroy();
    }

    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        Log.i(TAG, "onWindowFocusChanged: " + hasFocus);
        super.onWindowFocusChanged(hasFocus);
    }

    @Override
    public void onMultiWindowModeChanged(boolean isInMultiWindowMode, Configuration newConfig) {
        Log.i(TAG, "onMultiWindowModeChanged: " + isInMultiWindowMode);
        super.onMultiWindowModeChanged(isInMultiWindowMode, newConfig);
        // The activity may be resized in multi-window mode. Forward the
        // configuration so the runtime can re-layout.
        reportCurrentMetrics();
    }

    @Override
    public void onPictureInPictureModeChanged(boolean isInPictureInPictureMode, Configuration newConfig) {
        Log.i(TAG, "onPictureInPictureModeChanged: " + isInPictureInPictureMode);
        super.onPictureInPictureModeChanged(isInPictureInPictureMode, newConfig);
    }

    private void reportCurrentMetrics() {
        WindowManager wm = (WindowManager) getSystemService(Context.WINDOW_SERVICE);
        if (wm == null) return;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            WindowMetrics metrics = wm.getCurrentWindowMetrics();
            Rect bounds = metrics.getBounds();
            nativeOnWindowMetricsChanged(bounds.width(), bounds.height());
        } else {
            Display display = wm.getDefaultDisplay();
            if (display != null) {
                Point size = new Point();
                display.getRealSize(size);
                nativeOnWindowMetricsChanged(size.x, size.y);
            }
        }
    }

    @Override
    public void onLowMemory() {
        Log.i(TAG, "onLowMemory called");
        super.onLowMemory();
    }

    @Override
    public void onTrimMemory(int level) {
        Log.i(TAG, "onTrimMemory called: level=" + level);
        super.onTrimMemory(level);
        nativeOnTrimMemory(level);
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        boolean granted = grantResults != null
                && grantResults.length > 0
                && grantResults[0] == PackageManager.PERMISSION_GRANTED;
        boolean permanent = false;
        if (!granted && permissions != null && permissions.length > 0) {
            permanent = !shouldShowRequestPermissionRationale(permissions[0]);
        }
        nativePermissionResult(requestCode, granted, permanent);
    }

    public void requestPermission(String permission, int requestCode) {
        Log.i(TAG, "requestPermission: " + permission + " code=" + requestCode);
        if (!hasDeclaredPermission(permission)) {
            Log.w(TAG, "requestPermission rejected for undeclared permission: " + permission);
            return;
        }
        requestPermissions(new String[] { permission }, requestCode);
    }

    public int checkPermission(String permission) {
        Log.i(TAG, "checkPermission: " + permission);
        if (!hasDeclaredPermission(permission)) {
            return PackageManager.PERMISSION_DENIED;
        }
        return checkSelfPermission(permission);
    }

    public boolean hasDeclaredPermission(String permission) {
        try {
            PackageInfo info = getPackageManager().getPackageInfo(getPackageName(), PackageManager.GET_PERMISSIONS);
            if (info == null || info.requestedPermissions == null) {
                return false;
            }
            for (String declared : info.requestedPermissions) {
                if (permission != null && permission.equals(declared)) {
                    return true;
                }
            }
        } catch (PackageManager.NameNotFoundException e) {
            Log.e(TAG, "Unable to inspect declared permissions", e);
        }
        return false;
    }

    public void showSoftKeyboard() {
        Log.i(TAG, "showSoftKeyboard called");
        runOnUiThread(() -> {
            if (imeView == null) {
                return;
            }
            imeView.requestFocus();
            InputMethodManager imm = (InputMethodManager) getSystemService(Context.INPUT_METHOD_SERVICE);
            if (imm != null) {
                imm.showSoftInput(imeView, InputMethodManager.SHOW_IMPLICIT);
            }
        });
    }

    public void hideSoftKeyboard() {
        Log.i(TAG, "hideSoftKeyboard called");
        runOnUiThread(() -> {
            if (imeView == null) {
                return;
            }
            InputMethodManager imm = (InputMethodManager) getSystemService(Context.INPUT_METHOD_SERVICE);
            if (imm != null) {
                imm.hideSoftInputFromWindow(imeView.getWindowToken(), 0);
            }
            imeView.clearFocus();
        });
    }

    @Override
    public void onAudioFocusChange(int focusChange) {
        Log.i(TAG, "onAudioFocusChange: " + focusChange);
        nativeOnAudioFocusChange(focusChange);
    }

    public void requestAudioFocus() {
        Log.i(TAG, "requestAudioFocus");
        if (audioManager == null) return;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O && audioFocusRequest != null) {
            audioManager.requestAudioFocus(audioFocusRequest);
        } else {
            audioManager.requestAudioFocus(this, AudioManager.STREAM_MUSIC, AudioManager.AUDIOFOCUS_GAIN);
        }
    }

    public void abandonAudioFocus() {
        Log.i(TAG, "abandonAudioFocus");
        if (audioManager == null) return;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O && audioFocusRequest != null) {
            audioManager.abandonAudioFocusRequest(audioFocusRequest);
        } else {
            audioManager.abandonAudioFocus(this);
        }
    }

    public void setExtractionProgress(final float progress) {
        runOnUiThread(() -> {
            if (splashText == null) return;
            splashText.setVisibility(android.view.View.VISIBLE);
            int pct = Math.round(progress * 100);
            splashText.setText("Extracting assets... " + pct + "%");
            if (progress >= 1.0f) {
                splashText.setVisibility(android.view.View.GONE);
            }
        });
    }

    private native void nativeOnTrimMemory(int level);

    private native void nativeImeCompose(String text, int cursorPos);
    private native void nativeImeCommit(String text);
    private native void nativeImeKeyEvent(int keyCode, int action, int metaState);
    private native void nativeOnWindowInsets(int top, int bottom, int left, int right,
                                              int cutoutLeft, int cutoutTop,
                                              int cutoutRight, int cutoutBottom);
    private native void nativeOnAudioFocusChange(int focusChange);
    private native void nativeOnBackInvoked();
    private native void nativeOnWindowMetricsChanged(int width, int height);
    private native byte[] nativeGetSavedState();
    private native void nativeSetSavedState(byte[] state);
    private native void nativePermissionResult(int requestCode, boolean granted, boolean permanent);

    private static final class ImeInputView extends View {
        private final LurpicNativeActivity activity;

        ImeInputView(LurpicNativeActivity activity) {
            super(activity);
            this.activity = activity;
            setFocusable(true);
            setFocusableInTouchMode(true);
            setVisibility(INVISIBLE);
        }

        @Override
        public boolean onCheckIsTextEditor() {
            return true;
        }

        @Override
        public InputConnection onCreateInputConnection(EditorInfo outAttrs) {
            outAttrs.inputType = InputType.TYPE_CLASS_TEXT
                    | InputType.TYPE_TEXT_FLAG_MULTI_LINE
                    | InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS;
            outAttrs.imeOptions = EditorInfo.IME_FLAG_NO_FULLSCREEN | EditorInfo.IME_FLAG_NO_EXTRACT_UI;
            return new ImeInputConnection(this, activity);
        }
    }

    private static final class ImeInputConnection extends BaseInputConnection {
        private final LurpicNativeActivity activity;

        ImeInputConnection(View targetView, LurpicNativeActivity activity) {
            super(targetView, true);
            this.activity = activity;
        }

        @Override
        public boolean setComposingText(CharSequence text, int newCursorPosition) {
            activity.nativeImeCompose(text != null ? text.toString() : "", newCursorPosition);
            return true;
        }

        @Override
        public boolean commitText(CharSequence text, int newCursorPosition) {
            activity.nativeImeCommit(text != null ? text.toString() : "");
            return true;
        }

        @Override
        public boolean finishComposingText() {
            activity.nativeImeCompose("", 0);
            return true;
        }

        @Override
        public boolean deleteSurroundingText(int beforeLength, int afterLength) {
            activity.nativeImeKeyEvent(KeyEvent.KEYCODE_DEL, KeyEvent.ACTION_DOWN, 0);
            activity.nativeImeKeyEvent(KeyEvent.KEYCODE_DEL, KeyEvent.ACTION_UP, 0);
            return true;
        }

        @Override
        public boolean sendKeyEvent(KeyEvent event) {
            if (event == null) {
                return false;
            }
            activity.nativeImeKeyEvent(event.getKeyCode(), event.getAction(), event.getMetaState());
            return true;
        }

        @Override
        public boolean performEditorAction(int actionCode) {
            int keyCode;
            int metaState = 0;
            switch (actionCode) {
                case EditorInfo.IME_ACTION_DONE:
                case EditorInfo.IME_ACTION_GO:
                case EditorInfo.IME_ACTION_SEND:
                case EditorInfo.IME_ACTION_SEARCH:
                    keyCode = KeyEvent.KEYCODE_ENTER;
                    break;
                case EditorInfo.IME_ACTION_NEXT:
                    keyCode = KeyEvent.KEYCODE_TAB;
                    break;
                case EditorInfo.IME_ACTION_PREVIOUS:
                    keyCode = KeyEvent.KEYCODE_TAB;
                    metaState = KeyEvent.META_SHIFT_ON;
                    break;
                default:
                    keyCode = KeyEvent.KEYCODE_ENTER;
                    break;
            }
            activity.nativeImeKeyEvent(keyCode, KeyEvent.ACTION_DOWN, metaState);
            activity.nativeImeKeyEvent(keyCode, KeyEvent.ACTION_UP, metaState);
            return true;
        }
    }
}
