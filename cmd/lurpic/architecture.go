package main

type Architecture struct {
	ABI         string
	GOARCH      string
	GOARM       string
	NDKTriple   string
	CargoTarget string
	EmulatorABI string
}

var architectures = map[string]Architecture{
	archX8664: {
		ABI:         archX8664,
		GOARCH:      "amd64",
		GOARM:       "",
		NDKTriple:   "x86_64-linux-android",
		CargoTarget: "x86_64-linux-android",
		EmulatorABI: archX8664,
	},
	archArm64V8a: {
		ABI:         archArm64V8a,
		GOARCH:      "arm64",
		GOARM:       "",
		NDKTriple:   "aarch64-linux-android",
		CargoTarget: "aarch64-linux-android",
		EmulatorABI: archArm64V8a,
	},
	"armeabi-v7a": {
		ABI:         "armeabi-v7a",
		GOARCH:      "arm",
		GOARM:       "7",
		NDKTriple:   "armv7a-linux-androideabi",
		CargoTarget: "armv7-linux-androideabi",
		EmulatorABI: "",
	},
}

func ArchitectureByABI(abi string) (Architecture, bool) {
	arch, ok := architectures[abi]
	return arch, ok
}

func DefaultEmulatorArchitecture() Architecture {
	return architectures[archX8664]
}

func ReleaseArchitectures() []Architecture {
	result := make([]Architecture, 0, len(architectures))
	for _, arch := range architectures {
		result = append(result, arch)
	}
	return result
}
