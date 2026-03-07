/**
 * 思考后缀解析模块
 * 使用连字符格式从模型名中提取思考配置
 * 格式：model-name-level，例如 gpt-5-xhigh、gpt-5-16384
 *
 * 解析逻辑：
 *   1. 先检查完整模型名是否为已知模型，如果是则不解析后缀
 *   2. 再从最后一个连字符分割，检查前半部分是否为已知模型 + 尾部是否为有效后缀
 *   3. 有效的思考级别：minimal, low, medium, high, xhigh, max, none, auto
 *   4. 有效的数字：正整数（作为 token 预算）、0（禁用）、-1（自动）
 */
package thinking

import (
	"strconv"
	"strings"
)

/**
 * validThinkingSuffixes 存储所有有效的思考级别后缀
 * 用于快速判断最后一段是否为思考配置
 */
var validThinkingSuffixes = map[string]bool{
	"minimal": true,
	"low":     true,
	"medium":  true,
	"high":    true,
	"xhigh":   true,
	"max":     true,
	"none":    true,
	"auto":    true,
}

/**
 * knownBaseModels 已知的基础模型名集合
 * 用于防止将模型名的一部分（如 gpt-5 中的 5）误判为思考后缀
 * 解析时：只有去掉尾部后剩余部分在此集合中，才认为尾部是思考后缀
 */
var knownBaseModels = map[string]bool{
	/* GPT-5 系列 */
	"gpt-5": true, "gpt-5-codex": true, "gpt-5-codex-mini": true,
	/* GPT-5.1 系列 */
	"gpt-5.1": true, "gpt-5.1-codex": true, "gpt-5.1-codex-mini": true, "gpt-5.1-codex-max": true,
	/* GPT-5.2 系列 */
	"gpt-5.2": true, "gpt-5.2-codex": true,
	/* GPT-5.3 系列 */
	"gpt-5.3-codex": true, "gpt-5.3-codex-spark": true,
	/* GPT-5.4 */
	"gpt-5.4": true,
	/* 旧版兼容 */
	"codex-mini": true,
}

/**
 * ParseModelSuffix 从模型名中解析思考后缀（连字符格式）
 * 使用已知模型名列表防止误判（如 gpt-5 不会被解析为 gpt + budget=5）
 *
 * 解析流程：
 *   1. 完整模型名是已知模型 → 无后缀
 *   2. 去掉最后一段后是已知模型 + 最后一段是有效后缀 → 有后缀
 *   3. 其他情况 → 无后缀
 *
 * 示例：
 *   - "gpt-5" → ModelName="gpt-5", HasSuffix=false（已知模型，不拆分）
 *   - "gpt-5-xhigh" → ModelName="gpt-5", RawSuffix="xhigh"
 *   - "gpt-5-16384" → ModelName="gpt-5", RawSuffix="16384"
 *   - "gpt-5-codex-high" → ModelName="gpt-5-codex", RawSuffix="high"
 *
 * @param model - 原始模型名
 * @returns ParseResult - 解析结果
 */
func ParseModelSuffix(model string) ParseResult {
	model = strings.TrimSpace(model)
	if model == "" {
		return ParseResult{ModelName: model, HasSuffix: false}
	}

	/* 1. 完整模型名就是已知模型，不做后缀解析 */
	if knownBaseModels[strings.ToLower(model)] {
		return ParseResult{ModelName: model, HasSuffix: false}
	}

	/* 2. 找到最后一个连字符的位置 */
	lastDash := strings.LastIndex(model, "-")
	if lastDash <= 0 || lastDash >= len(model)-1 {
		return ParseResult{ModelName: model, HasSuffix: false}
	}

	suffix := strings.ToLower(model[lastDash+1:])
	modelName := model[:lastDash]

	/* 3. 前半部分必须是已知模型，才认为后半部分是思考后缀 */
	if !knownBaseModels[strings.ToLower(modelName)] {
		return ParseResult{ModelName: model, HasSuffix: false}
	}

	/* 4. 检查是否为有效的思考级别 */
	if validThinkingSuffixes[suffix] {
		return ParseResult{
			ModelName: modelName,
			HasSuffix: true,
			RawSuffix: suffix,
		}
	}

	/* 5. 检查是否为数字（token 预算） */
	if _, err := strconv.Atoi(suffix); err == nil {
		return ParseResult{
			ModelName: modelName,
			HasSuffix: true,
			RawSuffix: suffix,
		}
	}

	/* 不是有效的思考后缀，原样返回 */
	return ParseResult{ModelName: model, HasSuffix: false}
}

/**
 * RegisterBaseModel 动态注册已知基础模型名
 * 允许在运行时添加新模型，无需修改代码
 * @param modelName - 模型名（不区分大小写）
 */
func RegisterBaseModel(modelName string) {
	knownBaseModels[strings.ToLower(modelName)] = true
}

/**
 * ParseSuffixToConfig 将原始后缀字符串转换为 ThinkingConfig
 *
 * 解析优先级：
 *   1. 特殊值：none → ModeNone, auto/-1 → ModeAuto
 *   2. 级别名：minimal/low/medium/high/xhigh/max → ModeLevel
 *   3. 数字值：正整数 → ModeBudget, 0 → ModeNone
 *
 * @param rawSuffix - 原始后缀字符串
 * @returns ThinkingConfig - 解析后的思考配置
 */
func ParseSuffixToConfig(rawSuffix string) ThinkingConfig {
	rawSuffix = strings.TrimSpace(strings.ToLower(rawSuffix))
	if rawSuffix == "" {
		return ThinkingConfig{}
	}

	/* 1. 特殊值 */
	switch rawSuffix {
	case "none":
		return ThinkingConfig{Mode: ModeNone, Budget: 0}
	case "auto", "-1":
		return ThinkingConfig{Mode: ModeAuto, Budget: -1}
	}

	/* 2. 级别名 */
	switch rawSuffix {
	case "minimal":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelMinimal}
	case "low":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelLow}
	case "medium":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelMedium}
	case "high":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelHigh}
	case "xhigh":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelXHigh}
	case "max":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelMax}
	}

	/* 3. 数字值 */
	if value, err := strconv.Atoi(rawSuffix); err == nil {
		if value == 0 {
			return ThinkingConfig{Mode: ModeNone, Budget: 0}
		}
		if value > 0 {
			return ThinkingConfig{Mode: ModeBudget, Budget: value}
		}
	}

	return ThinkingConfig{}
}
