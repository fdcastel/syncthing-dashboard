package demo

import (
	"context"
	"testing"
	"time"
)

func TestDemoCollectorProducesRichSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := NewCollector(5 * time.Millisecond)
	c.Start(ctx)
	time.Sleep(20 * time.Millisecond)

	snapshot, ok := c.Snapshot()
	if !ok {
		t.Fatalf("expected snapshot to be available")
	}

	if !snapshot.SourceOnline {
		t.Fatalf("expected source_online to be true in demo mode")
	}
	if len(snapshot.Folders) != 10 {
		t.Fatalf("expected 10 demo folders, got %d", len(snapshot.Folders))
	}
	if len(snapshot.Remotes) != 4 {
		t.Fatalf("expected 4 demo remotes, got %d", len(snapshot.Remotes))
	}

	var hasSyncing bool
	var hasLocalChanges bool
	var hasError bool
	var hasPaused bool
	for _, folder := range snapshot.Folders {
		if folder.NeedItems > 0 || folder.NeedBytes > 0 {
			hasSyncing = true
		}
		if folder.LocalChangesItems > 0 {
			hasLocalChanges = true
		}
		if folder.State == "error" {
			hasError = true
		}
		if folder.State == "paused" {
			hasPaused = true
		}
	}

	if !hasSyncing || !hasLocalChanges || !hasError || !hasPaused {
		t.Fatalf("expected mixed demo folder states")
	}

	var hasDisconnectedRemote bool
	var hasNamedRemote bool
	for _, remote := range snapshot.Remotes {
		if !remote.Connected {
			hasDisconnectedRemote = true
		}
		if remote.Name == "Attic" || remote.Name == "Desk" || remote.Name == "Backpack" || remote.Name == "Keyring" {
			hasNamedRemote = true
		}
	}
	if !hasDisconnectedRemote {
		t.Fatalf("expected at least one disconnected demo remote")
	}
	if !hasNamedRemote {
		t.Fatalf("expected friendly demo remote names")
	}
	if len(snapshot.Alerts) == 0 {
		t.Fatalf("expected alerts in demo snapshot")
	}
}

func TestDemoCollectorProgressMoves(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := NewCollector(5 * time.Millisecond)
	c.Start(ctx)
	time.Sleep(10 * time.Millisecond)
	first, ok := c.Snapshot()
	if !ok {
		t.Fatalf("expected first snapshot")
	}

	time.Sleep(20 * time.Millisecond)
	second, ok := c.Snapshot()
	if !ok {
		t.Fatalf("expected second snapshot")
	}

	var firstPct float64
	var secondPct float64
	var foundFirst bool
	var foundSecond bool
	for _, folder := range first.Folders {
		if folder.ID == "folder-media" && folder.CompletionPct != nil {
			firstPct = *folder.CompletionPct
			foundFirst = true
			break
		}
	}
	for _, folder := range second.Folders {
		if folder.ID == "folder-media" && folder.CompletionPct != nil {
			secondPct = *folder.CompletionPct
			foundSecond = true
			break
		}
	}

	if !foundFirst || !foundSecond {
		t.Fatalf("expected demo folder folder-media in snapshots")
	}
	if firstPct == secondPct {
		t.Fatalf("expected demo progress to evolve over time")
	}
}
