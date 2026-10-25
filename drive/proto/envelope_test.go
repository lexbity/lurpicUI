package proto

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	line := `{"v":2,"id":"42","method":"sys.ping","params":{}}`
	req, err := DecodeRequest([]byte(line))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.V != 2 || req.ID != "42" || req.Method != "sys.ping" {
		t.Fatalf("decoded = %+v", req)
	}
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	again, err := DecodeRequest(out)
	if err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if again.V != req.V || again.ID != req.ID || again.Method != req.Method ||
		!bytesEqualParams(again.Params, req.Params) {
		t.Fatalf("round trip changed request: %+v vs %+v", again, req)
	}
}

func bytesEqualParams(a, b json.RawMessage) bool {
	return string(a) == string(b)
}

func TestDecodeRequest_AcceptsValid(t *testing.T) {
	for _, line := range []string{
		`{"v":2,"id":"1","method":"sys.hello","params":{"token":"` + strings.Repeat("ab", 32) + `","create_session":true}}`,
		`{"v":2,"id":"18446744073709551615","method":"sys.ping"}`,
		`{"v":2,"id":"3","method":"sys.ping","params":{}}`,
	} {
		if _, err := DecodeRequest([]byte(line)); err != nil {
			t.Errorf("DecodeRequest(%s) = %v, want nil", line, err)
		}
	}
}

func TestDecodeRequest_Rejects(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"wrong major", `{"v":1,"id":"1","method":"sys.ping"}`},
		{"missing major", `{"id":"1","method":"sys.ping"}`},
		{"zero id", `{"v":2,"id":"0","method":"sys.ping"}`},
		{"negative id", `{"v":2,"id":"-1","method":"sys.ping"}`},
		{"leading zero id", `{"v":2,"id":"01","method":"sys.ping"}`},
		{"non-decimal id", `{"v":2,"id":"1x","method":"sys.ping"}`},
		{"float id", `{"v":2,"id":1.0,"method":"sys.ping"}`},
		{"numeric id", `{"v":2,"id":42,"method":"sys.ping"}`},
		{"id overflow", `{"v":2,"id":"18446744073709551616","method":"sys.ping"}`},
		{"missing id", `{"v":2,"method":"sys.ping"}`},
		{"missing method", `{"v":2,"id":"1"}`},
		{"unknown field", `{"v":2,"id":"1","method":"sys.ping","extra":1}`},
		{"duplicate key", `{"v":2,"v":2,"id":"1","method":"sys.ping"}`},
		{"params not object", `{"v":2,"id":"1","method":"sys.ping","params":[1]}`},
		{"params string", `{"v":2,"id":"1","method":"sys.ping","params":"x"}`},
		{"duplicate params key", `{"v":2,"id":"1","method":"sys.ping","params":{"a":1,"a":2}}`},
		{"deep nesting", `{"v":2,"id":"1","method":"sys.ping","params":{"a":` +
			strings.Repeat(`{"b":`, 33) + `1` + strings.Repeat(`}`, 33) + `}}`},
		{"overflowing number", `{"v":2,"id":"1","method":"sys.ping","params":{"n":1e999}}`},
		{"trailing data", `{"v":2,"id":"1","method":"sys.ping"} {"v":2}`},
		{"array envelope", `[{"v":2,"id":"1"}]`},
		{"string envelope", `"sys.ping"`},
		{"empty", ``},
		{"invalid utf-8", "{\"v\":2,\"id\":\"1\",\"method\":\"\xff\xfe\"}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeRequest([]byte(tc.line)); err == nil {
				t.Fatalf("DecodeRequest(%s) = nil, want error", tc.line)
			}
		})
	}
}

func TestDecodeRequest_VersionErrorTyped(t *testing.T) {
	_, err := DecodeRequest([]byte(`{"v":1,"id":"1","method":"sys.ping"}`))
	var verr *VersionError
	if !errors.As(err, &verr) || verr.Got != 1 {
		t.Fatalf("err = %v, want *VersionError{1}", err)
	}
}

