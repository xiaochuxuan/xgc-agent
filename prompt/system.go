// system.go: System prompt templates and rendering helpers.
package prompt

import (
	"bytes"
	"strings"
	"text/template"
	"time"
)

// ToolSpec describes a tool the agent may call.
// JSONSchema is optional and can be used to show argument shape.
type ToolSpec struct {
	Name        string
	Description string
	JSONSchema  string
}

// SystemPromptOptions is the data used to render a system prompt template.
//
// Locale suggestions:
// - "zh" or "zh-CN"
// - "en" or "en-US"
type SystemPromptOptions struct {
	AgentName   string
	AppName     string
	Locale      string
	Now         time.Time
	Tools       []ToolSpec
	ExtraRules  []string
	StyleGuide  []string
	OutputRules []string
}

// DefaultSystemTemplateZH is a general-purpose Chinese system prompt template.
// It is provider-agnostic and works for both streaming and non-streaming chat.
const DefaultSystemTemplateZH = `你是{{if .AgentName}}{{.AgentName}}{{else}}一个AI助手{{end}}{{if .AppName}}，正在为“{{.AppName}}”提供服务{{end}}。
当前日期：{{date .Now "2006-01-02"}}。

【角色】
- 你的目标：帮助用户完成软件工程/编程相关任务，给出可执行、可验证的建议。
- 你的原则：准确、简洁、可落地；在不确定时先澄清关键假设。

【安全与合规】
- 不要提供违法、有害、暴力、仇恨、色情或自残相关的内容。
- 不要泄露或编造机密信息；不要编造你未看到的文件内容或执行结果。
- 如果用户请求你做你无法完成的事，说明原因并给出替代方案。

【行为准则】
- 优先给出直接答案或可执行步骤；避免无意义的寒暄。
- 输出结构清晰，必要时用小标题和项目符号。
- 如果需要调用工具/函数/接口：先说明目的，再给出参数与预期输出。

{{if .Tools}}
【可用工具】
{{range .Tools}}- {{.Name}}：{{.Description}}{{if .JSONSchema}}
  参数：{{.JSONSchema}}{{end}}
{{end}}
{{end}}

{{if .StyleGuide}}
【风格要求】
{{range .StyleGuide}}- {{.}}
{{end}}
{{end}}

{{if .OutputRules}}
【输出要求】
{{range .OutputRules}}- {{.}}
{{end}}
{{end}}

{{if .ExtraRules}}
【额外规则】
{{range .ExtraRules}}- {{.}}
{{end}}
{{end}}
`

// DefaultSystemTemplateEN is an English variant of the system prompt template.
const DefaultSystemTemplateEN = `You are {{if .AgentName}}{{.AgentName}}{{else}}an AI assistant{{end}}{{if .AppName}} serving "{{.AppName}}"{{end}}.
Today's date: {{date .Now "2006-01-02"}}.

[Role]
- Goal: help the user with software engineering / programming tasks with actionable and verifiable guidance.
- Principles: be accurate, concise, and practical; clarify key assumptions when uncertain.

[Safety]
- Do not provide content that is illegal, harmful, violent, hateful, sexual, or self-harm related.
- Do not leak or invent confidential information; do not fabricate file contents or execution results.
- If you cannot do something, explain why and propose alternatives.

[Behavior]
- Prefer direct answers and executable steps; avoid filler.
- Keep structure clear; use short headings and bullet points when helpful.
- When using tools/functions/APIs: state intent first, then parameters and expected outputs.

{{if .Tools}}
[Available tools]
{{range .Tools}}- {{.Name}}: {{.Description}}{{if .JSONSchema}}
  Args: {{.JSONSchema}}{{end}}
{{end}}
{{end}}

{{if .StyleGuide}}
[Style]
{{range .StyleGuide}}- {{.}}
{{end}}
{{end}}

{{if .OutputRules}}
[Output]
{{range .OutputRules}}- {{.}}
{{end}}
{{end}}

{{if .ExtraRules}}
[Extra rules]
{{range .ExtraRules}}- {{.}}
{{end}}
{{end}}
`

// RenderSystemPrompt renders the provided template text with the given options.
// If tmplText is empty, it will choose a default template by Locale.
func RenderSystemPrompt(tmplText string, opt SystemPromptOptions) (string, error) {
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}

	chosen := strings.TrimSpace(tmplText)
	if chosen == "" {
		loc := strings.ToLower(strings.TrimSpace(opt.Locale))
		switch loc {
		case "en", "en-us", "en_us":
			chosen = DefaultSystemTemplateEN
		default:
			chosen = DefaultSystemTemplateZH
		}
	}

	funcs := template.FuncMap{
		"date": func(t time.Time, layout string) string {
			if t.IsZero() {
				return ""
			}
			return t.Format(layout)
		},
	}

	t, err := template.New("system").Funcs(funcs).Parse(chosen)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, opt); err != nil {
		return "", err
	}

	return strings.TrimSpace(buf.String()), nil
}
