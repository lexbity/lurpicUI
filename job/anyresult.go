package job

// AnyResult is the narrow opaque result type exposed to runtime-facing code.
//
// The generic Result type remains the typed worker-facing API in this package.
type AnyResult interface{}
