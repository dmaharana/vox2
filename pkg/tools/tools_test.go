package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestEditToolMultiAndDiffs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "edit-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	reg := NewRegistry()
	RegisterBuiltinTools(reg, tempDir)
	ctx := context.Background()

	filePath := filepath.Join(tempDir, "sample.txt")
	initialContent := "alpha\nbeta\ngamma\ndelta\nepsilon\n"
	_ = os.WriteFile(filePath, []byte(initialContent), 0644)

	// 1. Multiple disjoint edits in one call
	editPayload := map[string]any{
		"path": "sample.txt",
		"edits": []map[string]string{
			{"oldText": "alpha\n", "newText": "ALPHA\n"},
			{"oldText": "gamma\n", "newText": "GAMMA\n"},
		},
	}
	rawArgs, _ := json.Marshal(editPayload)

	res, err := reg.Execute(ctx, "edit", rawArgs)
	if err != nil {
		t.Fatalf("edit execution failed: %v", err)
	}

	out, ok := res.(EditFileOutput)
	if !ok || !out.Success || out.ReplacedCount != 2 {
		t.Fatalf("unexpected edit result: %+v", res)
	}

	// Verify diff and patch output
	if !strings.Contains(out.Diff, "ALPHA") || !strings.Contains(out.Diff, "GAMMA") {
		t.Errorf("expected visual diff to contain ALPHA & GAMMA: %s", out.Diff)
	}
	if !strings.Contains(out.Patch, "--- sample.txt") || !strings.Contains(out.Patch, "+++ sample.txt") {
		t.Errorf("expected unified patch: %s", out.Patch)
	}

	// Verify file content on disk
	updatedBytes, _ := os.ReadFile(filePath)
	expectedContent := "ALPHA\nbeta\nGAMMA\ndelta\nepsilon\n"
	if string(updatedBytes) != expectedContent {
		t.Errorf("file content mismatch. Expected:\n%s\nGot:\n%s", expectedContent, string(updatedBytes))
	}

	// 2. Overlapping edits rejection
	overlapPayload := map[string]any{
		"path": "sample.txt",
		"edits": []map[string]string{
			{"oldText": "ALPHA\nbeta\n", "newText": "ONE\n"},
			{"oldText": "beta\nGAMMA\n", "newText": "TWO\n"},
		},
	}
	rawOverlap, _ := json.Marshal(overlapPayload)
	_, err = reg.Execute(ctx, "edit", rawOverlap)
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Errorf("expected overlap error, got: %v", err)
	}

	// 3. Duplicate occurrence error
	_ = os.WriteFile(filePath, []byte("repeat\nrepeat\nrepeat\n"), 0644)
	dupPayload := map[string]any{
		"path": "sample.txt",
		"edits": []map[string]string{
			{"oldText": "repeat", "newText": "unique"},
		},
	}
	rawDup, _ := json.Marshal(dupPayload)
	_, err = reg.Execute(ctx, "edit", rawDup)
	if err == nil || !strings.Contains(err.Error(), "Found 3 occurrences") {
		t.Errorf("expected duplicate error with count 3, got: %v", err)
	}

	// 4. Not found error
	notFoundPayload := map[string]any{
		"path": "sample.txt",
		"edits": []map[string]string{
			{"oldText": "non-existent-line", "newText": "something"},
		},
	}
	rawNotFound, _ := json.Marshal(notFoundPayload)
	_, err = reg.Execute(ctx, "edit", rawNotFound)
	if err == nil || !strings.Contains(err.Error(), "Could not find the exact text") {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestEditToolFuzzyNormalizationAndEndings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "edit-fuzzy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	reg := NewRegistry()
	RegisterBuiltinTools(reg, tempDir)
	ctx := context.Background()

	// 1. CRLF line endings preservation
	crlfFile := filepath.Join(tempDir, "crlf.txt")
	_ = os.WriteFile(crlfFile, []byte("line 1\r\nline 2\r\nline 3\r\n"), 0644)

	editCRLF, _ := json.Marshal(map[string]any{
		"path": "crlf.txt",
		"edits": []map[string]string{
			{"oldText": "line 2", "newText": "LINE TWO"},
		},
	})
	_, err = reg.Execute(ctx, "edit", editCRLF)
	if err != nil {
		t.Fatalf("CRLF edit failed: %v", err)
	}
	crlfBytes, _ := os.ReadFile(crlfFile)
	if !strings.Contains(string(crlfBytes), "line 1\r\nLINE TWO\r\nline 3\r\n") {
		t.Errorf("CRLF line endings were not preserved: %q", string(crlfBytes))
	}

	// 2. BOM preservation
	bomFile := filepath.Join(tempDir, "bom.txt")
	_ = os.WriteFile(bomFile, []byte("\xef\xbb\xbfheader\nbody\n"), 0644)

	editBOM, _ := json.Marshal(map[string]any{
		"path": "bom.txt",
		"edits": []map[string]string{
			{"oldText": "body", "newText": "MODIFIED_BODY"},
		},
	})
	_, err = reg.Execute(ctx, "edit", editBOM)
	if err != nil {
		t.Fatalf("BOM edit failed: %v", err)
	}
	bomBytes, _ := os.ReadFile(bomFile)
	if !strings.HasPrefix(string(bomBytes), "\xef\xbb\xbf") || !strings.Contains(string(bomBytes), "MODIFIED_BODY") {
		t.Errorf("BOM header was not preserved: %q", string(bomBytes))
	}

	// 3. Fuzzy matching: Smart quotes, Unicode dashes, trailing spaces
	fuzzyFile := filepath.Join(tempDir, "fuzzy.txt")
	// Original file contains smart quotes and unicode dash and trailing space
	_ = os.WriteFile(fuzzyFile, []byte("function test() {   \n  let quote = “hello”;  \n  let dash = 1–2;\n}\n"), 0644)

	// Model sends ASCII quotes and ASCII dash and clean trailing space
	editFuzzy, _ := json.Marshal(map[string]any{
		"path": "fuzzy.txt",
		"edits": []map[string]string{
			{"oldText": "let quote = \"hello\";\n  let dash = 1-2;", "newText": "let quote = \"world\";\n  let dash = 3-4;"},
		},
	})
	_, err = reg.Execute(ctx, "edit_file", editFuzzy)
	if err != nil {
		t.Fatalf("fuzzy edit failed: %v", err)
	}

	fuzzyBytes, _ := os.ReadFile(fuzzyFile)
	if !strings.Contains(string(fuzzyBytes), "world") || !strings.Contains(string(fuzzyBytes), "3-4") {
		t.Errorf("fuzzy edit did not apply replacement: %s", string(fuzzyBytes))
	}

	// 4. Tolerant input formats (legacy top-level oldText/newText, JSON string edits)
	singleFile := filepath.Join(tempDir, "single.txt")
	_ = os.WriteFile(singleFile, []byte("start text\n"), 0644)

	legacyInput, _ := json.Marshal(map[string]any{
		"path":    "single.txt",
		"oldText": "start text",
		"newText": "finished text",
	})
	_, err = reg.Execute(ctx, "edit", legacyInput)
	if err != nil {
		t.Fatalf("legacy input edit failed: %v", err)
	}
	singleBytes, _ := os.ReadFile(singleFile)
	if string(singleBytes) != "finished text\n" {
		t.Errorf("expected finished text, got: %s", string(singleBytes))
	}
}

