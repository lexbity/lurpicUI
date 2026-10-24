package proto

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	line := `{"v":1,"id":42,"method":"frame.run","params":{"steps":[]}}`
	var req Request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.V != 1 || req.ID != 42 || req.Method != "frame.run" {
		t.Fatalf("decoded = %+v", req)
	}
	var params struct {
		Steps []json.RawMessage `json:"steps"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatalf("params: %v", err)
	}
	if len(params.Steps) != 0 {
		t.Fatalf("steps = %d, want 0", len(params.Steps))
	}
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var again Request
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if again.V != req.V || again.ID != req.ID || again.Method != req.Method ||
		!bytes.Equal(again.Params, req.Params) {
		t.Fatalf("round trip changed request: %+v vs %+v", again, req)
	}
}

func TestResponseRoundTrip(t *testing.T) {
	res := Response{
		ID:     7,
		OK:     true,
		Result: json.RawMessage(`{"pong":true}`),
		Frame:  1043,
		Cached: true,
	}
	out, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Response
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.ID != 7 || !back.OK || back.Frame != 1043 || !back.Cached {
		t.Fatalf("round trip = %+v", back)
	}
	if !bytes.Equal(back.Result, res.Result) {
		t.Fatalf("result = %s, want %s", back.Result, res.Result)
	}
}

func TestResponseErrorShape(t *testing.T) {
	res := Response{
		ID:    9,
		OK:    false,
		Error: &Error{Code: ErrGrant, Detail: "input"},
		Frame: 12,
	}
	out, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errObj, ok := raw["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error object in %s", out)
	}
	if errObj["code"] != string(ErrGrant) {
		t.Fatalf("code = %v", errObj["code"])
	}
	if raw["ok"] != false {
		t.Fatalf("ok = %v", raw["ok"])
	}
	if _, cached := raw["cached"]; cached {
		t.Fatalf("cached present on non-retry response: %s", out)
	}
}

func TestDecodeMalformedLines(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"empty", ""},
		{"not json", "hello"},
		{"truncated", `{"v":1,"id":`},
		{"array", `[1,2,3]`},
		{"wrong id type", `{"v":1,"id":"x","method":"sys.ping"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req Request
			if err := json.Unmarshal([]byte(tc.line), &req); err == nil {
				t.Fatalf("expected error for %q", tc.line)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	e := &Error{Code: ErrAuth}
	if !strings.Contains(e.Error(), string(ErrAuth)) {
		t.Fatalf("Error() = %q", e.Error())
	}
	var nilErr *Error
	if nilErr.Error() != "<nil>" {
		t.Fatalf("nil Error() = %q", nilErr.Error())
	}
	var target *Error
	if !errors.As(error(&Error{Code: ErrGrant}), &target) || target.Code != ErrGrant {
		t.Fatal("errors.As should surface *Error")
	}
}

func FuzzEnvelope(f *testing.F) {
	seeds := []string{
		`{"v":1,"id":1,"method":"sys.ping"}`,
		`{"v":1,"id":2,"method":"sys.hello","params":{"token":"abc"}}`,
		`{"id":42,"ok":true,"result":{"pong":true},"frame":1043,"cached":true}`,
		`{"id":9,"ok":false,"error":{"code":"grant.denied","detail":"input"},"frame":3}`,
		`{"method":"own.preempted","params":{"by":"human"}}`,
		`{"v":2,"id":0,"method":"sys.ping","params":null}`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, line string) {
		if len(line) > MaxLineBytes {
			t.Skip("over the wire bound; rejected before decode")
		}
		var req Request
		_ = json.Unmarshal([]byte(line), &req)
		var res Response
		_ = json.Unmarshal([]byte(line), &res)
		var n Notification
		_ = json.Unmarshal([]byte(line), &n)
		var e Error
		_ = json.Unmarshal([]byte(line), &e)
		// Decoding arbitrary input must never panic; any error is fine.
	})
}
