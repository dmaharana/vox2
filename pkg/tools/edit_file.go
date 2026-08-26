package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/sashabaranov/go-openai/jsonschema"
	"golang.org/x/text/unicode/norm"
)

// Edit represents a single targeted text replacement.
type Edit struct {
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

// EditFileInput represents the input parameters for the edit / edit_file tool.
type EditFileInput struct {
	Path     string `json:"path"`
	FilePath string `json:"file_path,omitempty"`
	Edits    []Edit `json:"edits"`
	OldText  string `json:"oldText,omitempty"`
	NewText  string `json:"newText,omitempty"`
}

// EditFileOutput represents the structured response of an edit operation.
type EditFileOutput struct {
	Path             string `json:"path"`
	Message          string `json:"message"`
	Diff             string `json:"diff"`
	Patch            string `json:"patch"`
	FirstChangedLine int    `json:"first_changed_line,omitempty"`
	ReplacedCount    int    `json:"replaced_count"`
	Success          bool   `json:"success"`
}

// NewEditTool creates the advanced exact/fuzzy text replacement tool with multi-edit support.
func NewEditTool(baseDir string) ToolDefinition {
	return ToolDefinition{
		Name: "edit",
		Description: "Edit a file using targeted exact or fuzzy text replacement with support for multiple disjoint edits in one call. " +
			"Every edit's oldText must match a unique region of the original file. Generates unified diff patch and line-numbered diff view.",
		Category: "builtin",
		Enabled:  true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {
					Type:        jsonschema.String,
					Description: "Path to the file to edit (relative to workspace or absolute)",
				},
				"edits": {
					Type: jsonschema.Array,
					Description: "One or more targeted replacements. Each edit is matched against the original file simultaneously. " +
						"Do not include overlapping edits.",
					Items: &jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"oldText": {
								Type:        jsonschema.String,
								Description: "Exact text for one targeted replacement. It must be unique in the original file.",
							},
							"newText": {
								Type:        jsonschema.String,
								Description: "Replacement text for this targeted edit.",
							},
						},
						Required: []string{"oldText", "newText"},
					},
				},
			},
			Required: []string{"path"},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			return executeEditTool(ctx, baseDir, args)
		},
	}
}

// NewEditFileTool creates an alias tool definition for edit_file.
func NewEditFileTool(baseDir string) ToolDefinition {
	tool := NewEditTool(baseDir)
	tool.Name = "edit_file"
	return tool
}

func executeEditTool(ctx context.Context, baseDir string, rawArgs json.RawMessage) (any, error) {
	filePath, edits, err := parseEditArguments(rawArgs)
	if err != nil {
		return nil, err
	}

	if filePath == "" {
		return nil, errors.New("path is required for edit tool")
	}
	if len(edits) == 0 {
		return nil, errors.New("edits must contain at least one replacement")
	}

	targetPath := resolvePath(baseDir, filePath)
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("could not edit file: %s. File does not exist", filePath)
		}
		return nil, fmt.Errorf("could not edit file: %s. %w", filePath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("could not edit file: %s. Path is a directory", filePath)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file '%s': %w", filePath, err)
	}

	rawContent := string(data)
	hasBOM, content := stripBOM(rawContent)
	originalEnding := detectLineEnding(content)
	normalizedContent := normalizeToLF(content)

	baseContent, newContent, err := applyEditsToNormalizedContent(normalizedContent, edits, filePath)
	if err != nil {
		return nil, err
	}

	// Restore line endings and BOM
	finalContent := restoreLineEndings(newContent, originalEnding)
	if hasBOM {
		finalContent = "\xef\xbb\xbf" + finalContent
	}

	if err := os.WriteFile(targetPath, []byte(finalContent), info.Mode().Perm()); err != nil {
		return nil, fmt.Errorf("failed to write updated file '%s': %w", filePath, err)
	}

	diffText, firstChangedLine := GenerateDisplayDiff(baseContent, newContent, 4)
	patchText := GenerateUnifiedPatch(filePath, baseContent, newContent, 4)

	return EditFileOutput{
		Path:             filePath,
		Message:          fmt.Sprintf("Successfully replaced %d block(s) in %s.", len(edits), filePath),
		Diff:             diffText,
		Patch:            patchText,
		FirstChangedLine: firstChangedLine,
		ReplacedCount:    len(edits),
		Success:          true,
	}, nil
}

