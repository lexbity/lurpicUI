// Package common contains small reusable helpers shared by platform backends.
//
// Only move code here when it has a concrete reuse path across multiple
// platform backends. Platform-specific glue, syscalls, display-server details,
// and host-specific lifecycle code stay in the backend's own internal packages.
package common
