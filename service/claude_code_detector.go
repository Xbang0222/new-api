package service

// custom: claude code only — Claude Code CLI 客户端识别
//
// 设计要点：
//  1. 多维 fingerprint，命中任一即视为 Claude Code（用户选择"命中 1 项即可"，
//     保证 Claude Code 升级时不会因单一字段变化而误杀整个 Claude Code 用户群）。
//  2. fingerprint 来源（公开可验证）：
//     - User-Agent: claude-cli/X.Y.Z 或 Claude-Code/X.Y.Z（官方 npm 包 @anthropic-ai/claude-code）
//     - X-Claude-Code-Session-Id 请求头：v2.1.86+ 引入，专门用于代理识别
//     - 请求体 system 字段：以 "You are Claude Code, Anthropic's official CLI for Claude" 开头
//       （来源：system_prompts_leaks 仓库 v2.1.120 dump）
//  3. body 解析复用 common.GetBodyStorage（与 channel_affinity 同一模式），不破坏后续中间件读取。
//  4. 任何解析异常（body 损坏、读取失败、非 JSON）都返回 false，不 panic。

import (
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	claudeCodeSessionHeader = "X-Claude-Code-Session-Id"
	claudeCodeSystemMarker  = "You are Claude Code, Anthropic's official CLI for Claude"
)

// claudeCodeUserAgentPattern 匹配 Claude Code CLI 的 User-Agent 前缀。
// 已观测格式：claude-cli/2.1.123 (external, cli)、Claude-Code/2.0.5 (Node.js/v22)
var claudeCodeUserAgentPattern = regexp.MustCompile(`(?i)claude-(cli|code)/`)

// IsClaudeCodeRequest 判断当前请求是否来自 Claude Code CLI 客户端。
// 命中以下任一 fingerprint 即返回 true：
//   - User-Agent 含 claude-cli/ 或 claude-code/（不区分大小写）
//   - 请求头存在 X-Claude-Code-Session-Id（任意非空值）
//   - 请求体顶层 system 字段（兼容 string 与 array-of-blocks 两种格式）
//     以 "You are Claude Code, Anthropic's official CLI for Claude" 开头
//
// 任何解析异常一律返回 false。
func IsClaudeCodeRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}

	if ua := c.Request.Header.Get("User-Agent"); ua != "" && claudeCodeUserAgentPattern.MatchString(ua) {
		return true
	}

	if c.Request.Header.Get(claudeCodeSessionHeader) != "" {
		return true
	}

	return matchClaudeCodeSystemPrompt(c)
}

// matchClaudeCodeSystemPrompt 检查请求体 system 字段是否以 Claude Code 标识开头。
// 兼容两种 Anthropic Messages API 格式：
//
//	{"system": "You are Claude Code, ..."}
//	{"system": [{"type": "text", "text": "You are Claude Code, ..."}, ...]}
func matchClaudeCodeSystemPrompt(c *gin.Context) bool {
	storage, err := common.GetBodyStorage(c)
	if err != nil || storage == nil {
		return false
	}
	body, err := storage.Bytes()
	if err != nil || len(body) == 0 {
		return false
	}

	res := gjson.GetBytes(body, "system")
	if !res.Exists() {
		return false
	}

	switch res.Type {
	case gjson.String:
		return hasClaudeCodeMarker(res.String())
	case gjson.JSON:
		if !res.IsArray() {
			return false
		}
		for _, item := range res.Array() {
			text := item.Get("text").String()
			if text == "" {
				text = item.Get("value").String()
			}
			if hasClaudeCodeMarker(text) {
				return true
			}
		}
	}
	return false
}

func hasClaudeCodeMarker(s string) bool {
	return strings.HasPrefix(strings.TrimLeft(s, " \t\n\r"), claudeCodeSystemMarker)
}
