package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/drive/schema/gen"
)

// driveStreamVerbs are the stream/administration verbs: watch (live
// subscription consumption with acks and keepalives), sub-state, sub-cancel,
// op-cancel, own-state, session-state, session-list, session-revoke. Every
// call is a generated typed client call; the watch loop is the one place
// the CLI speaks raw notifications (client notifications carry no ID).
func driveStreamVerbs(verb string, fs *flag.FlagSet, conn *driveJSON) int {
	c := &gen.Client{Transport: conn}
	switch verb {
	case "watch":
		return cmdDriveWatch(conn, c, fs)
	case "sub-state":
		streamID := fs.Lookup("stream-id").Value.String()
		if streamID == "" {
			fmt.Fprintln(os.Stderr, "drive: sub-state requires --stream-id")
			return 1
		}
		payload, err := c.SubState(map[string]string{"stream_id": streamID})
		return printPayload(err, payload)
	case "sub-cancel":
		streamID := fs.Lookup("stream-id").Value.String()
		if streamID == "" {
			fmt.Fprintln(os.Stderr, "drive: sub-cancel requires --stream-id")
			return 1
		}
		payload, err := c.SubCancel(map[string]string{"stream_id": streamID})
		return printPayload(err, payload)
	case "op-cancel":
		operationID := fs.Lookup("operation-id").Value.String()
		if operationID == "" {
			fmt.Fprintln(os.Stderr, "drive: op-cancel requires --operation-id")
			return 1
		}
		payload, err := c.OpCancel(map[string]string{"operation_id": operationID})
		return printPayload(err, payload)
	case "own-state":
		payload, err := c.OwnState(map[string]any{})
		return printPayload(err, payload)
	case "session-state":
		payload, err := c.SessionState(map[string]any{})
		return printPayload(err, payload)
	case "session-list":
		params := map[string]any{}
		if v := fs.Lookup("cursor").Value.String(); v != "" {
			params["cursor"] = v
		}
		if v := fs.Lookup("limit").Value.String(); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 32 {
				fmt.Fprintln(os.Stderr, "drive: session-list --limit must be 1..32")
				return 1
			}
			params["limit"] = n
		}
		payload, err := c.SessionList(params)
		return printPayload(err, payload)
	case "session-revoke":
		sessionID := fs.Lookup("session-id").Value.String()
		reason := fs.Lookup("reason").Value.String()
		if sessionID == "" || reason == "" {
			fmt.Fprintln(os.Stderr, "drive: session-revoke requires --session-id and --reason (operator, policy, instance.shutdown)")
			return 1
		}
		payload, err := c.SessionRevoke(map[string]string{"session_id": sessionID, "reason": reason})
		return printPayload(err, payload)
	}
	return 2
}

func printPayload(err error, payload json.RawMessage) int {
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive: %v\n", err)
		return 1
	}
	fmt.Println(string(payload))
	return 0
}

// cmdDriveWatch implements `lurpic drive watch`: create a subscription,
// print every delivered record, acknowledge each cursor, send keepalives,
// and exit after --duration (default 10s) or on a terminal stream
// notification. It is the live observation half of driving a running app.
func cmdDriveWatch(conn *driveJSON, client *gen.Client, fs *flag.FlagSet) int {
	params := map[string]any{}
	filter := map[string]any{}
	if v := fs.Lookup("classes").Value.String(); v != "" {
		filter["classes"] = strings.Split(v, ",")
	}
	if v := fs.Lookup("names").Value.String(); v != "" {
		filter["names"] = strings.Split(v, ",")
	}
	if len(filter) > 0 {
		params["filter"] = filter
	}
	if v := fs.Lookup("after").Value.String(); v != "" {
		params["after"] = v
	}
	if fs.Lookup("snapshot").Value.String() == "true" {
		params["snapshot"] = true
	}
	if v := fs.Lookup("credit").Value.String(); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 8 {
			fmt.Fprintln(os.Stderr, "drive: watch --credit must be 1..8")
			return 1
		}
		params["credit"] = n
	}
	haveAfter := params["after"] != nil
	_, haveSnapshot := params["snapshot"]
	if haveAfter == haveSnapshot {
		fmt.Fprintln(os.Stderr, "drive: watch requires exactly one of --after or --snapshot")
		return 1
	}
	duration := 10 * time.Second
	if v := fs.Lookup("duration").Value.String(); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil || parsed <= 0 {
			fmt.Fprintln(os.Stderr, "drive: watch --duration must be a positive Go duration")
			return 1
		}
		duration = parsed
	}
	payload, err := client.SubWatch(params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive: %v\n", err)
		return 1
	}
	var created struct {
		StreamID string `json:"stream_id"`
		Position string `json:"position"`
		Credit   int    `json:"credit"`
	}
	if err := json.Unmarshal(payload, &created); err != nil || created.StreamID == "" {
		fmt.Fprintf(os.Stderr, "drive: watch result: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "watching stream %s from %s (credit %d) for %s\n", created.StreamID, created.Position, created.Credit, duration)

	deadline := time.Now().Add(duration)
	keepalive := time.NewTicker(2 * time.Second)
	defer keepalive.Stop()
	for {
		if time.Now().After(deadline) {
			// Explicit unsubscribe before exit: the stream does not outlive
			// the driver's interest.
			_, _ = client.SubCancel(map[string]string{"stream_id": created.StreamID})
			return 0
		}
		select {
		case <-keepalive.C:
			if err := conn.sendClientNotification("sys.keepalive", nil); err != nil {
				fmt.Fprintf(os.Stderr, "drive: keepalive: %v\n", err)
				return 1
			}
		default:
		}
		notification, err := conn.nextNotification()
		if err != nil {
			fmt.Fprintf(os.Stderr, "drive: read: %v\n", err)
			return 1
		}
		switch notification.Method {
		case "sub.data":
			var data struct {
				StreamID string            `json:"stream_id"`
				Cursor   string            `json:"cursor"`
				Cut      string            `json:"cut"`
				Frame    string            `json:"frame"`
				Events   []json.RawMessage `json:"events"`
			}
			if err := json.Unmarshal(notification.Params, &data); err != nil {
				fmt.Fprintf(os.Stderr, "drive: sub.data payload: %v\n", err)
				return 1
			}
			for _, ev := range data.Events {
				fmt.Printf("cut=%s frame=%s cursor=%s %s\n", data.Cut, data.Frame, data.Cursor, string(ev))
			}
			if err := conn.sendClientNotification("sub.ack", map[string]string{
				"stream_id": data.StreamID, "position": data.Cursor,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "drive: ack: %v\n", err)
				return 1
			}
		case "sub.expired":
			fmt.Printf("stream expired: %s\n", string(notification.Params))
			return 0
		case "own.preempted":
			fmt.Printf("preempted: %s\n", string(notification.Params))
		case "sys.shutdown":
			fmt.Println("server shutting down")
			return 0
		default:
			// op.completed and other notifications are printed as-is.
			fmt.Printf("%s: %s\n", notification.Method, string(notification.Params))
		}
	}
}