func TestExecuteScriptTool(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltinTools(reg, ".")

	ctx := context.Background()

	// Test Python execution
	pythonArgs, _ := json.Marshal(ExecuteScriptInput{
		Language: "python",
		Code:     "import sys\nprint('Hello from Python script')\n",
	})
	res, err := reg.Execute(ctx, "execute_script", pythonArgs)
	if err != nil {
		t.Logf("Python interpreter not available, skipping: %v", err)
	} else {
		out := res.(ExecuteScriptOutput)
		if !strings.Contains(out.Stdout, "Hello from Python script") {
			t.Errorf("unexpected python stdout: %s", out.Stdout)
		}
		if out.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", out.ExitCode)
		}
	}

	// Test Bash execution
	bashArgs, _ := json.Marshal(ExecuteScriptInput{
		Language: "bash",
		Code:     "echo 'Hello from Bash script'\n",
	})
	res, err = reg.Execute(ctx, "execute_script", bashArgs)
	if err != nil {
		t.Fatalf("bash execution failed: %v", err)
	}
	bashOut := res.(ExecuteScriptOutput)
	if !strings.Contains(bashOut.Stdout, "Hello from Bash script") {
		t.Errorf("unexpected bash stdout: %s", bashOut.Stdout)
	}
	if bashOut.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", bashOut.ExitCode)
	}

	// Test unsupported language error
	invalidArgs, _ := json.Marshal(ExecuteScriptInput{
		Language: "unsupported_lang_xyz",
		Code:     "echo test\n",
	})
	_, err = reg.Execute(ctx, "execute_script", invalidArgs)
	if err == nil {
		t.Errorf("expected error for unsupported language")
	}

	// Test empty code error
	emptyCodeArgs, _ := json.Marshal(ExecuteScriptInput{
		Language: "python",
		Code:     "   \n",
	})
	_, err = reg.Execute(ctx, "execute_script", emptyCodeArgs)
	if err == nil {
		t.Errorf("expected error for empty code")
	}
}
