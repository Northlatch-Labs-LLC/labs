// Labs - the Northlatch Labs agent harness
// License: MIT

package providers

import "encoding/json"

// NormalizeToolCall fills the redundant halves of a tool call (Name/Function,
// Arguments/Function.Arguments) so every consumer sees one consistent shape.
func NormalizeToolCall(tc ToolCall) ToolCall {
	normalized := tc

	if normalized.ThoughtSignature == "" &&
		normalized.ExtraContent != nil &&
		normalized.ExtraContent.Google != nil {
		normalized.ThoughtSignature = normalized.ExtraContent.Google.ThoughtSignature
	}

	// Ensure Name is populated from Function if not set
	if normalized.Name == "" && normalized.Function != nil {
		normalized.Name = normalized.Function.Name
	}

	// Ensure Arguments is not nil
	if normalized.Arguments == nil {
		normalized.Arguments = map[string]any{}
	}

	// Parse Arguments from Function.Arguments if not already set
	if len(normalized.Arguments) == 0 && normalized.Function != nil && normalized.Function.Arguments != "" {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(normalized.Function.Arguments), &parsed); err == nil && parsed != nil {
			normalized.Arguments = parsed
		}
	}

	// Ensure Function is populated with consistent values
	argsJSON, _ := json.Marshal(normalized.Arguments)
	if normalized.Function == nil {
		normalized.Function = &FunctionCall{
			Name:             normalized.Name,
			Arguments:        string(argsJSON),
			ThoughtSignature: normalized.ThoughtSignature,
		}
	} else {
		if normalized.Function.Name == "" {
			normalized.Function.Name = normalized.Name
		}
		if normalized.Name == "" {
			normalized.Name = normalized.Function.Name
		}
		if normalized.Function.Arguments == "" {
			normalized.Function.Arguments = string(argsJSON)
		}
		if normalized.Function.ThoughtSignature == "" {
			normalized.Function.ThoughtSignature = normalized.ThoughtSignature
		}
		if normalized.ThoughtSignature == "" {
			normalized.ThoughtSignature = normalized.Function.ThoughtSignature
		}
	}

	return normalized
}
