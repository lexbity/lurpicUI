package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeburg.org/lexbit/lurpicui/drive/schema/gen"
)

// cmdDrive is the LDP driver CLI: connect, authenticate, execute one verb.
// Every verb is a generated typed call (drive/schema/gen.Client) — the
// hand-rolled per-verb plumbing was deleted in Phase 8; the client is
// regenerated from the registry, never extended by hand.
func cmdDrive(args []string) int {
	// The verb is one of the known names; lift it out (flag values like a
	// socket path are bare words too, so match by name) and parse the
	// remaining flags regardless of position.
	verb := ""
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if verb == "" && (a == "ping" || a == "watch" || a == "resolve" || a == "tree" || a == "--check") {
			verb = a
			continue
		}
		rest = append(rest, a)
	}
	if verb == "" {
		fmt.Fprintln(os.Stderr, "drive: a verb is required (ping, watch, resolve, tree, --check)")
		return 1
	}
	if verb == "--check" {
		fs2 := driveFlagSet()
		if err := fs2.Parse(rest); err != nil {
			return 1
		}
		return cmdDriveRun(fs2, func(fs *flag.FlagSet, conn *driveJSON) int {
			return cmdDriveCheck(conn, &gen.Client{Transport: conn})
		})
	}
	fs := driveFlagSet()
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	socket := fs.Lookup("socket").Value.String()
	tokenFile := fs.Lookup("token-file").Value.String()
	if socket == "" || tokenFile == "" {
		fmt.Fprintln(os.Stderr, "drive: --socket and --token-file are required")
		return 1
	}

	switch verb {
	case "ping":
		return cmdDriveRun(fs, func(fs *flag.FlagSet, conn *driveJSON) int {
			client := &gen.Client{Transport: conn}
			if _, err := client.SysPing(); err != nil {
				fmt.Fprintf(os.Stderr, "drive: %v\n", err)
				return 1
			}
			fmt.Printf("ok frame=%d\n", conn.lastFrame)
			return 0
		})
	case "watch":
		return cmdDriveRun(fs, cmdDriveWatch)
	case "resolve":
		return cmdDriveRun(fs, func(fs *flag.FlagSet, conn *driveJSON) int {
			client := &gen.Client{Transport: conn}
			payload, err := client.LocateResolve(locatorFromFlags(fs))
			if err != nil {
				fmt.Fprintf(os.Stderr, "drive: %v\n", err)
				return 1
			}
			fmt.Println(string(payload))
			return 0
		})
	case "tree":
		return cmdDriveRun(fs, func(fs *flag.FlagSet, conn *driveJSON) int {
			client := &gen.Client{Transport: conn}
			params := map[string]any{"from": locatorFromFlags(fs)}
			if v := fs.Lookup("depth"); v != nil {
				var depth int
				if _, err := fmt.Sscanf(v.Value.String(), "%d", &depth); err == nil && depth > 0 {
					params["depth"] = depth
				}
			}
			payload, err := client.TreeDump(params)
			if err != nil {
				fmt.Fprintf(os.Stderr, "drive: %v\n", err)
				return 1
			}
			fmt.Println(string(payload))
			return 0
		})
	default:
		fmt.Fprintf(os.Stderr, "drive: unknown verb %q (ping, watch, resolve, tree, --check)\n", verb)
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

// locatorFromFlags builds a locator map from the locator flags.
func locatorFromFlags(fs *flag.FlagSet) map[string]string {
	locator := map[string]string{}
	for _, flagName := range []string{"testid", "role", "name", "key", "text", "kind"} {
		if v := fs.Lookup(flagName); v != nil && v.Value.String() != "" {
			locator[flagName] = v.Value.String()
		}
	}
	return locator
}

// cmdDriveCheck implements `lurpic drive --check` (G8.4): hello + registry
// digest comparison — the client's method list digest against the server's
// sys.info methods. Prints "compatible v<major> digest=<sha>" and exits 0 on
// a match; any skew exits non-zero with both digests.
func cmdDriveCheck(conn *driveJSON, client *gen.Client) int {
	// The handshake already happened in driveConnect (a second sys.hello
	// post-auth would be refused); the check validates the negotiated major
	// via sys.info's payload instead.
	info, err := client.SysInfo(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive: sys.info: %v\n", err)
		return 1
	}
	var payload struct {
		V       int      `json:"v"`
		Methods []string `json:"methods"`
	}
	if err := json.Unmarshal(info, &payload); err != nil {
		fmt.Fprintf(os.Stderr, "drive: sys.info payload: %v\n", err)
		return 1
	}

	clientDigest := methodDigest(gen.Methods())
	serverDigest := methodDigest(payload.Methods)
	if clientDigest != serverDigest {
		fmt.Fprintf(os.Stderr, "drive: INCOMPATIBLE — client digest=%s server digest=%s (regenerate: lurpiclint drive-schema --out drive/schema)\n",
			clientDigest, serverDigest)
		return 1
	}
	fmt.Printf("compatible v%d digest=%s\n", payload.V, clientDigest)
	return 0
}

// methodDigest sha256s the sorted method list (the registry digest the
// --check comparison keys on).
func methodDigest(methods []string) string {
	sorted := append([]string(nil), methods...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return hex.EncodeToString(sum[:8])
}
