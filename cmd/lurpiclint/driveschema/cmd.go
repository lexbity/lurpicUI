package driveschema

import (
	"flag"
	"fmt"
	"os"
)

// Run implements `lurpiclint drive-schema --out <dir>`: emits the JSON
// Schema and the typed Go client from the protocol registry (QP-15).
func Run(args []string) int {
	fs := flag.NewFlagSet("drive-schema", flag.ContinueOnError)
	out := fs.String("out", "drive/schema", "output directory (schema.json + gen/client.go)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if err := Generate(*out); err != nil {
		fmt.Fprintf(os.Stderr, "drive-schema: %v\n", err)
		return 1
	}
	fmt.Printf("drive-schema: wrote %s/schema.json and %s/gen/client.go\n", *out, *out)
	return 0
}
