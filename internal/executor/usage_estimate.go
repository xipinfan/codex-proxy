package executor

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"codex-proxy/internal/translator"

	"github.com/tidwall/gjson"
)

func EstimateUsageWithFallback(usage translator.ResponseUsage, outputText string, estimatedPromptTokens int64, model string) translator.ResponseUsage {
	if usage.TotalTokens <= 0 && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	if usage.OutputTokens <= 0 {
		estimatedOutput := estimateTokensFromOutputText(outputText, model)
		if estimatedOutput > 0 {
			usage.OutputTokens = estimatedOutput
			usage.FoundUsage = true
		}
	}
	if usage.OutputTokens > 0 && usage.InputTokens <= 0 && estimatedPromptTokens > 0 {
		usage.InputTokens = estimatedPromptTokens
		usage.FoundUsage = true
	}
	if usage.TotalTokens <= 0 && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		usage.FoundUsage = true
	}
	return usage
}

func EstimatePromptTokensFromRequest(requestBody []byte, model string) int64 {
	return estimatePromptTokensFromRequest(requestBody, model)
}

func estimatePromptTokensFromRequest(requestBody []byte, model string) int64 {
	root := gjson.ParseBytes(requestBody)
	var sb strings.Builder
	collectPromptText(root, "", 0, &sb)
	return estimateTokensFromOutputText(sb.String(), model)
}

func collectPromptText(value gjson.Result, key string, depth int, sb *strings.Builder) {
	if depth > 24 {
		return
	}
	switch value.Type {
	case gjson.String:
		s := strings.TrimSpace(value.String())
		if !shouldIncludePromptString(s, key) {
			return
		}
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(s)
	case gjson.JSON:
		if value.IsArray() {
			value.ForEach(func(_, item gjson.Result) bool {
				collectPromptText(item, key, depth+1, sb)
				return true
			})
			return
		}
		value.ForEach(func(k, item gjson.Result) bool {
			childKey := strings.ToLower(strings.TrimSpace(k.String()))
			if shouldSkipPromptKey(childKey) {
				return true
			}
			collectPromptText(item, childKey, depth+1, sb)
			return true
		})
	}
}

func shouldSkipPromptKey(key string) bool {
	switch key {
	case "model", "id", "type", "created", "timestamp", "user", "voice", "format",
		"audio", "image_url", "url", "b64_json", "mime_type", "file_id", "service_tier",
		"parallel_tool_calls", "tool_choice", "stream", "stream_options", "temperature",
		"top_p", "max_tokens", "max_output_tokens", "max_completion_tokens":
		return true
	default:
		return false
	}
}

func shouldIncludePromptString(s, key string) bool {
	if s == "" {
		return false
	}
	// Skip obvious non-text payloads to avoid wildly inflated estimates.
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "data:") {
		return false
	}
	if len(s) > 512 && strings.Count(s, " ") <= 2 && isLikelyBase64Blob(s) {
		return false
	}

	switch key {
	case "instructions", "input", "prompt", "content", "text", "input_text",
		"output_text", "arguments", "description", "summary", "message":
		return true
	}
	return strings.Contains(key, "text") || strings.Contains(key, "content") || strings.Contains(key, "prompt")
}

func isLikelyBase64Blob(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) {
			return false
		}
		if !(r == '+' || r == '/' || r == '=' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}
	return true
}

func estimateTokensFromOutputText(text, model string) int64 {
	_ = model
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	var asciiChars, cjkChars, otherChars int
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		switch {
		case isCJKRune(r):
			cjkChars++
		case r <= unicode.MaxASCII:
			asciiChars++
		default:
			otherChars++
		}
	}

	tokens := int64(cjkChars)
	tokens += int64((asciiChars + 3) / 4)
	tokens += int64((otherChars + 1) / 2)
	if tokens <= 0 && utf8.RuneCountInString(text) > 0 {
		return 1
	}
	return tokens
}

func isCJKRune(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana)
}
