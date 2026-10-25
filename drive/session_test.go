package drive

import (
	"testing"
	"time"
)

func TestResumeGenerationFencesOldEOF(t *testing.T) {
	inst, err := NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	table := NewSessionTable(inst)
	sess, _, err := table.Create(Grants{Read: true, Command: true})
	if err != nil {
		t.Fatal(err)
	}
	oldGeneration := sess.Generation()
	newGeneration, err := sess.Attach(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if newGeneration != oldGeneration+1 {
		t.Fatalf("generation = %d, want %d", newGeneration, oldGeneration+1)
	}
	if sess.DetachGeneration(oldGeneration, time.Now()) {
		t.Fatal("delayed EOF from the replaced generation detached the session")
	}
	if sess.Generation() != newGeneration || sess.State() != SessionAttached {
		t.Fatalf("session after old EOF = generation %d, state %s", sess.Generation(), sess.State())
	}
	if !sess.DetachGeneration(newGeneration, time.Now()) || sess.State() != SessionDetached {
		t.Fatal("current generation EOF did not detach the session")
	}
}

func TestSessionInstanceRestart(t *testing.T) {
	firstInstance, err := NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	first := NewSessionTable(firstInstance)
	session, credential, err := first.Create(Grants{Read: true, Command: true})
	if err != nil {
		t.Fatal(err)
	}
	session.Detach(fixedNow())

	secondInstance, err := NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	second := NewSessionTable(secondInstance)
	if _, _, err := second.Resume(session.ID, credential); err == nil {
		t.Fatal("a fresh app instance must not recreate an old session identity")
	}
	if second.Count() != 0 {
		t.Fatalf("resume after restart allocated %d sessions", second.Count())
	}
}
