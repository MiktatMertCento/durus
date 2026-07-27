package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://durus:durus@127.0.0.1:5433/durus?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := openDB(ctx, url)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM live_session`)
		_, _ = db.Exec(context.Background(), `DELETE FROM session_archives WHERE id LIKE 'test-%' OR length(id) > 10`)
		db.Close()
	})

	// isolate each test run
	_, _ = db.Exec(ctx, `DELETE FROM live_session`)
	store, err := NewStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestAwaitOvertimeAndAdvance(t *testing.T) {
	store := testStore(t)
	if _, err := store.Start(); err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	store.session.PhaseStartedAt = time.Now().UTC().Add(-41 * time.Minute)
	store.session.PhaseBeganAt = store.session.PhaseStartedAt
	store.mu.Unlock()

	st := store.Snapshot()
	if !st.AwaitingAdvance {
		t.Fatalf("expected awaiting, got %+v", st)
	}
	if st.OvertimeSec < 60 {
		t.Fatalf("expected overtime, got %v", st.OvertimeSec)
	}
	if st.RemainingSec >= 0 {
		t.Fatalf("remaining should be negative, got %v", st.RemainingSec)
	}

	st, err := store.Advance()
	if err != nil {
		t.Fatal(err)
	}
	if st.AwaitingAdvance {
		t.Fatal("still awaiting")
	}
	if st.Phase != PhaseStanding {
		t.Fatalf("phase=%v", st.Phase)
	}
	if st.TransitionCount != 1 {
		t.Fatalf("count=%d", st.TransitionCount)
	}

	store.mu.Lock()
	late := store.session.Phases[0].LateSec
	store.mu.Unlock()
	if late < 60 {
		t.Fatalf("lateSec=%v", late)
	}
}

func TestCloseArchivesSession(t *testing.T) {
	store := testStore(t)
	st, err := store.Start()
	if err != nil {
		t.Fatal(err)
	}
	id := st.ID

	store.mu.Lock()
	store.session.PhaseStartedAt = time.Now().UTC().Add(-41 * time.Minute)
	store.session.PhaseBeganAt = store.session.PhaseStartedAt
	store.mu.Unlock()
	if _, err := store.Advance(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Close(); err != nil {
		t.Fatal(err)
	}

	items, err := store.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("archived session missing from list")
	}

	arch, err := store.GetSession(id)
	if err != nil {
		t.Fatal(err)
	}
	if arch.Summary.Transitions != 1 {
		t.Fatalf("transitions=%d", arch.Summary.Transitions)
	}
	if len(arch.Phases) < 2 {
		t.Fatalf("phases=%d", len(arch.Phases))
	}
}
