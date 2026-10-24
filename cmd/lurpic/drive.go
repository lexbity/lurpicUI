package main

import (
	"flag"
	"fmt"
	"os"
)

// cmdDrive is the driver CLI: connect, authenticate, execute one verb
// (ping / resolve / tree). Flags may precede the verb. Phase 8 replaces
// this file's plumbing with the generated typed client.
func cmdDrive(args []string) int {
	// The verb is one of the known names; lift it out (flag values like a
	// socket path are bare words too, so match by name) and parse the
	// remaining flags regardless of position.
	verb := ""
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if verb == "" && (a == "ping" || a == "resolve" || a == "tree") {
			verb = a
			continue
		}
		rest = append(rest, a)
	}
	if verb == "" {
		fmt.Fprintln(os.Stderr, "drive: a verb is required (ping, resolve, tree)")
		return 1
	}
	fs := driveFlagSet()
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	switch verb {
	case "ping":
		return cmdDriveRun(fs, func(fs *flag.FlagSet, conn *driveJSON) int {
			res, err := conn.call("sys.ping", nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "drive: %v\n", err)
				return 1
			}
			if !res.OK {
				fmt.Fprintf(os.Stderr, "drive: %v\n", res.Error)
				return 1
			}
			fmt.Printf("ok frame=%d\n", res.Frame)
			return 0
		})
	case "watch":
		return cmdDriveRun(fs, cmdDriveWatch)
	case "resolve":
		return cmdDriveRun(fs, cmdDriveResolve)
	case "tree":
		return cmdDriveRun(fs, cmdDriveTree)
	default:
		fmt.Fprintf(os.Stderr, "drive: unknown verb %q (ping, watch, resolve, tree)\n", verb)
		return 1
	}
}

// cmdDriveRun connects with the parsed flags and runs the verb.
func cmdDriveRun(fs *flag.FlagSet, run func(*flag.FlagSet, *driveJSON) int) int {
	socket := fs.Lookup("socket").Value.String()
	tokenFile := fs.Lookup("token-file").Value.String()
	if socket == "" || tokenFile == "" {
		fmt.Fprintln(os.Stderr, "drive: --socket and --token-file are required")
		return 1
	}
	conn, err := driveConnect(socket, tokenFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive: %v\n", err)
		return 1
	}
	defer conn.close()
	return run(fs, conn)
}
