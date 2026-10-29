package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/drive/schema/gen"
)

// cmdDrive is the LDP driver CLI: connect, authenticate, execute one verb.
// The major-2 surface is hello + ping (+ --check, the schema-digest
// compatibility gate); observation and actuation verbs return with their
// native v2 slices. Every verb is a generated typed call
// (drive/schema/gen.Client) — the client is regenerated from the registry,
// never extended by hand.
func cmdDrive(args []string) int {
	verb := ""
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if verb == "" && (a == "ping" || a == "--check" || a == "commands" || a == "execute" || a == "op-status" || a == "op-cancel" ||
			a == "watch" || a == "sub-state" || a == "sub-cancel" || a == "own-state" ||
			a == "session-state" || a == "session-list" || a == "session-revoke") {
			verb = a
			continue
		}
		rest = append(rest, a)
	}
	if verb == "" {
		fmt.Fprintln(os.Stderr, "drive: a verb is required (ping, --check, commands, execute, op-status, op-cancel, watch, sub-state, sub-cancel, own-state, session-state, session-list, session-revoke)")
		return 1
	}
	fs := driveFlagSet()
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	socket := fs.Lookup("socket").Value.String()
	tokenFile := fs.Lookup("token-file").Value.String()
	sessionFile := fs.Lookup("session-file").Value.String()
	if socket == "" || tokenFile == "" {
		fmt.Fprintln(os.Stderr, "drive: --socket and --token-file are required")
		return 1
	}
	conn, err := driveConnect(socket, tokenFile, sessionFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive: %v\n", err)
		return 1
	}
	defer conn.close()

	if handled := driveStreamVerbs(verb, fs, conn); handled != 2 {
		return handled
	}

	switch verb {
	case "ping":
		client := &gen.Client{Transport: conn}
		payload, err := client.SysPing()
		if err != nil {
			fmt.Fprintf(os.Stderr, "drive: %v\n", err)
			return 1
		}
		var pong struct {
			Cut   string `json:"cut"`
			Frame string `json:"frame"`
		}
		_ = json.Unmarshal(payload, &pong)
		fmt.Printf("ok cut=%s frame=%s\n", pong.Cut, pong.Frame)
		return 0
	case "--check":
		return cmdDriveCheck(conn)
	case "commands":
		payload, err := (&gen.Client{Transport: conn}).CmdList(map[string]any{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "drive: %v\n", err)
			return 1
		}
		fmt.Println(string(payload))
		return 0
	case "execute":
		name := fs.Lookup("name").Value.String()
		version, _ := strconv.ParseUint(fs.Lookup("version").Value.String(), 10, 64)
		if name == "" || version == 0 {
			fmt.Fprintln(os.Stderr, "drive: execute requires --name and positive --version")
			return 1
		}
		payload, err := (&gen.Client{Transport: conn}).CmdExecute(map[string]any{"name": name, "version": version})
		if err != nil {
			fmt.Fprintf(os.Stderr, "drive: %v\n", err)
			return 1
		}
		fmt.Println(string(payload))
		return 0
	case "op-status":
		operationID := fs.Lookup("operation-id").Value.String()
		if operationID == "" {
			fmt.Fprintln(os.Stderr, "drive: op-status requires --operation-id")
			return 1
		}
		payload, err := (&gen.Client{Transport: conn}).OpStatus(map[string]string{"operation_id": operationID})
		if err != nil {
			fmt.Fprintf(os.Stderr, "drive: %v\n", err)
			return 1
		}
		fmt.Println(string(payload))
		return 0
	default:
		fmt.Fprintf(os.Stderr, "drive: unknown verb %q (ping, --check, commands, execute, op-status)\n", verb)
		return 1
	}
}

// cmdDriveCheck implements `lurpic drive --check`: the client's schema
// digest (computed from its generated method list) against the server's
// hello schema digest. Prints "compatible v<major> digest=<sha>" and exits
// 0 on a match; any skew exits non-zero with both digests.
func cmdDriveCheck(conn *driveJSON) int {
	clientDigest := methodDigest(gen.Methods())
	if clientDigest != conn.schemaDigest {
		fmt.Fprintf(os.Stderr, "drive: INCOMPATIBLE — client digest=%s server digest=%s (regenerate: lurpiclint drive-schema --out drive/schema)\n",
			clientDigest, conn.schemaDigest)
		return 1
	}
	fmt.Printf("compatible v%d digest=%s\n", proto.ProtocolMajor, clientDigest)
	return 0
}

// methodDigest sha256s the sorted method list (the registry digest the
// --check comparison keys on; the same canonical form proto.SchemaDigest
// uses, computed over the generated client's method list).
func methodDigest(methods []string) string {
	sorted := append([]string(nil), methods...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	sum := sha256.Sum256([]byte("ldp/2\x00" + strings.Join(sorted, "\x00")))
	return "sha256:" + hex.EncodeToString(sum[:])
}
