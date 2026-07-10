package datastore

import (
	"os"
	"testing"
)

func TestDBClear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twnetmap_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := NewDB(tmpDir)
	if err != nil {
		t.Fatalf("failed to create DB: %v", err)
	}
	defer db.Close()

	// Initial clear on empty buckets (they should exist because of initBuckets)
	if err := db.ClearAllNodes(); err != nil {
		t.Errorf("ClearAllNodes failed on empty database: %v", err)
	}
	if err := db.ClearAllLinks(); err != nil {
		t.Errorf("ClearAllLinks failed on empty database: %v", err)
	}

	// Insert a dummy node and link
	node := &Node{ID: "node_1", IP: "192.168.1.1", Label: "Test Node"}
	if err := db.SaveNode(node); err != nil {
		t.Fatalf("SaveNode failed: %v", err)
	}

	link := &Link{ID: "link_1", From: "node_1", To: "node_2"}
	if err := db.SaveLink(link); err != nil {
		t.Fatalf("SaveLink failed: %v", err)
	}

	// Verify they are saved
	nodes, err := db.GetNodes()
	if err != nil || len(nodes) != 1 {
		t.Fatalf("Expected 1 node, got: %v (err: %v)", nodes, err)
	}
	links, err := db.GetLinks()
	if err != nil || len(links) != 1 {
		t.Fatalf("Expected 1 link, got: %v (err: %v)", links, err)
	}

	// Clear again
	if err := db.ClearAllNodes(); err != nil {
		t.Errorf("ClearAllNodes failed: %v", err)
	}
	if err := db.ClearAllLinks(); err != nil {
		t.Errorf("ClearAllLinks failed: %v", err)
	}

	// Verify they are cleared
	nodes, err = db.GetNodes()
	if err != nil || len(nodes) != 0 {
		t.Errorf("Expected 0 nodes after clear, got: %v (err: %v)", nodes, err)
	}
	links, err = db.GetLinks()
	if err != nil || len(links) != 0 {
		t.Errorf("Expected 0 links after clear, got: %v (err: %v)", links, err)
	}
}
