package cook

// Platform identifies the cook target platform.
type Platform string

const (
	PlatformLinux   Platform = "linux"
	PlatformAndroid Platform = "android"
	PlatformDarwin  Platform = "darwin"
	PlatformWindows Platform = "windows"
)

// Compiler transforms one source file into one or more cooked LOD binaries.
// Implementations must be safe for concurrent use.
type Compiler interface {
	// Extensions returns file extensions this compiler handles, e.g. []string{".svg"}.
	Extensions() []string

	// Compile reads src and returns cooked LOD levels.
	Compile(src []byte, target Platform) ([]CompiledLOD, error)
}

// CompiledLOD is the output of one LOD compilation.
type CompiledLOD struct {
	Level int
	Data  []byte
}
