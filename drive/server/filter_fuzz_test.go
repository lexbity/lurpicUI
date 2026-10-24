package server

import "testing"

func FuzzFilter(f *testing.F) {
	for _, seed := range []string{
		`{"filter":{"class":"state.changed","store":"e1.chartType"}}`,
		`{"filter":{"class":"tree.changed","test_id":"shell.cmdk"}}`,
		`{"filter":{"class":"frame.completed"}}`,
		`{"filter":{"class":"own.preempted"}}`,
		`{}`,
		`{"filter":{"class":"unknown"}}`,
		`{"filter":{"class":7}}`,
		`null`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = parseSubscriptionFilter(raw)
	})
}
