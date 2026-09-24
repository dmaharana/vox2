package llm_test

import (
	"testing"

	"go-harness/pkg/llm"
)

func TestParseCronCommand(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedName string
		expectedSched string
		expectedIntent string
		expectError  bool
	}{
		{
			name:           "Standard with quotes and dashes",
			input:          `"*/10 * * * *" -- /check-stock-price AMD`,
			expectedName:   "/check-stock-price AMD",
			expectedSched:  "*/10 * * * *",
			expectedIntent: "/check-stock-price AMD",
			expectError:    false,
		},
		{
			name:           "With name prefix",
			input:          `name:stock "*/10 * * * *" -- /check-stock-price AMD`,
			expectedName:   "stock",
			expectedSched:  "*/10 * * * *",
			expectedIntent: "/check-stock-price AMD",
			expectError:    false,
		},
		{
			name:           "With quotes without dashes",
			input:          `"*/10 * * * *" /check-stock-price AMD`,
			expectedName:   "/check-stock-price AMD",
			expectedSched:  "*/10 * * * *",
			expectedIntent: "/check-stock-price AMD",
			expectError:    false,
		},
		{
			name:           "Descriptor @every with dashes",
			input:          `@every 10m -- /check-stock-price AMD`,
			expectedName:   "/check-stock-price AMD",
			expectedSched:  "@every 10m",
			expectedIntent: "/check-stock-price AMD",
			expectError:    false,
		},
		{
			name:           "Descriptor @hourly with dashes",
			input:          `@hourly -- /check-stock-price AMD`,
			expectedName:   "/check-stock-price AMD",
			expectedSched:  "@hourly",
			expectedIntent: "/check-stock-price AMD",
			expectError:    false,
		},
		{
			name:        "Empty input",
			input:       "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, sched, intent, err := llm.ParseCronCommand(tc.input)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tc.expectedName {
				t.Errorf("name mismatch: expected %q, got %q", tc.expectedName, name)
			}
			if sched != tc.expectedSched {
				t.Errorf("schedule mismatch: expected %q, got %q", tc.expectedSched, sched)
			}
			if intent != tc.expectedIntent {
				t.Errorf("intent mismatch: expected %q, got %q", tc.expectedIntent, intent)
			}
		})
	}
}
