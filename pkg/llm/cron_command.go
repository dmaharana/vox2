package llm

import (
	"fmt"
	"strings"
)

// ParseCronCommand parses the argument string of a /cron creation command.
// Supported syntaxes:
//   - "/10 * * * *" -- /check-stock-price AMD
//   - name:stock "*/10 * * * *" -- /check-stock-price AMD
//   - "*/10 * * * *" /check-stock-price AMD
//   - @every 10m -- /check-stock-price AMD
//   - @hourly -- /check-stock-price AMD
func ParseCronCommand(input string) (name, schedule, intent string, err error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", "", "", fmt.Errorf("empty cron command")
	}

	// 1. Check for optional name:<alias> prefix
	if strings.HasPrefix(strings.ToLower(trimmed), "name:") {
		after := strings.TrimPrefix(trimmed, "name:")
		parts := strings.SplitN(after, " ", 2)
		name = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			trimmed = strings.TrimSpace(parts[1])
		} else {
			trimmed = ""
		}
	}

	if trimmed == "" {
		return "", "", "", fmt.Errorf("missing schedule and intent")
	}

	// 2. Parse schedule and intent
	if strings.Contains(trimmed, " -- ") {
		parts := strings.SplitN(trimmed, " -- ", 2)
		schedule = strings.TrimSpace(parts[0])
		intent = strings.TrimSpace(parts[1])
		schedule = strings.Trim(schedule, `"'`)
	} else if strings.HasPrefix(trimmed, `"`) || strings.HasPrefix(trimmed, `'`) {
		quote := trimmed[0]
		endIdx := strings.Index(trimmed[1:], string(quote))
		if endIdx == -1 {
			return "", "", "", fmt.Errorf("unclosed quote in cron expression")
		}
		schedule = trimmed[1 : endIdx+1]
		after := strings.TrimSpace(trimmed[endIdx+2:])
		after = strings.TrimPrefix(after, "--")
		intent = strings.TrimSpace(after)
	} else if strings.HasPrefix(trimmed, "@every ") {
		fields := strings.Fields(trimmed)
		if len(fields) < 3 {
			return "", "", "", fmt.Errorf("invalid @every expression: expected format '@every <duration> <intent>'")
		}
		schedule = fields[0] + " " + fields[1]
		intent = strings.TrimSpace(strings.TrimPrefix(trimmed, schedule))
	} else if strings.HasPrefix(trimmed, "@") {
		fields := strings.Fields(trimmed)
		schedule = fields[0]
		if len(fields) > 1 {
			intent = strings.TrimSpace(strings.TrimPrefix(trimmed, schedule))
		}
	} else {
		return "", "", "", fmt.Errorf("invalid /cron syntax: expected '<schedule>' -- <intent> or '<schedule>' <intent>")
	}

	schedule = strings.TrimSpace(schedule)
	intent = strings.TrimSpace(intent)

	if schedule == "" {
		return "", "", "", fmt.Errorf("cron schedule expression is required")
	}
	if intent == "" {
		return "", "", "", fmt.Errorf("cron intent is required")
	}
	if name == "" {
		name = intent
	}

	return name, schedule, intent, nil
}
