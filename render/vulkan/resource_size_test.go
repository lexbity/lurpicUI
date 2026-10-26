package vulkan

import "testing"

func TestReadbackBufferSizeValidatesBeforeAllocation(t *testing.T) {
	size, err := readbackBufferSize(64, 32)
	if err != nil {
		t.Fatal(err)
	}
	if size != 64*32*4 {
		t.Fatalf("readback size = %d, want %d", size, 64*32*4)
	}
	for _, dimensions := range [][2]int{{0, 1}, {1, 0}, {-1, 1}, {1, -1}, {int(^uint(0) >> 1), 2}} {
		if _, err := readbackBufferSize(dimensions[0], dimensions[1]); err == nil {
			t.Errorf("readback dimensions %v accepted", dimensions)
		}
	}
}