func parseEditArguments(rawArgs json.RawMessage) (string, []Edit, error) {
	var rawMap map[string]any
	if err := json.Unmarshal(rawArgs, &rawMap); err != nil {
		return "", nil, fmt.Errorf("invalid edit JSON payload: %w", err)
	}

	filePath := ""
	if p, ok := rawMap["path"].(string); ok {
		filePath = p
	} else if p, ok := rawMap["file_path"].(string); ok {
		filePath = p
	}

	var edits []Edit

	// 1. Check if edits is provided
	if rawEdits, exists := rawMap["edits"]; exists && rawEdits != nil {
		switch val := rawEdits.(type) {
		case string:
			// JSON encoded string
			var parsed []Edit
			if err := json.Unmarshal([]byte(val), &parsed); err == nil && len(parsed) > 0 {
				edits = parsed
			} else {
				var single Edit
				if err := json.Unmarshal([]byte(val), &single); err == nil && single.OldText != "" {
					edits = []Edit{single}
				}
			}
		case []any:
			for _, item := range val {
				if itemMap, ok := item.(map[string]any); ok {
					oldText, _ := itemMap["oldText"].(string)
					newText, _ := itemMap["newText"].(string)
					edits = append(edits, Edit{OldText: oldText, NewText: newText})
				}
			}
		case map[string]any:
			oldText, _ := val["oldText"].(string)
			newText, _ := val["newText"].(string)
			if oldText != "" {
				edits = []Edit{{OldText: oldText, NewText: newText}}
			}
		}
	}

	// 2. Legacy / root-level oldText/newText fallback
	if len(edits) == 0 {
		oldText, hasOld := rawMap["oldText"].(string)
		newText, hasNew := rawMap["newText"].(string)
		if hasOld && hasNew {
			edits = append(edits, Edit{OldText: oldText, NewText: newText})
		}
	}

	return filePath, edits, nil
}

type fuzzyMatchResult struct {
	found          bool
	index          int
	matchLength    int
	usedFuzzyMatch bool
}

type matchedEdit struct {
	editIndex   int
	matchIndex  int
	matchLength int
	newText     string
}

type lineSpan struct {
	start int
	end   int
}

func applyEditsToNormalizedContent(normalizedContent string, edits []Edit, path string) (string, string, error) {
	totalEdits := len(edits)
	normalizedEdits := make([]Edit, len(edits))
	for i, e := range edits {
		normalizedEdits[i] = Edit{
			OldText: normalizeToLF(e.OldText),
			NewText: normalizeToLF(e.NewText),
		}
		if len(normalizedEdits[i].OldText) == 0 {
			if totalEdits == 1 {
				return "", "", fmt.Errorf("oldText must not be empty in %s.", path)
			}
			return "", "", fmt.Errorf("edits[%d].oldText must not be empty in %s.", i, path)
		}
	}

	// Check if any edit requires fuzzy matching
	var anyFuzzy bool
	for _, edit := range normalizedEdits {
		res := fuzzyFindText(normalizedContent, edit.OldText)
		if res.usedFuzzyMatch {
			anyFuzzy = true
			break
		}
	}

	replacementBaseContent := normalizedContent
	if anyFuzzy {
		replacementBaseContent = normalizeForFuzzyMatch(normalizedContent)
	}

	var matchedEdits []matchedEdit
	for i, edit := range normalizedEdits {
		targetBase := replacementBaseContent
		queryText := edit.OldText
		if anyFuzzy {
			queryText = normalizeForFuzzyMatch(edit.OldText)
		}

		matchRes := fuzzyFindText(targetBase, queryText)
		if !matchRes.found {
			if totalEdits == 1 {
				return "", "", fmt.Errorf("Could not find the exact text in %s. The old text must match exactly including all whitespace and newlines.", path)
			}
			return "", "", fmt.Errorf("Could not find edits[%d] in %s. The oldText must match exactly including all whitespace and newlines.", i, path)
		}

		occurrences := countOccurrences(targetBase, queryText)
		if occurrences > 1 {
			if totalEdits == 1 {
				return "", "", fmt.Errorf("Found %d occurrences of the text in %s. The text must be unique. Please provide more context to make it unique.", occurrences, path)
			}
			return "", "", fmt.Errorf("Found %d occurrences of edits[%d] in %s. Each oldText must be unique. Please provide more context to make it unique.", occurrences, i, path)
		}

		matchedEdits = append(matchedEdits, matchedEdit{
			editIndex:   i,
			matchIndex:  matchRes.index,
			matchLength: matchRes.matchLength,
			newText:     edit.NewText,
		})
	}

	// Sort matched edits by matchIndex to verify non-overlapping regions
	sort.Slice(matchedEdits, func(i, j int) bool {
		return matchedEdits[i].matchIndex < matchedEdits[j].matchIndex
	})

	for i := 1; i < len(matchedEdits); i++ {
		prev := matchedEdits[i-1]
		curr := matchedEdits[i]
		if prev.matchIndex+prev.matchLength > curr.matchIndex {
			return "", "", fmt.Errorf("edits[%d] and edits[%d] overlap in %s. Merge them into one edit or target disjoint regions.", prev.editIndex, curr.editIndex, path)
		}
	}

	var newContent string
	if anyFuzzy {
		newContent = applyReplacementsPreservingUnchangedLines(normalizedContent, replacementBaseContent, matchedEdits)
	} else {
		newContent = applyReplacements(replacementBaseContent, matchedEdits)
	}

	if normalizedContent == newContent {
		if totalEdits == 1 {
			return "", "", fmt.Errorf("No changes made to %s. The replacement produced identical content. This might indicate an issue with special characters or the text not existing as expected.", path)
		}
		return "", "", fmt.Errorf("No changes made to %s. The replacements produced identical content.", path)
	}

	return normalizedContent, newContent, nil
}