func TestValidateRequestID(t *testing.T) {
	if v, err := ValidateRequestID("1"); err != nil || v != 1 {
		t.Fatalf("ValidateRequestID(1) = %d, %v", v, err)
	}
	for _, bad := range []string{"", "0", "01", "+1", " 1", "1 ", "abc", "18446744073709551616"} {
		if _, err := ValidateRequestID(bad); err == nil {
			t.Errorf("ValidateRequestID(%q) = nil, want error", bad)
		}
	}
}

func TestDecodeResponse_Strict(t *testing.T) {
	line := `{"v":2,"id":"7","ok":true,"result":{"pong":true},"cut":"12","frame":"9"}`
	res, err := DecodeResponse([]byte(line))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.ID != "7" || !res.OK || res.Cut != "12" || res.Frame != "9" {
		t.Fatalf("decoded = %+v", res)
	}
	for _, bad := range []string{
		`{"v":1,"id":"7","ok":true}`,
		`{"v":2,"id":"7","ok":false}`,
		`{"v":2,"id":"7","ok":true,"error":{"code":"internal.panic"}}`,
		`{"v":2,"id":"7","ok":false,"error":{"code":"not.a.code"}}`,
		`{"v":2,"id":"7","ok":false,"error":{"code":"internal.panic"},"error":{"code":"auth.denied"}}`,
		`{"v":2,"id":"7","ok":true,"unknown":1}`,
		`{"v":2,"id":"7","ok":true} trailing`,
	} {
		if _, err := DecodeResponse([]byte(bad)); err == nil {
			t.Errorf("DecodeResponse(%s) = nil, want error", bad)
		}
	}
}

func TestSchemaDigest_IsStableAndSensitive(t *testing.T) {
	d1 := SchemaDigest()
	d2 := SchemaDigest()
	if d1 != d2 {
		t.Fatalf("digest not deterministic: %s vs %s", d1, d2)
	}
	if !strings.HasPrefix(d1, "sha256:") {
		t.Fatalf("digest = %s, want sha256: prefix", d1)
	}
}

func TestRegistry_AdvertisedSurfaceHasNoWrite(t *testing.T) {
	for _, spec := range AdvertisedRegistry() {
		if spec.Grant == "write" {
			t.Errorf("advertised method %s carries the invalid write grant", spec.Name)
		}
		if spec.Name == "store.write" {
			t.Errorf("store.write must not exist in any registry")
		}
	}
	for _, name := range MethodNames() {
		if name == "store.write" {
			t.Fatalf("store.write remains in the registry")
		}
	}
}

// FuzzEnvelope is the wire-grammar fuzz target: no input, however hostile,
// may panic DecodeRequest or DecodeResponse. The committed seeds cover
// valid envelopes and the documented rejection modes.
func FuzzEnvelope(f *testing.F) {
	f.Add(`{"v":2,"id":"1","method":"sys.ping","params":{}}`)
	f.Add(`{"v":1,"id":"1","method":"sys.ping"}`)
	f.Add(`{"v":2,"id":"0","method":"sys.ping"}`)
	f.Add(`{"v":2,"id":"1","method":"sys.ping","params":{"a":1,"a":1}}`)
	f.Add(`{"v":2,"id":"1","method":"sys.ping","params":{"n":1e999}}`)
	f.Add(`{"v":2,"id":"7","ok":true,"result":{"pong":true},"cut":"12","frame":"9"}`)
	f.Add(`{"v":2,"id":"7","ok":false,"error":{"code":"internal.panic"}}`)
	f.Add("")
	f.Fuzz(func(t *testing.T, line string) {
		// The property is "no panic": every input, however hostile, is a
		// typed error or a valid envelope — never a crash.
		_, _ = DecodeRequest([]byte(line))
		_, _ = DecodeResponse([]byte(line))
	})
}
