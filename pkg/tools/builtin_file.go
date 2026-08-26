package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sashabaranov/go-openai/jsonschema"
)

// RegisterBuiltinTools registers the core file and script execution tools into the registry.
func RegisterBuiltinTools(r *Registry, rootDir string) {
	if rootDir == "" {
		rootDir = "."
	}

	r.Register(NewReadFileTool(rootDir))
	r.Register(NewWriteFileTool(rootDir))
	r.Register(NewEditTool(rootDir))
	r.Register(NewEditFileTool(rootDir))
	r.Register(NewUpdateFileTool(rootDir))
	r.Register(NewListDirectoryTool(rootDir))
	r.Register(NewExecuteScriptTool(rootDir))
}

// -------------------------------------------------------------
// Tool: read_file
// -------------------------------------------------------------

type ReadFileInput struct {
	Path      string `json:"path" jsonschema:"description=The relative or absolute file path to read"`
	StartLine int    `json:"start_line,omitempty" jsonschema:"description=Optional 1-based start line number to read"`
	EndLine   int    `json:"end_line,omitempty" jsonschema:"description=Optional 1-based end line number to read"`
}

type ReadFileOutput struct {
	Path       string `json:"path"`
	TotalLines int    `json:"total_lines"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Content    string `json:"content"`
}

func NewReadFileTool(baseDir string) ToolDefinition {
	return ToolDefinition{
		Name:        "read_file",
		Description: "Reads the text contents of a file (markdown, text, json, yaml, source code) with optional line range slice.",
		Category:    "builtin",
		Enabled:     true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {
					Type:        jsonschema.String,
					Description: "Path to the file to read",
				},
				"start_line": {
					Type:        jsonschema.Integer,
					Description: "Optional starting line number (1-based, inclusive)",
				},
				"end_line": {
					Type:        jsonschema.Integer,
					Description: "Optional ending line number (1-based, inclusive)",
				},
			},
			Required: []string{"path"},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in ReadFileInput
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("invalid arguments for read_file: %w", err)
			}

			targetPath := resolvePath(baseDir, in.Path)
			file, err := os.Open(targetPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open file '%s': %w", in.Path, err)
			}
			defer file.Close()

			var lines []string
			scanner := bufio.NewScanner(file)
			// Increase buffer size for large lines
			buf := make([]byte, 1024*1024)
			scanner.Buffer(buf, 10*1024*1024)

			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("error reading file '%s': %w", in.Path, err)
			}

			total := len(lines)
			start := 1
			end := total

			if in.StartLine > 0 {
				start = in.StartLine
				if start > total {
					start = total
				}
			}
			if in.EndLine > 0 {
				end = in.EndLine
				if end > total {
					end = total
				}
			}
			if start > end {
				start = end
			}

			var selected []string
			if total > 0 && start <= total {
				selected = lines[start-1 : end]
			}

			return ReadFileOutput{
				Path:       in.Path,
				TotalLines: total,
				StartLine:  start,
				EndLine:    end,
				Content:    strings.Join(selected, "\n"),
			}, nil
		},
	}
}

// -------------------------------------------------------------
// Tool: write_file
// -------------------------------------------------------------

type WriteFileInput struct {
	Path    string `json:"path" jsonschema:"description=The file path to create or overwrite"`
	Content string `json:"content" jsonschema:"description=The text content to write into the file"`
}

type WriteFileOutput struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
	Success      bool   `json:"success"`
}

func NewWriteFileTool(baseDir string) ToolDefinition {
	return ToolDefinition{
		Name:        "write_file",
		Description: "Creates or overwrites a file with the given content, automatically creating any missing parent directories.",
		Category:    "builtin",
		Enabled:     true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {
					Type:        jsonschema.String,
					Description: "Path to the file to write",
				},
				"content": {
					Type:        jsonschema.String,
					Description: "The full file content to write",
				},
			},
			Required: []string{"path", "content"},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in WriteFileInput
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("invalid arguments for write_file: %w", err)
			}

			targetPath := resolvePath(baseDir, in.Path)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create directories for '%s': %w", in.Path, err)
			}

			if err := os.WriteFile(targetPath, []byte(in.Content), 0644); err != nil {
				return nil, fmt.Errorf("failed to write file '%s': %w", in.Path, err)
			}

			return WriteFileOutput{
				Path:         in.Path,
				BytesWritten: len(in.Content),
				Success:      true,
			}, nil
		},
	}
}

// -------------------------------------------------------------
// Tool: update_file
// -------------------------------------------------------------

type UpdateFileInput struct {
	Path               string `json:"path" jsonschema:"description=The file path to update"`
	TargetContent      string `json:"target_content" jsonschema:"description=The exact block of text to replace"`
	ReplacementContent string `json:"replacement_content" jsonschema:"description=The replacement text"`
}

type UpdateFileOutput struct {
	Path       string `json:"path"`
	Replaced   int    `json:"replaced_count"`
	Success    bool   `json:"success"`
}

func NewUpdateFileTool(baseDir string) ToolDefinition {
	return ToolDefinition{
		Name:        "update_file",
		Description: "Updates an existing file by locating a specific target block of text and replacing it with replacement text.",
		Category:    "builtin",
		Enabled:     true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {
					Type:        jsonschema.String,
					Description: "Path to the file to modify",
				},
				"target_content": {
					Type:        jsonschema.String,
					Description: "The exact substring/block of code to be replaced",
				},
				"replacement_content": {
					Type:        jsonschema.String,
					Description: "The replacement code block",
				},
			},
			Required: []string{"path", "target_content", "replacement_content"},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in UpdateFileInput
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("invalid arguments for update_file: %w", err)
			}

			targetPath := resolvePath(baseDir, in.Path)
			data, err := os.ReadFile(targetPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file '%s': %w", in.Path, err)
			}

			content := string(data)
			if !strings.Contains(content, in.TargetContent) {
				return nil, errors.New("target_content not found in file")
			}

			count := strings.Count(content, in.TargetContent)
			newContent := strings.Replace(content, in.TargetContent, in.ReplacementContent, 1)

			if err := os.WriteFile(targetPath, []byte(newContent), 0644); err != nil {
				return nil, fmt.Errorf("failed to save updated file '%s': %w", in.Path, err)
			}

			return UpdateFileOutput{
				Path:     in.Path,
				Replaced: count,
				Success:  true,
			}, nil
		},
	}
}

// -------------------------------------------------------------
// Tool: list_directory
// -------------------------------------------------------------

type ListDirectoryInput struct {
	Path      string `json:"path,omitempty" jsonschema:"description=The directory path to explore (default .)"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"description=Whether to recursively traverse subdirectories"`
	MaxDepth  int    `json:"max_depth,omitempty" jsonschema:"description=Maximum recursive depth (default 5)"`
}

type FileItem struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size_bytes"`
	Children int    `json:"children_count,omitempty"`
}