func applyReplacements(content string, replacements []matchedEdit) string {
	res := content
	// Apply in reverse order so character offsets remain stable
	for i := len(replacements) - 1; i >= 0; i-- {
		r := replacements[i]
		res = res[:r.matchIndex] + r.newText + res[r.matchIndex+r.matchLength:]
	}
	return res
}

func splitLinesWithEndings(content string) []string {
	if content == "" {
		return []string{}
	}
	var lines []string
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, content[start:i+1])
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, content[start:])
	}
	return lines
}

func getLineSpans(content string) []lineSpan {
	lines := splitLinesWithEndings(content)
	spans := make([]lineSpan, len(lines))
	offset := 0
	for i, l := range lines {
		spans[i] = lineSpan{start: offset, end: offset + len(l)}
		offset += len(l)
	}
	return spans
}

func applyReplacementsPreservingUnchangedLines(originalContent, baseContent string, replacements []matchedEdit) string {
	originalLines := splitLinesWithEndings(originalContent)
	baseLines := getLineSpans(baseContent)

	if len(originalLines) != len(baseLines) {
		// Line count mismatch fallback
		return applyReplacements(baseContent, replacements)
	}

	type group struct {
		startLine    int
		endLine      int
		replacements []matchedEdit
	}

	var groups []group
	for _, rep := range replacements {
		repStart := rep.matchIndex
		repEnd := rep.matchIndex + rep.matchLength

		startLine := -1
		for i, ls := range baseLines {
			if repStart >= ls.start && repStart < ls.end {
				startLine = i
				break
			}
		}
		if startLine == -1 {
			startLine = 0
		}

		endLine := startLine
		for endLine < len(baseLines) && baseLines[endLine].end < repEnd {
			endLine++
		}
		if endLine < len(baseLines) {
			endLine++
		}

		if len(groups) > 0 && startLine < groups[len(groups)-1].endLine {
			last := &groups[len(groups)-1]
			if endLine > last.endLine {
				last.endLine = endLine
			}
			last.replacements = append(last.replacements, rep)
		} else {
			groups = append(groups, group{
				startLine:    startLine,
				endLine:      endLine,
				replacements: []matchedEdit{rep},
			})
		}
	}

	var sb strings.Builder
	origIdx := 0

	for _, g := range groups {
		// Copy untouched lines from original
		for l := origIdx; l < g.startLine; l++ {
			sb.WriteString(originalLines[l])
		}

		groupStartOffset := baseLines[g.startLine].start
		groupEndOffset := baseLines[g.endLine-1].end
		groupBaseSlice := baseContent[groupStartOffset:groupEndOffset]

		// Adjust replacement offsets relative to group base slice
		var relReplacements []matchedEdit
		for _, r := range g.replacements {
			relReplacements = append(relReplacements, matchedEdit{
				editIndex:   r.editIndex,
				matchIndex:  r.matchIndex - groupStartOffset,
				matchLength: r.matchLength,
				newText:     r.newText,
			})
		}

		appliedSlice := applyReplacements(groupBaseSlice, relReplacements)
		sb.WriteString(appliedSlice)

		origIdx = g.endLine
	}

	// Copy remaining untouched lines from original
	for l := origIdx; l < len(originalLines); l++ {
		sb.WriteString(originalLines[l])
	}

	return sb.String()
}

