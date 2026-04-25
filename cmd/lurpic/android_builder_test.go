package main

import "testing"

func TestAndroidBuilder_selectAndroidAPI(t *testing.T) {
	b := &androidBuilder{
		config: &Config{
			Android: AndroidConfig{TargetSDK: 33},
		},
	}
	if err := b.selectAndroidAPI(); err != nil {
		t.Fatalf("selectAndroidAPI: %v", err)
	}
}
