//go:build !android

package android

import (
	"testing"
)

func TestPermissionConstants(t *testing.T) {
	if PermissionCamera != "android.permission.CAMERA" {
		t.Errorf("unexpected PermissionCamera: %s", PermissionCamera)
	}
	if PermissionPostNotifications != "android.permission.POST_NOTIFICATIONS" {
		t.Errorf("unexpected PermissionPostNotifications: %s", PermissionPostNotifications)
	}
	if PermissionLocationFine != "android.permission.ACCESS_FINE_LOCATION" {
		t.Errorf("unexpected PermissionLocationFine: %s", PermissionLocationFine)
	}
	if PermissionLocationCoarse != "android.permission.ACCESS_COARSE_LOCATION" {
		t.Errorf("unexpected PermissionLocationCoarse: %s", PermissionLocationCoarse)
	}
}

func TestPermissionResultConstants(t *testing.T) {
	if PermissionGranted != 0 {
		t.Errorf("expected PermissionGranted=0, got %d", PermissionGranted)
	}
	if PermissionDenied != 1 {
		t.Errorf("expected PermissionDenied=1, got %d", PermissionDenied)
	}
	if PermissionDeniedPermanent != 2 {
		t.Errorf("expected PermissionDeniedPermanent=2, got %d", PermissionDeniedPermanent)
	}
}

func TestRequestPermission_nonAndroidReturnsError(t *testing.T) {
	_, err := RequestPermission(PermissionCamera)
	if err == nil {
		t.Fatal("expected error on non-Android platform")
	}
}

func TestCheckPermission_nonAndroidReturnsDenied(t *testing.T) {
	result, err := CheckPermission(PermissionCamera)
	if err == nil {
		t.Fatal("expected error on non-Android platform")
	}
	if result != PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", result)
	}
}

func TestOnPermissionDenied_nonAndroidNoPanic(t *testing.T) {
	OnPermissionDenied(func(permission Permission, permanent bool) {
		t.Error("should not be called on non-Android")
	})
}

func TestRequestLocationWithPrecision_nonAndroidReturnsDenied(t *testing.T) {
	ch := RequestLocationWithPrecision(true)
	result := <-ch
	if result != PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", result)
	}
}
