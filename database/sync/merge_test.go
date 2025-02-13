package sync

import (
	"testing"

	"nyiyui.ca/jts/data"
)

func TestMerge1(t *testing.T) {
	original := []data.Session{
		{ID: "1", Description: "learn Haskell"},
		{ID: "2", Description: "learn Rust"},
		{ID: "3", Description: "10x engineer"},
	}
	local := []data.Session{
		{ID: "1", Description: "learn Haskell"},
		{ID: "2", Description: "learn Rust"},
		{ID: "3", Description: "10x engineer"},
		{ID: "4", Description: "learn Go"},
	}
	remote := []data.Session{
		{ID: "1", Description: "learn Haskell"},
		{ID: "2", Description: "learn Rust"},
		{ID: "3", Description: "10x engineer"},
	}
	changes, conflicts := mergeSlice(mergeSession, getIDSession, original, local, remote)
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %v", conflicts)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %v", len(changes))
	}
	if changes[0].Operation != ChangeOperationExist {
		t.Fatalf("expected change operation %v, got %v", ChangeOperationExist, changes[0].Operation)
	}
	if changes[0].Data.ID != "4" {
		t.Fatalf("expected change ID 4, got %v", changes[0].Data.ID)
	}
}

func TestMerge2(t *testing.T) {
	original := []data.Session{
		{ID: "1", Description: "learn Haskell"},
		{ID: "2", Description: "learn Rust"},
		{ID: "3", Description: "10x engineer"},
	}
	local := []data.Session{
		{ID: "1", Description: "learn Haskell"},
		{ID: "2", Description: "learn Rust"},
		{ID: "3", Description: "10x engineer"},
	}
	remote := []data.Session{
		{ID: "1", Description: "learn Haskell"},
		{ID: "2", Description: "learn Rust"},
		{ID: "3", Description: "10x engineer"},
		{ID: "4", Description: "learn Go"},
	}
	changes, conflicts := mergeSlice(mergeSession, getIDSession, original, local, remote)
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %v", conflicts)
	}
	if len(changes) != 0 {
		t.Fatalf("expected no changes, got %v", len(changes))
	}
}
