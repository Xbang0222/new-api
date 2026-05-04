package service

// custom: claude code only — Claude Code 客户端识别单元测试

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

func newGinCtxForDetector(t *testing.T, method, path, body string, headers map[string]string) *gin.Context {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	var reqBody *bytes.Reader
	if body != "" {
		reqBody = bytes.NewReader([]byte(body))
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	ctx.Request = req
	return ctx
}

func TestIsClaudeCodeRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		headers map[string]string
		want    bool
	}{
		{
			name: "UA hit claude-cli lowercase",
			headers: map[string]string{
				"User-Agent": "claude-cli/2.1.123 (external, cli)",
			},
			want: true,
		},
		{
			name: "UA hit Claude-Code mixed case",
			headers: map[string]string{
				"User-Agent": "Claude-Code/2.0.5 (Node.js/v22)",
			},
			want: true,
		},
		{
			name: "UA hit claude-CODE uppercase variant",
			headers: map[string]string{
				"User-Agent": "claude-CODE/3.0.0",
			},
			want: true,
		},
		{
			name: "Session header hit even with curl UA",
			headers: map[string]string{
				"User-Agent":              "curl/8.6.0",
				"X-Claude-Code-Session-Id": "01HZK5...uuid",
			},
			want: true,
		},
		{
			name: "System prompt string hit",
			body: `{"model":"claude-3-5","system":"You are Claude Code, Anthropic's official CLI for Claude. You assist with software engineering tasks."}`,
			headers: map[string]string{
				"User-Agent": "python-httpx/0.27.0",
			},
			want: true,
		},
		{
			name: "System prompt array-of-blocks hit",
			body: `{"model":"claude-3-5","system":[{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude. ..."},{"type":"text","text":"<env>...</env>"}]}`,
			headers: map[string]string{
				"User-Agent": "python-httpx/0.27.0",
			},
			want: true,
		},
		{
			name: "System prompt with leading whitespace hit",
			body: `{"system":"  \n You are Claude Code, Anthropic's official CLI for Claude. tail"}`,
			want: true,
		},
		{
			name: "All miss — cherry studio + unrelated system",
			body: `{"model":"claude-3-5","system":"You are a helpful assistant."}`,
			headers: map[string]string{
				"User-Agent": "cherry-studio/1.0",
			},
			want: false,
		},
		{
			name: "All miss — empty body, generic UA",
			headers: map[string]string{
				"User-Agent": "openai-python/1.0",
			},
			want: false,
		},
		{
			name: "All miss — UA mentions claude but not claude-cli/code prefix",
			headers: map[string]string{
				"User-Agent": "MyClaudeProxy/1.0",
			},
			want: false,
		},
		{
			name: "Malformed JSON body does not panic — returns false safely",
			body: `{"system":"You are Claude Code, ...`, // truncated, malformed
			want: false,                                 // 不能解析时安全返回 false（关键是不 panic）
		},
		{
			name: "Random non-JSON body does not panic",
			body: `not a json body at all`,
			want: false,
		},
		{
			name:    "Empty everything",
			body:    "",
			headers: map[string]string{},
			want:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := newGinCtxForDetector(t, http.MethodPost, "/v1/messages", tt.body, tt.headers)
			got := IsClaudeCodeRequest(ctx)
			if got != tt.want {
				t.Errorf("IsClaudeCodeRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsClaudeCodeRequest_NilContext(t *testing.T) {
	t.Parallel()
	if IsClaudeCodeRequest(nil) {
		t.Error("IsClaudeCodeRequest(nil) should return false")
	}
}

func TestIsClaudeCodeAnthropicPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"exact /v1/messages", "/v1/messages", true},
		{"sub-path /v1/messages/count_tokens", "/v1/messages/count_tokens", true},
		{"sub-path with trailing slash", "/v1/messages/", true},
		{"openai chat completions", "/v1/chat/completions", false},
		{"openai completions", "/v1/completions", false},
		{"openai responses", "/v1/responses", false},
		{"gemini path", "/v1beta/models/gemini-pro:generateContent", false},
		{"realtime ws", "/v1/realtime", false},
		{"prefix-similar but different segment", "/v1/messages_foo", false},
		{"root path", "/", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := newGinCtxForDetector(t, http.MethodPost, tt.path, "", nil)
			if got := IsClaudeCodeAnthropicPath(ctx); got != tt.want {
				t.Errorf("IsClaudeCodeAnthropicPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsClaudeCodeAnthropicPath_NilContext(t *testing.T) {
	t.Parallel()
	if IsClaudeCodeAnthropicPath(nil) {
		t.Error("IsClaudeCodeAnthropicPath(nil) should return false")
	}
}

// 验证检测器调用之后，body 仍可被后续中间件正常读取（不破坏 body 复用）。
func TestIsClaudeCodeRequest_BodyReusableAfterDetection(t *testing.T) {
	t.Parallel()
	body := `{"model":"claude-3-5","system":"You are Claude Code, Anthropic's official CLI for Claude. ..."}`
	ctx := newGinCtxForDetector(t, http.MethodPost, "/v1/messages", body, nil)

	if !IsClaudeCodeRequest(ctx) {
		t.Fatal("expected detection to hit")
	}

	storage, err := common.GetBodyStorage(ctx)
	if err != nil {
		t.Fatalf("GetBodyStorage after detection: %v", err)
	}
	got, err := storage.Bytes()
	if err != nil {
		t.Fatalf("Bytes after detection: %v", err)
	}
	if string(got) != body {
		t.Errorf("body content mismatch after detection.\nwant: %s\ngot:  %s", body, string(got))
	}
}
