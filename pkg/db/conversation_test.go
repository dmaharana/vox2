package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConversationCRUDAndCSV(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "db-conv-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// 1. Create Conversation
	conv, err := database.CreateConversation("Test Conversation 1")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	// 2. Add Messages
	err = database.AddMessage(Message{
		ConversationID: conv.ID,
		Role:           "user",
		Content:        "Hello Go Harness",
	})
	if err != nil {
		t.Fatalf("failed to add message 1: %v", err)
	}

	err = database.AddMessage(Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        "Hello! I am ready to assist you.",
	})
	if err != nil {
		t.Fatalf("failed to add message 2: %v", err)
	}

	// 3. Get Messages
	messages, err := database.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("failed to get messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// 4. Export CSV
	csvStr, err := database.ExportConversationCSV(conv.ID)
	if err != nil {
		t.Fatalf("failed to export CSV: %v", err)
	}
	if !strings.Contains(csvStr, "Hello Go Harness") || !strings.Contains(csvStr, "Role") {
		t.Errorf("unexpected CSV content: %s", csvStr)
	}

	// 5. List and Delete
	list, err := database.ListConversations()
	if err != nil || len(list) != 1 {
		t.Fatalf("list error or length mismatch: %v, len=%d", err, len(list))
	}

	if err := database.DeleteConversation(conv.ID); err != nil {
		t.Fatalf("failed to delete conversation: %v", err)
	}

	listAfter, _ := database.ListConversations()
	if len(listAfter) != 0 {
		t.Errorf("expected 0 conversations after delete, got %d", len(listAfter))
	}
}
