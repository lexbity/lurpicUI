package org.lurpicui.bridge;

import android.app.NativeActivity;
import android.content.Context;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.os.Build;
import android.os.Bundle;
import android.util.Log;
import android.text.InputType;
import android.view.KeyEvent;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.view.inputmethod.BaseInputConnection;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputConnection;
import android.view.inputmethod.InputMethodManager;
import android.widget.FrameLayout;

/**
 * LurpicNativeActivity extends NativeActivity to provide the entry point
 * for lurpicUI applications on Android.
 */
public class LurpicNativeActivity extends NativeActivity {
    private static final String TAG = "LurpicNativeActivity";

    private FrameLayout imeRoot;
    private ImeInputView imeView;

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
                return v.onApplyWindowInsets(insets);
            }
        );

        imeView = new ImeInputView(this);
        imeRoot = new FrameLayout(this);
        imeRoot.setFocusable(false);
        imeRoot.setFocusableInTouchMode(false);
        imeRoot.addView(imeView, new FrameLayout.LayoutParams(1, 1));
        setContentView(imeRoot);
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
    public void onLowMemory() {
        Log.i(TAG, "onLowMemory called");
        super.onLowMemory();
    }

    @Override
    public void onTrimMemory(int level) {
        Log.i(TAG, "onTrimMemory called: level=" + level);
        super.onTrimMemory(level);
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

    private native void nativeImeCompose(String text, int cursorPos);
    private native void nativeImeCommit(String text);
    private native void nativeImeKeyEvent(int keyCode, int action, int metaState);
    private native void nativeOnWindowInsets(int top, int bottom, int left, int right,
                                              int cutoutLeft, int cutoutTop,
                                              int cutoutRight, int cutoutBottom);
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
