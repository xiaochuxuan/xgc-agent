// example_test.go: Tests for prompt template rendering.
package prompt

import "testing"

func TestRenderSystemPrompt_DefaultZH(t *testing.T) {
	got, err := RenderSystemPrompt("", SystemPromptOptions{Locale: "zh", AppName: "xgc-agent"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got == "" {
		t.Fatalf("empty prompt")
	}
}
