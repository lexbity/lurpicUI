package main

import "testing"

func TestValidateBenchmarkOutputRequiresMeasurement(t *testing.T) {
	const benchmark = "BenchmarkVulkanSubmit_AcceptanceCorpus"
	t.Run("measured", func(t *testing.T) {
		output := []byte("BenchmarkVulkanSubmit_AcceptanceCorpus-16 260 4636494 ns/op 0 B/op 0 allocs/op\n")
		if err := validateBenchmarkOutput(output, benchmark); err != nil {
			t.Fatalf("valid benchmark output rejected: %v", err)
		}
	})
	t.Run("skipped", func(t *testing.T) {
		output := []byte("--- SKIP: BenchmarkVulkanSubmit_AcceptanceCorpus\n")
		if err := validateBenchmarkOutput(output, benchmark); err == nil {
			t.Fatal("required benchmark skip was accepted")
		}
	})
	t.Run("missing", func(t *testing.T) {
		output := []byte("PASS\nok\tcodeburg.org/lexbit/lurpicui/render/vulkan\t0.1s\n")
		if err := validateBenchmarkOutput(output, benchmark); err == nil {
			t.Fatal("missing benchmark result was accepted")
		}
	})
}