type ListDirectoryOutput struct {
	RootPath string     `json:"root_path"`
	Items    []FileItem `json:"items"`
}

func NewListDirectoryTool(baseDir string) ToolDefinition {
	return ToolDefinition{
		Name:        "list_directory",
		Description: "Recursively or flatly lists files and subdirectories within a folder with size and metadata.",
		Category:    "builtin",
		Enabled:     true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {
					Type:        jsonschema.String,
					Description: "Directory path to explore (defaults to root)",
				},
				"recursive": {
					Type:        jsonschema.Boolean,
					Description: "Set to true to explore recursively",
				},
				"max_depth": {
					Type:        jsonschema.Integer,
					Description: "Maximum recursive depth (default 5)",
				},
			},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in ListDirectoryInput
			if len(args) > 0 && string(args) != "{}" && string(args) != "null" {
				_ = json.Unmarshal(args, &in)
			}
			if in.Path == "" {
				in.Path = "."
			}
			if in.MaxDepth <= 0 {
				in.MaxDepth = 5
			}

			targetPath := resolvePath(baseDir, in.Path)
			var items []FileItem

			if !in.Recursive {
				entries, err := os.ReadDir(targetPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read directory '%s': %w", in.Path, err)
				}
				for _, e := range entries {
					info, _ := e.Info()
					var size int64
					if info != nil {
						size = info.Size()
					}
					items = append(items, FileItem{
						Name:  e.Name(),
						Path:  filepath.Join(in.Path, e.Name()),
						IsDir: e.IsDir(),
						Size:  size,
					})
				}
			} else {
				startDepth := strings.Count(filepath.Clean(targetPath), string(filepath.Separator))
				err := filepath.WalkDir(targetPath, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					rel, _ := filepath.Rel(targetPath, path)
					if rel == "." {
						return nil
					}

					currentDepth := strings.Count(filepath.Clean(path), string(filepath.Separator)) - startDepth
					if currentDepth > in.MaxDepth {
						if d.IsDir() {
							return fs.SkipDir
						}
						return nil
					}

					// Skip .git and node_modules
					if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
						return fs.SkipDir
					}

					info, _ := d.Info()
					var size int64
					if info != nil {
						size = info.Size()
					}

					items = append(items, FileItem{
						Name:  d.Name(),
						Path:  filepath.Join(in.Path, rel),
						IsDir: d.IsDir(),
						Size:  size,
					})
					return nil
				})
				if err != nil {
					return nil, fmt.Errorf("failed to list directory '%s': %w", in.Path, err)
				}
			}

			return ListDirectoryOutput{
				RootPath: in.Path,
				Items:    items,
			}, nil
		},
	}
}

func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}
