/**
 * 思考配置应用模块
 * 将解析后的思考配置应用到 Codex 请求体中
 * Codex 使用 reasoning.effort 字段设置思考级别
 */
package thinking

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

/**
 * levelToBudgetMap 级别到预算的映射表
 */
var levelToBudgetMap = map[string]int{
	"none":    0,
	"auto":    -1,
	"minimal": 512,
	"low":     1024,
	"medium":  8192,
	"high":    24576,
	"xhigh":   32768,
	"max":     128000,
}

/**
 * ApplyThinking 将思考配置和服务层级应用到请求体
 * 解析模型名中的思考后缀和 -fast 后缀，写入请求 JSON
 *
 * 支持的后缀格式：
 *   - gpt-5.4-high → reasoning.effort = "high"
 *   - gpt-5.4-fast → service_tier = "fast"
 *   - gpt-5.4-high-fast → reasoning.effort = "high" + service_tier = "fast"
 *
 * @param body - 原始请求体 JSON
 * @param model - 模型名（可能包含思考后缀和/或 -fast 后缀）
 * @returns []byte - 处理后的请求体 JSON
 * @returns string - 去除所有后缀后的真实模型名
 */
func ApplyThinking(body []byte, model string) ([]byte, string) {
	parsed := ParseModelSuffix(model)
	baseModel := parsed.ModelName

	var config ThinkingConfig
	if parsed.HasSuffix {
		config = ParseSuffixToConfig(parsed.RawSuffix)
	} else {
		/* 没有后缀时，尝试从请求体中提取 */
		config = extractConfigFromBody(body)
	}

	/* 应用思考配置到请求体 */
	if hasThinkingConfig(config) {
		body = applyCodexThinking(body, config)
	}

	/* 应用 fast 模式：设置 service_tier */
	if parsed.IsFast {
		body, _ = sjson.SetBytes(body, "service_tier", parsed.ServiceTier)
	}

	return body, baseModel
}

/**
 * extractConfigFromBody 从请求体中提取思考配置
 * 支持多种格式：
 *   - OpenAI: reasoning_effort
 *   - Codex: reasoning.effort
 *
 * @param body - 请求体 JSON
 * @returns ThinkingConfig - 提取的思考配置
 */
func extractConfigFromBody(body []byte) ThinkingConfig {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ThinkingConfig{}
	}

	/* 检查 Codex 格式 reasoning.effort */
	if effort := gjson.GetBytes(body, "reasoning.effort"); effort.Exists() {
		value := strings.ToLower(strings.TrimSpace(effort.String()))
		if value == "none" {
			return ThinkingConfig{Mode: ModeNone, Budget: 0}
		}
		if value != "" {
			return ThinkingConfig{Mode: ModeLevel, Level: ThinkingLevel(value)}
		}
	}

	/* 检查 OpenAI 格式 reasoning_effort */
	if effort := gjson.GetBytes(body, "reasoning_effort"); effort.Exists() {
		value := strings.ToLower(strings.TrimSpace(effort.String()))
		if value == "none" {
			return ThinkingConfig{Mode: ModeNone, Budget: 0}
		}
		if value != "" {
			return ThinkingConfig{Mode: ModeLevel, Level: ThinkingLevel(value)}
		}
	}

	return ThinkingConfig{}
}

/**
 * applyCodexThinking 将思考配置写入 Codex 请求体
 * 设置 reasoning.effort 字段
 *
 * @param body - 请求体 JSON
 * @param config - 思考配置
 * @returns []byte - 修改后的请求体
 */
func applyCodexThinking(body []byte, config ThinkingConfig) []byte {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		body = []byte(`{}`)
	}

	var effort string
	switch config.Mode {
	case ModeLevel:
		if config.Level == "" {
			return body
		}
		effort = string(config.Level)
	case ModeNone:
		effort = "none"
	case ModeAuto:
		effort = "medium"
	case ModeBudget:
		/* 将预算转换为最近的级别 */
		effort = budgetToLevel(config.Budget)
		if effort == "" {
			return body
		}
	default:
		return body
	}

	result, _ := sjson.SetBytes(body, "reasoning.effort", effort)
	return result
}

/**
 * budgetToLevel 将数字预算转换为最近的思考级别
 * @param budget - token 预算值
 * @returns string - 对应的思考级别
 */
func budgetToLevel(budget int) string {
	switch {
	case budget <= 0:
		return "none"
	case budget <= 512:
		return "minimal"
	case budget <= 1024:
		return "low"
	case budget <= 8192:
		return "medium"
	case budget <= 24576:
		return "high"
	default:
		return "xhigh"
	}
}

/**
 * LevelToBudget 将思考级别转换为预算值
 * @param level - 思考级别字符串
 * @returns int - 预算值
 * @returns bool - 是否为有效级别
 */
func LevelToBudget(level string) (int, bool) {
	budget, ok := levelToBudgetMap[strings.ToLower(level)]
	return budget, ok
}

/**
 * hasThinkingConfig 检查是否包含有效的思考配置
 * @param config - 思考配置
 * @returns bool - 是否有配置
 */
func hasThinkingConfig(config ThinkingConfig) bool {
	return config.Mode != ModeBudget || config.Budget != 0 || config.Level != ""
}

/**
 * StripThinkingConfig 从请求体中移除思考配置字段
 * @param body - 请求体 JSON
 * @returns []byte - 移除后的请求体
 */
func StripThinkingConfig(body []byte) []byte {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body
	}

	result := body
	result, _ = sjson.DeleteBytes(result, "reasoning.effort")
	result, _ = sjson.DeleteBytes(result, "reasoning_effort")
	return result
}