func fuzzyFindText(content, oldText string) fuzzyMatchResult {
	// Try exact match first
	exactIdx := strings.Index(content, oldText)
	if exactIdx != -1 {
		return fuzzyMatchResult{
			found:          true,
			index:          exactIdx,
			matchLength:    len(oldText),
			usedFuzzyMatch: false,
		}
	}

	// Try fuzzy matching in normalized space
	fuzzyContent := normalizeForFuzzyMatch(content)
	fuzzyOldText := normalizeForFuzzyMatch(oldText)
	fuzzyIdx := strings.Index(fuzzyContent, fuzzyOldText)

	if fuzzyIdx == -1 {
		return fuzzyMatchResult{
			found:          false,
			index:          -1,
			matchLength:    0,
			usedFuzzyMatch: false,
		}
	}

	return fuzzyMatchResult{
		found:          true,
		index:          fuzzyIdx,
		matchLength:    len(fuzzyOldText),
		usedFuzzyMatch: true,
	}
}

func countOccurrences(content, oldText string) int {
	return strings.Count(content, oldText)
}

var (
	smartSingleQuotesRegex = regexp.MustCompile(`[\x{2018}\x{2019}\x{201A}\x{201B}]`)
	smartDoubleQuotesRegex = regexp.MustCompile(`[\x{201C}\x{201D}\x{201E}\x{201F}]`)
	unicodeDashesRegex     = regexp.MustCompile(`[\x{2010}\x{2011}\x{2012}\x{2013}\x{2014}\x{2015}\x{2212}]`)
	unicodeSpacesRegex     = regexp.MustCompile(`[\x{00A0}\x{2002}-\x{200A}\x{202F}\x{205F}\x{3000}]`)
)

// NormalizeForFuzzyMatch applies NFKC normalization, strips per-line trailing whitespace,
// and normalizes smart quotes, unicode dashes, and special spaces.
func NormalizeForFuzzyMatch(text string) string {
	return normalizeForFuzzyMatch(text)
}

func normalizeForFuzzyMatch(text string) string {
	// 1. NFKC Unicode normalization
	normalized := norm.NFKC.String(text)

	// 2. Strip trailing whitespace per line
	lines := strings.Split(normalized, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}
	normalized = strings.Join(lines, "\n")

	// 3. Smart single quotes → '
	normalized = smartSingleQuotesRegex.ReplaceAllString(normalized, "'")

	// 4. Smart double quotes → "
	normalized = smartDoubleQuotesRegex.ReplaceAllString(normalized, "\"")

	// 5. Unicode dashes → -
	normalized = unicodeDashesRegex.ReplaceAllString(normalized, "-")

	// 6. Special spaces → regular space
	normalized = unicodeSpacesRegex.ReplaceAllString(normalized, " ")

	return normalized
}

func detectLineEnding(content string) string {
	crlfIdx := strings.Index(content, "\r\n")
	lfIdx := strings.Index(content, "\n")
	if lfIdx == -1 {
		return "\n"
	}
	if crlfIdx == -1 {
		return "\n"
	}
	if crlfIdx < lfIdx {
		return "\r\n"
	}
	return "\n"
}

func normalizeToLF(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}

func restoreLineEndings(text, ending string) string {
	if ending == "\r\n" {
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	return text
}

func stripBOM(content string) (bool, string) {
	if strings.HasPrefix(content, "\xef\xbb\xbf") {
		return true, strings.TrimPrefix(content, "\xef\xbb\xbf")
	}
	if strings.HasPrefix(content, "\uFEFF") {
		return true, strings.TrimPrefix(content, "\uFEFF")
	}
	return false, content
}
