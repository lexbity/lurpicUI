package job

// AnyJob is the narrow opaque job type exposed to runtime-facing services.
//
// The generic Job type remains the worker-facing API in this package.
type AnyJob interface{}
