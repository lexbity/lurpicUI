package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// cmdDrive is the Phase 1 driver CLI: connect, authenticate, execute one
// verb. Phase 8 replaces this file's plumbing with the generated typed
// client.
func cmdDrive(args []string) int {
	fs := flag.NewFlagSet("drive", flag.ContinueOnError)
	socket := fs.String("socket", "", "drive socket path")
	tokenFile := fs.String("token-file", "", "drive token file path (<socket>.token)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	verb := fs.Arg(0)
	if verb != "ping" {
		fmt.Fprintf(os.Stderr, "drive: unknown verb %q (Phase 1 supports: ping)\n", verb)
		return 1
	}
	if *socket == "" || *tokenFile == "" {
		fmt.Fprintln(os.Stderr, "drive: --socket and --token-file are required")
		return 1
	}
	if err := drivePing(*socket, *tokenFile); err != nil {
		fmt.Fprintf(os.Stderr, "drive: %v\n", err)
		return 1
	}
	return 0
}

func drivePing(socket, tokenFile string) error {
	token, err := os.ReadFile(tokenFile) //nolint:gosec // operator-supplied path by design
	if err != nil {
		return fmt.Errorf("read token: %w", err)
	}
	nc, err := net.Dial("unix", socket)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = nc.Close() }()
	rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))

	send := func(req proto.Request) error {
		out, err := json.Marshal(req)
		if err != nil {
			return err
		}
		if _, err := rw.Write(append(out, '\n')); err != nil {
			return err
		}
		return rw.Flush()
	}
	recv := func() (proto.Response, error) {
		line, err := rw.ReadString('\n')
		if err != nil {
			return proto.Response{}, err
		}
		var res proto.Response
		if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &res); err != nil {
			return proto.Response{}, fmt.Errorf("decode %q: %w", line, err)
		}
		return res, nil
	}

	if err := send(proto.Request{V: proto.ProtocolMajor, ID: 0, Method: "sys.hello",
		Params: json.RawMessage(`{"token":"` + strings.TrimSpace(string(token)) + `"}`)}); err != nil {
		return fmt.Errorf("hello: %w", err)
	}
	hello, err := recv()
	if err != nil {
		return fmt.Errorf("hello: %w", err)
	}
	if !hello.OK {
		return fmt.Errorf("hello rejected: %s", hello.Error.Error())
	}
	if err := send(proto.Request{V: proto.ProtocolMajor, ID: 1, Method: "sys.ping"}); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	res, err := recv()
	if err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	if !res.OK {
		return fmt.Errorf("ping rejected: %s", res.Error.Error())
	}
	fmt.Printf("ok frame=%d\n", res.Frame)
	return nil
}
