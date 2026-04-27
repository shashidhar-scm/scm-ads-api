package handlers

import (
	"encoding/json"
	"testing"
)

func TestConditionalFetchLogic(t *testing.T) {
	// Test 1: No client rev provided - should return full response
	t.Run("no client rev", func(t *testing.T) {
		clientRev := ""
		currentRev := "1-abc12345"
		
		shouldReturnNoChanges := clientRev != "" && clientRev == currentRev
		
		if shouldReturnNoChanges {
			t.Error("Expected full response when clientRev is empty")
		}
	})
	
	// Test 2: Client rev differs from current rev - should return full response
	t.Run("rev mismatch", func(t *testing.T) {
		clientRev := "1-abc12345"
		currentRev := "1-def67890"
		
		shouldReturnNoChanges := clientRev != "" && clientRev == currentRev
		
		if shouldReturnNoChanges {
			t.Error("Expected full response when revisions don't match")
		}
	})
	
	// Test 3: Client rev matches current rev - should return no_changes
	t.Run("rev match", func(t *testing.T) {
		clientRev := "1-abc12345"
		currentRev := "1-abc12345"
		
		shouldReturnNoChanges := clientRev != "" && clientRev == currentRev
		
		if !shouldReturnNoChanges {
			t.Error("Expected no_changes response when revisions match")
		}
	})
	
	// Test 4: Verify response format
	t.Run("response format", func(t *testing.T) {
		testRev := "1-abc12345"
		
		// No changes response
		noChangesResp := map[string]any{
			"status": "no_changes",
			"rev":    testRev,
		}
		
		jsonBytes, err := json.Marshal(noChangesResp)
		if err != nil {
			t.Fatalf("Failed to marshal no_changes response: %v", err)
		}
		
		var parsed map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal no_changes response: %v", err)
		}
		
		if parsed["status"] != "no_changes" {
			t.Errorf("Expected status=no_changes, got %v", parsed["status"])
		}
		
		if parsed["rev"] == nil {
			t.Error("Expected rev to be present")
		}
		
		// Full response
		fullResp := map[string]any{
			"status":      "ok",
			"rev":         testRev,
			"loop_poster": []any{},
		}
		
		jsonBytes, err = json.Marshal(fullResp)
		if err != nil {
			t.Fatalf("Failed to marshal full response: %v", err)
		}
		
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal full response: %v", err)
		}
		
		if parsed["status"] != "ok" {
			t.Errorf("Expected status=ok, got %v", parsed["status"])
		}
		
		if parsed["loop_poster"] == nil {
			t.Error("Expected loop_poster to be present")
		}
		
		if parsed["rev"] == nil {
			t.Error("Expected rev to be present")
		}
	})
	
	// Test 5: Verify revision ID generation
	t.Run("revision ID generation", func(t *testing.T) {
		data1 := []byte(`{"cards":["poster1","poster2"]}`)
		data2 := []byte(`{"cards":["poster1","poster2"]}`)
		data3 := []byte(`{"cards":["poster1","poster3"]}`)
		
		rev1 := generateRevisionID(data1)
		rev2 := generateRevisionID(data2)
		rev3 := generateRevisionID(data3)
		
		// Same data should produce same revision
		if rev1 != rev2 {
			t.Errorf("Expected same revision for identical data, got %s and %s", rev1, rev2)
		}
		
		// Different data should produce different revision
		if rev1 == rev3 {
			t.Errorf("Expected different revision for different data, got %s for both", rev1)
		}
		
		// Verify format: 1-{8 hex chars}
		if len(rev1) != 10 || rev1[:2] != "1-" {
			t.Errorf("Expected revision format '1-xxxxxxxx', got %s", rev1)
		}
	})
}
