// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Common utilities shared across providers

package provider

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sony-level/readme-runner/internal/llm"
)

// TruncateForError truncates a string for error messages
func TruncateForError(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ExtractPlanFromLLMContent extracts and parses a RunPlan from LLM response content
func ExtractPlanFromLLMContent(content string) (*llm.RunPlan, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	if start < 0 {
		return nil, fmt.Errorf("%w: no JSON object found", llm.ErrInvalidJSON)
	}

	depth := 0
	end := -1
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end > 0 {
			break
		}
	}

	if end <= start {
		return nil, fmt.Errorf("%w: malformed JSON", llm.ErrInvalidJSON)
	}

	jsonStr := content[start:end]

	var plan llm.RunPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("%w: %v", llm.ErrInvalidJSON, err)
	}

	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("plan validation failed: %w", err)
	}

	return &plan, nil
}
