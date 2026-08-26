package tools

import (
	"fmt"
	"strconv"
	"strings"
)

// DiffOp represents the operation type of a diff chunk.
type DiffOp int

const (
	DiffEqual DiffOp = iota
	DiffInsert
	DiffDelete
)

// DiffLine represents a single line in a line-by-line diff.
type DiffLine struct {
	Op   DiffOp
	Text string
}

// DiffLines computes a line-level diff between oldContent and newContent using LCS.
func DiffLines(oldContent, newContent string) []DiffLine {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	n := len(oldLines)
	m := len(newLines)

	// DP table for Longest Common Subsequence
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to construct diff
	var result []DiffLine
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			result = append(result, DiffLine{Op: DiffEqual, Text: oldLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			result = append(result, DiffLine{Op: DiffInsert, Text: newLines[j-1]})
			j--
		} else if i > 0 && (j == 0 || dp[i][j-1] < dp[i-1][j]) {
			result = append(result, DiffLine{Op: DiffDelete, Text: oldLines[i-1]})
			i--
		}
	}

	// Reverse since we backtracked from end to start
	for l, r := 0, len(result)-1; l < r; l, r = l+1, r-1 {
		result[l], result[r] = result[r], result[l]
	}

	return result
}

// GenerateUnifiedPatch generates a standard unified diff patch (--- / +++ / @@ -l,s +l,s @@).
func GenerateUnifiedPatch(path, oldContent, newContent string, contextLines int) string {
	if contextLines <= 0 {
		contextLines = 4
	}

	diff := DiffLines(oldContent, newContent)
	if len(diff) == 0 {
		return ""
	}

	// Check if there are any changes
	hasChanges := false
	for _, d := range diff {
		if d.Op != DiffEqual {
			hasChanges = true
			break
		}
	}
	if !hasChanges {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- %s\n", path))
	sb.WriteString(fmt.Sprintf("+++ %s\n", path))

	// Group into hunks
	type hunk struct {
		oldStart int
		oldCount int
		newStart int
		newCount int
		lines    []DiffLine
	}

	var hunks []hunk
	var currentHunk *hunk

	oldLineNum := 1
	newLineNum := 1

	for idx := 0; idx < len(diff); idx++ {
		d := diff[idx]

		if d.Op != DiffEqual {
			if currentHunk == nil {
				// Start new hunk, including leading context
				startIdx := idx - contextLines
				if startIdx < 0 {
					startIdx = 0
				}

				h := hunk{
					oldStart: oldLineNum - (idx - startIdx),
					newStart: newLineNum - (idx - startIdx),
				}
				if h.oldStart < 1 {
					h.oldStart = 1
				}
				if h.newStart < 1 {
					h.newStart = 1
				}

				for c := startIdx; c < idx; c++ {
					h.lines = append(h.lines, diff[c])
					h.oldCount++
					h.newCount++
				}
				currentHunk = &h
			}

			currentHunk.lines = append(currentHunk.lines, d)
			if d.Op == DiffDelete {
				currentHunk.oldCount++
				oldLineNum++
			} else {
				currentHunk.newCount++
				newLineNum++
			}
		} else {
			if currentHunk != nil {
				// Look ahead to see if another change comes within 2*contextLines
				hasNearChange := false
				for look := idx; look < len(diff) && look <= idx+contextLines*2; look++ {
					if diff[look].Op != DiffEqual {
						hasNearChange = true
						break
					}
				}

				if hasNearChange {
					currentHunk.lines = append(currentHunk.lines, d)
					currentHunk.oldCount++
					currentHunk.newCount++
					oldLineNum++
					newLineNum++
				} else {
					// Add trailing context lines
					trailingEnd := idx + contextLines
					if trailingEnd > len(diff) {
						trailingEnd = len(diff)
					}
					for c := idx; c < trailingEnd; c++ {
						if diff[c].Op != DiffEqual {
							break
						}
						currentHunk.lines = append(currentHunk.lines, diff[c])
						currentHunk.oldCount++
						currentHunk.newCount++
						oldLineNum++
						newLineNum++
						idx = c
					}
					hunks = append(hunks, *currentHunk)
					currentHunk = nil
				}
			} else {
				oldLineNum++
				newLineNum++
			}
		}
	}

	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	for _, h := range hunks {
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", h.oldStart, h.oldCount, h.newStart, h.newCount))
		for _, l := range h.lines {
			switch l.Op {
			case DiffEqual:
				sb.WriteString(" " + l.Text + "\n")
			case DiffInsert:
				sb.WriteString("+" + l.Text + "\n")
			case DiffDelete:
				sb.WriteString("-" + l.Text + "\n")
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// GenerateDisplayDiff generates a clean, line-numbered diff view suitable for interactive TUI / CLI inspection.
func GenerateDisplayDiff(oldContent, newContent string, contextLines int) (diffText string, firstChangedLine int) {
	if contextLines <= 0 {
		contextLines = 4
	}

	diff := DiffLines(oldContent, newContent)
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)
	maxLineNum := len(oldLines)
	if len(newLines) > maxLineNum {
		maxLineNum = len(newLines)
	}
	numWidth := len(strconv.Itoa(maxLineNum))
	if numWidth < 1 {
		numWidth = 1
	}

	oldLineNum := 1
	newLineNum := 1
	firstChangedLine = 0
	lastWasChange := false

	var output []string

	for i := 0; i < len(diff); i++ {
		d := diff[i]

		if d.Op != DiffEqual {
			if firstChangedLine == 0 {
				firstChangedLine = newLineNum
			}

			if d.Op == DiffInsert {
				lineNumStr := fmt.Sprintf("%*d", numWidth, newLineNum)
				output = append(output, fmt.Sprintf("+%s %s", lineNumStr, d.Text))
				newLineNum++
			} else {
				lineNumStr := fmt.Sprintf("%*d", numWidth, oldLineNum)
				output = append(output, fmt.Sprintf("-%s %s", lineNumStr, d.Text))
				oldLineNum++
			}
			lastWasChange = true
		} else {
			// Context line handling
			nextPartIsChange := false
			for j := i + 1; j < len(diff) && j <= i+contextLines*2; j++ {
				if diff[j].Op != DiffEqual {
					nextPartIsChange = true
					break
				}
			}

			hasLeadingChange := lastWasChange
			hasTrailingChange := nextPartIsChange

			if hasLeadingChange || hasTrailingChange {
				lineNumStr := fmt.Sprintf("%*d", numWidth, oldLineNum)
				output = append(output, fmt.Sprintf(" %s %s", lineNumStr, d.Text))
				oldLineNum++
				newLineNum++
			} else {
				// Check if we need to insert an ellipsis indicator
				if len(output) > 0 && !strings.HasPrefix(output[len(output)-1], " "+strings.Repeat(" ", numWidth)+" ...") {
					output = append(output, fmt.Sprintf(" %s ...", strings.Repeat(" ", numWidth)))
				}
				oldLineNum++
				newLineNum++
			}

			lastWasChange = false
		}
	}

	return strings.Join(output, "\n"), firstChangedLine
}

func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
