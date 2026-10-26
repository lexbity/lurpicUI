package vulkan

import "fmt"

// ResourceUsage reports memory tracked through the Vulkan GPU allocator.
// Driver-private memory, swapchain allocations, and OS surface storage are excluded.
type ResourceUsage struct {
	LiveBytes        [6]uint64
	ReservedBytes    [6]uint64
	RetiringBytes    [6]uint64
	LiveCounts       [6]uint64
	DeviceGeneration uint64
	BudgetBytes      uint64
}

// TotalLive returns currently usable renderer-owned allocation bytes.
func (s ResourceUsage) TotalLive() uint64 { return sumResourceBytes(s.LiveBytes) }

// TotalReserved returns bytes admitted for allocation but not yet published.
func (s ResourceUsage) TotalReserved() uint64 { return sumResourceBytes(s.ReservedBytes) }

// TotalRetiring returns bytes held until their release boundary completes.
func (s ResourceUsage) TotalRetiring() uint64 { return sumResourceBytes(s.RetiringBytes) }

// TotalManaged returns the saturated total across live, reserved and retiring.
func (s ResourceUsage) TotalManaged() uint64 {
	total := s.TotalLive()
	if ^uint64(0)-total < s.TotalReserved() {
		return ^uint64(0)
	}
	total += s.TotalReserved()
	if ^uint64(0)-total < s.TotalRetiring() {
		return ^uint64(0)
	}
	return total + s.TotalRetiring()
}

func sumResourceBytes(values [6]uint64) uint64 {
	var total uint64
	for _, value := range values {
		if ^uint64(0)-total < value {
			return ^uint64(0)
		}
		total += value
	}
	return total
}

func readbackBufferSize(width, height int) (int, error) {
	if width <= 0 || height <= 0 {
		return 0, fmt.Errorf("vulkan: readback dimensions must be positive")
	}
	if uint64(width) > uint64(^uint32(0)) || uint64(height) > uint64(^uint32(0)) {
		return 0, fmt.Errorf("vulkan: readback dimensions exceed Vulkan's 32-bit extent")
	}
	maxInt := int(^uint(0) >> 1)
	if width > maxInt/4 || height > maxInt/(width*4) {
		return 0, fmt.Errorf("vulkan: readback dimensions overflow addressable storage")
	}
	return width * height * 4, nil
}
