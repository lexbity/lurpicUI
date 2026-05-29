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
	"x86_64": {
		ABI:         "x86_64",
		GOARCH:      "amd64",
		GOARM:       "",
		NDKTriple:   "x86_64-linux-android",
		CargoTarget: "x86_64-linux-android",
		EmulatorABI: "x86_64",
	},
	"arm64-v8a": {
		ABI:         "arm64-v8a",
		GOARCH:      "arm64",
		GOARM:       "",
		NDKTriple:   "aarch64-linux-android",
		CargoTarget: "aarch64-linux-android",
		EmulatorABI: "arm64-v8a",
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
	return architectures["x86_64"]
}

func ReleaseArchitectures() []Architecture {
	result := make([]Architecture, 0, len(architectures))
	for _, arch := range architectures {
		result = append(result, arch)
	}
	return result
}
