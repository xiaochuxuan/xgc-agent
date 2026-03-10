package main

import (
	_ "xgc-agent/model/mock"
	_ "xgc-agent/model/openai"
)

// func main() {
// 	cfg := model.Config{
// 		Provider: os.Getenv("XGC_PROVIDER"),
// 		BaseURL:  os.Getenv("XGC_BASE_URL"),
// 		APIKey:   os.Getenv("XGC_API_KEY"),
// 		Model:    os.Getenv("XGC_MODEL"),
// 		Timeout:  60 * time.Second,
// 	}

// 	m, err := model.NewChatModel(cfg)
// 	if err != nil {
// 		panic(err)
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// 	defer cancel()

// 	sessionID := strings.TrimSpace(os.Getenv("XGC_SESSION"))
// 	if sessionID == "" {
// 		sessionID = "default"
// 	}

// 	historyLimit := 20
// 	if v := strings.TrimSpace(os.Getenv("XGC_HISTORY_LIMIT")); v != "" {
// 		if n, err := strconv.Atoi(v); err == nil {
// 			historyLimit = n
// 		}
// 	}

// 	maxStored := 200
// 	if v := strings.TrimSpace(os.Getenv("XGC_MEMORY_MAX")); v != "" {
// 		if n, err := strconv.Atoi(v); err == nil {
// 			maxStored = n
// 		}
// 	}

// 	store, err := memory.NewStore(memory.Config{
// 		Provider:    os.Getenv("XGC_MEMORY"),
// 		Dir:         os.Getenv("XGC_MEMORY_DIR"),
// 		DBPath:      os.Getenv("XGC_MEMORY_DB"),
// 		MaxMessages: maxStored,
// 	})
// 	if err != nil {
// 		panic(err)
// 	}
// 	if c, ok := store.(interface{ Close() error }); ok {
// 		defer func() { _ = c.Close() }()
// 	}

// 	tools := []prompt.ToolSpec{
// 		{Name: "search", Description: "Search the workspace for relevant code"},
// 		{Name: "run", Description: "Run a command or unit tests"},
// 	}

// 	sysOpt := prompt.SystemPromptOptions{
// 		AgentName:  "xgc-agent",
// 		AppName:    "xgc-agent",
// 		Locale:     "zh",
// 		Now:        time.Now(),
// 		Tools:      tools,
// 		ExtraRules: []string{"默认用中文回复。", "优先给出可执行步骤，再解释原因。"},
// 	}

// 	memCtx, memCancel := context.WithTimeout(context.Background(), 3*time.Second)
// 	history, err := store.List(memCtx, sessionID, historyLimit)
// 	memCancel()
// 	if err != nil {
// 		panic(err)
// 	}

// 	sysMsg, err := message.SystemFromTemplate("", sysOpt)
// 	if err != nil {
// 		panic(err)
// 	}

// 	userMsg := model.Messages{Role: "user", Content: "你好，给我一句话介绍 Go 的优点。"}
// 	conv := message.NewConversation().Add(sysMsg)
// 	for _, msg := range history {
// 		conv.Add(msg)
// 	}
// 	conv.Add(userMsg)

// 	req := &model.ChatRequest{
// 		Messages: conv.Build(),
// 	}

// 	events, errs := m.Stream(ctx, req)
// 	var assistant strings.Builder
// 	for ev := range events {
// 		if ev.Delta != "" {
// 			fmt.Print(ev.Delta)
// 			assistant.WriteString(ev.Delta)
// 		}
// 		if ev.Done {
// 			fmt.Println()
// 		}
// 	}

// 	if err, ok := <-errs; ok && err != nil {
// 		panic(err)
// 	}

// 	memCtx, memCancel = context.WithTimeout(context.Background(), 3*time.Second)
// 	_ = store.Add(memCtx, sessionID, userMsg, model.Messages{Role: "assistant", Content: assistant.String()})
// 	memCancel()
// }
