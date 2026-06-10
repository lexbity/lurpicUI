package withtests

import "testing"

func TestDoSomething(t *testing.T) {
	if DoSomething() != 42 {
		t.Fatal("unexpected")
	}
}
