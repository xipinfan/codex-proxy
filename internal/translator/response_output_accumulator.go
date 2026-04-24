package translator

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// ResponseOutputTextAccumulator 累积 SSE 中可用于 usage 估算的输出文本，并尽量避免 delta/done 重复计入。
type ResponseOutputTextAccumulator struct {
	sb                       strings.Builder
	reasoningSummaryHasDelta bool
	reasoningDeltaByKey      map[string]string
	outputTextDeltaByKey     map[string]string
	functionArgsDeltaByKey   map[string]string
	functionArgsClosed       map[string]bool
}

func NewResponseOutputTextAccumulator() *ResponseOutputTextAccumulator {
	return &ResponseOutputTextAccumulator{
		reasoningDeltaByKey:    make(map[string]string),
		outputTextDeltaByKey:   make(map[string]string),
		functionArgsDeltaByKey: make(map[string]string),
		functionArgsClosed:     make(map[string]bool),
	}
}

func (a *ResponseOutputTextAccumulator) Text() string {
	if a == nil {
		return ""
	}
	return a.sb.String()
}

func (a *ResponseOutputTextAccumulator) AddSSELine(rawLine []byte) {
	if a == nil {
		return
	}
	line := bytes.TrimSpace(rawLine)
	if !bytes.HasPrefix(line, dataPrefix) {
		return
	}
	rawJSON := bytes.TrimSpace(line[5:])
	if len(rawJSON) == 0 {
		return
	}

	root := gjson.ParseBytes(rawJSON)
	switch root.Get("type").String() {
	case "response.output_text.delta":
		key := sseEventKey(root, "output_text")
		delta := root.Get("delta").String()
		if delta == "" {
			return
		}
		a.outputTextDeltaByKey[key] += delta
		a.appendFragment(delta)
	case "response.output_text.done":
		key := sseEventKey(root, "output_text")
		tail := diffTail(root.Get("text").String(), a.outputTextDeltaByKey[key])
		delete(a.outputTextDeltaByKey, key)
		a.appendFragment(tail)
	case "response.reasoning_summary_text.delta":
		delta := root.Get("delta").String()
		if delta == "" {
			return
		}
		a.reasoningSummaryHasDelta = true
		a.appendFragment(delta)
	case "response.reasoning_summary_text.done":
		if a.reasoningSummaryHasDelta {
			return
		}
		a.appendFragment(root.Get("text").String())
	case "response.reasoning.delta", "response.reasoning_text.delta":
		key := sseEventKey(root, "reasoning")
		delta := root.Get("delta").String()
		if delta == "" {
			return
		}
		a.reasoningDeltaByKey[key] += delta
		a.appendFragment(delta)
	case "response.reasoning_text.done":
		key := sseEventKey(root, "reasoning")
		tail := diffTail(root.Get("text").String(), a.reasoningDeltaByKey[key])
		delete(a.reasoningDeltaByKey, key)
		a.appendFragment(tail)
	case "response.function_call_arguments.delta":
		key := sseEventKey(root, "function_args")
		delta := root.Get("delta").String()
		if delta == "" {
			return
		}
		delete(a.functionArgsClosed, key)
		a.functionArgsDeltaByKey[key] += delta
		a.appendFragment(delta)
	case "response.function_call_arguments.done":
		key := sseEventKey(root, "function_args")
		if a.functionArgsClosed[key] {
			return
		}
		tail := diffTail(root.Get("arguments").String(), a.functionArgsDeltaByKey[key])
		delete(a.functionArgsDeltaByKey, key)
		a.functionArgsClosed[key] = true
		a.appendFragment(tail)
	case "response.output_item.done":
		item := root.Get("item")
		if item.Get("type").String() != "function_call" {
			return
		}
		key := sseEventKey(item, "function_args")
		if a.functionArgsClosed[key] {
			return
		}
		tail := diffTail(item.Get("arguments").String(), a.functionArgsDeltaByKey[key])
		delete(a.functionArgsDeltaByKey, key)
		a.functionArgsClosed[key] = true
		a.appendFragment(tail)
	case "response.content_part.added":
		part := root.Get("part")
		if part.Get("type").String() != "reasoning_text" {
			return
		}
		a.appendFragment(part.Get("text").String())
	}
}

func ExtractResponseOutputTextFromSSEPayload(data []byte) string {
	acc := NewResponseOutputTextAccumulator()
	for _, line := range bytes.Split(data, []byte("\n")) {
		acc.AddSSELine(line)
	}
	return acc.Text()
}

func (a *ResponseOutputTextAccumulator) appendFragment(s string) {
	if s == "" {
		return
	}
	if a.sb.Len() > 0 {
		a.sb.WriteByte('\n')
	}
	a.sb.WriteString(s)
}

func diffTail(full, accumulated string) string {
	if full == "" {
		return ""
	}
	if accumulated == "" {
		return full
	}
	if full == accumulated {
		return ""
	}
	if strings.HasPrefix(full, accumulated) {
		return full[len(accumulated):]
	}
	return full
}

func sseEventKey(root gjson.Result, namespace string) string {
	if callID := root.Get("call_id").String(); callID != "" {
		return namespace + ":call:" + callID
	}
	if callID := root.Get("item.call_id").String(); callID != "" {
		return namespace + ":call:" + callID
	}
	if itemID := root.Get("item_id").String(); itemID != "" {
		return namespace + ":item:" + itemID
	}
	if outputIndex := root.Get("output_index"); outputIndex.Exists() {
		return fmt.Sprintf("%s:output:%d", namespace, outputIndex.Int())
	}
	return namespace + ":default"
}
