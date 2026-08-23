package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai/jsonschema"
)

// ExecuteScriptInput defines the parameters for execute_script tool.
type ExecuteScriptInput struct {
	Language       string   `json:"language"`
	Code           string   `json:"code"`
	Args           []string `json:"args,omitempty"`
	WorkingDir     string   `json:"working_dir,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}

// ExecuteScriptOutput defines the result structure for execute_script tool.
type ExecuteScriptOutput struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
	Language   string `json:"language"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

// NewExecuteScriptTool creates a tool for executing Python, JavaScript, PowerShell, Batch, and Bash scripts locally.
func NewExecuteScriptTool(baseDir string) ToolDefinition {
	return ToolDefinition{
		Name:        "execute_script",
		Description: "Executes a generated script locally (Python, JavaScript/Node.js, PowerShell, Windows Batch/CMD, Bash/Shell) and returns stdout, stderr, and exit code. Supports cross-platform execution on Windows, Linux, and macOS.",
		Category:    "builtin",
		Enabled:     true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"language": {
					Type:        jsonschema.String,
					Description: "Script language or interpreter to use: 'python', 'javascript', 'powershell', 'bat', 'bash'",
					Enum:        []string{"python", "python3", "javascript", "node", "powershell", "pwsh", "bat", "cmd", "bash", "sh"},
				},
				"code": {
					Type:        jsonschema.String,
					Description: "The complete script or code content to execute",
				},
				"args": {
					Type: jsonschema.Array,
					Items: &jsonschema.Definition{
						Type: jsonschema.String,
					},
					Description: "Optional command line arguments to pass to the script",
				},
				"working_dir": {
					Type:        jsonschema.String,
					Description: "Optional working directory for script execution (defaults to project root)",
				},
				"timeout_seconds": {
					Type:        jsonschema.Integer,
					Description: "Maximum runtime in seconds (default: 30, maximum: 120)",
				},
			},
			Required: []string{"language", "code"},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in ExecuteScriptInput
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("invalid arguments for execute_script: %w", err)
			}

			if strings.TrimSpace(in.Code) == "" {
				return nil, fmt.Errorf("code parameter cannot be empty")
			}

			// Normalize language and choose runtime
			lang := strings.ToLower(strings.TrimSpace(in.Language))
			var ext string
			var binary string
			var runnerArgs []string

			switch lang {
			case "python", "python3", "py":
				ext = ".py"
				binary = "python3"
				if _, err := exec.LookPath("python3"); err != nil {
					if _, err := exec.LookPath("python"); err == nil {
						binary = "python"
					} else if _, err := exec.LookPath("py"); err == nil {
						binary = "py"
					}
				}
			case "javascript", "js", "node":
				ext = ".js"
				binary = "node"
			case "powershell", "pwsh", "ps1":
				ext = ".ps1"
				binary = "powershell"
				if _, err := exec.LookPath("powershell"); err != nil {
					if _, err := exec.LookPath("pwsh"); err == nil {
						binary = "pwsh"
					}
				}
				runnerArgs = []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File"}
			case "bat", "cmd", "batch":
				ext = ".bat"
				binary = "cmd"
				runnerArgs = []string{"/c"}
			case "bash", "sh", "shell":
				ext = ".sh"
				binary = "bash"
				if _, err := exec.LookPath("bash"); err != nil {
					binary = "sh"
				}
			default:
				return nil, fmt.Errorf("unsupported script language '%s'. Supported: python, javascript, powershell, bat, bash", in.Language)
			}

			// Verify runner binary exists
			binPath, err := exec.LookPath(binary)
			if err != nil {
				return nil, fmt.Errorf("runtime interpreter '%s' is not installed or not in PATH", binary)
			}

			// Timeout setup
			timeout := 30 * time.Second
			if in.TimeoutSeconds > 0 {
				if in.TimeoutSeconds > 120 {
					in.TimeoutSeconds = 120
				}
				timeout = time.Duration(in.TimeoutSeconds) * time.Second
			}

			execCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Create a temporary script file
			tmpFile, err := os.CreateTemp("", fmt.Sprintf("vox2_script_*%s", ext))
			if err != nil {
				return nil, fmt.Errorf("failed to create temporary script file: %w", err)
			}
			tmpPath := tmpFile.Name()
			defer os.Remove(tmpPath)

			if _, err := tmpFile.WriteString(in.Code); err != nil {
				tmpFile.Close()
				return nil, fmt.Errorf("failed to write script to temp file: %w", err)
			}
			_ = tmpFile.Close()

			// Make shell scripts executable
			if ext == ".sh" {
				_ = os.Chmod(tmpPath, 0755)
			}

			// Assemble command
			cmdArgs := append(runnerArgs, tmpPath)
			cmdArgs = append(cmdArgs, in.Args...)
			cmd := exec.CommandContext(execCtx, binPath, cmdArgs...)

			// Set working directory
			workDir := baseDir
			if in.WorkingDir != "" {
				workDir = resolvePath(baseDir, in.WorkingDir)
			}
			cmd.Dir = workDir

			// Capture stdout and stderr
			var stdoutBuf, stderrBuf bytes.Buffer
			cmd.Stdout = &stdoutBuf
			cmd.Stderr = &stderrBuf

			startTime := time.Now()
			cmdErr := cmd.Run()
			duration := time.Since(startTime)

			exitCode := 0
			var errMsg string
			if cmdErr != nil {
				if exitErr, ok := cmdErr.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					exitCode = -1
				}
				if execCtx.Err() == context.DeadlineExceeded {
					errMsg = fmt.Sprintf("script timed out after %v", timeout)
				} else {
					errMsg = cmdErr.Error()
				}
			}

			return ExecuteScriptOutput{
				Stdout:     stdoutBuf.String(),
				Stderr:     stderrBuf.String(),
				ExitCode:   exitCode,
				DurationMS: duration.Milliseconds(),
				Language:   lang,
				Success:    cmdErr == nil,
				Error:      errMsg,
			}, nil
		},
	}
}
