package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBuiltinToolsCRUD(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tools-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	reg := NewRegistry()
	RegisterBuiltinTools(reg, tempDir)

	ctx := context.Background()

	// 1. write_file
	writeArgs, _ := json.Marshal(WriteFileInput{
		Path:    "subdir/test.txt",
		Content: "Line 1: Go Harness\nLine 2: AI Agent\nLine 3: Tool Testing\n",
	})
	res, err := reg.Execute(ctx, "write_file", writeArgs)
	if err != nil {
		t.Fatalf("write_file failed: %v", err)
	}
	writeRes := res.(WriteFileOutput)
	if !writeRes.Success {
		t.Errorf("expected write_file to succeed")
	}

	// 2. read_file with line slicing
	readArgs, _ := json.Marshal(ReadFileInput{
		Path:      "subdir/test.txt",
		StartLine: 2,
		EndLine:   3,
	})
	res, err = reg.Execute(ctx, "read_file", readArgs)
	if err != nil {
		t.Fatalf("read_file failed: %v", err)
	}
	readRes := res.(ReadFileOutput)
	if !strings.Contains(readRes.Content, "Line 2: AI Agent") || !strings.Contains(readRes.Content, "Line 3: Tool Testing") {
		t.Errorf("unexpected read content: %s", readRes.Content)
	}

	// 3. update_file
	updateArgs, _ := json.Marshal(UpdateFileInput{
		Path:               "subdir/test.txt",
		TargetContent:      "Line 2: AI Agent",
		ReplacementContent: "Line 2: Supercharged Agent",
	})
	res, err = reg.Execute(ctx, "update_file", updateArgs)
	if err != nil {
		t.Fatalf("update_file failed: %v", err)
	}
	updateRes := res.(UpdateFileOutput)
	if !updateRes.Success || updateRes.Replaced != 1 {
		t.Errorf("update_file failed: %+v", updateRes)
	}

	// 4. list_directory
	listArgs, _ := json.Marshal(ListDirectoryInput{
		Path:      ".",
		Recursive: true,
	})
	res, err = reg.Execute(ctx, "list_directory", listArgs)
	if err != nil {
		t.Fatalf("list_directory failed: %v", err)
	}
	listRes := res.(ListDirectoryOutput)
	if len(listRes.Items) == 0 {
		t.Errorf("expected directory items, got 0")
	}

	// 5. Tool Enable/Disable
	reg.SetEnabled("read_file", false)
	_, err = reg.Execute(ctx, "read_file", readArgs)
	if err == nil {
		t.Errorf("expected error executing disabled tool")
	}

	// 6. ToOpenAITools
	openAITools := reg.ToOpenAITools()
	for _, tool := range openAITools {
		if tool.Function.Name == "read_file" {
			t.Errorf("disabled tool should not be in openai tools")
		}
	}
}
