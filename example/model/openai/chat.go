package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"xgc-agent/message"
	"xgc-agent/model/openai"
)

func main() {
	_ = godotenv.Load("example/model/openai/.env")

	apiKey := strings.TrimSpace(os.Getenv("XGC_API_KEY"))
	if apiKey == "" {
		fmt.Println("missing XGC_API_KEY")
		return
	}

	// 读取模型名称并去除首尾空白
	modelName := strings.TrimSpace(os.Getenv("XGC_MODEL"))
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}

	baseURL := strings.TrimSpace(os.Getenv("XGC_BASE_URL"))
	proxyURL := strings.TrimSpace(os.Getenv("XGC_HTTP_PROXY"))

	client := http.DefaultClient
	if proxyURL != "" {
		parsedProxy, err := url.Parse(proxyURL)
		if err != nil {
			fmt.Printf("invalid XGC_HTTP_PROXY: %v\n", err)
			return
		}
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(parsedProxy),
			},
		}
	}

	opts := []openai.Option{
		openai.WithAPIKey(apiKey),
		openai.WithHTTPClient(client),
		openai.WithModel(modelName),
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}

	chatModel, err := openai.NewOpenAIChatModel(opts...)
	if err != nil {
		fmt.Printf("init model error: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	request := &message.Request{
		Messages: []message.Message{
			message.MessageSystem("你是一个简洁的中文助手。"),
			message.MessageUser("用一句话介绍 Go 协程的优势。"),
		},
	}

	// Non-streaming example.
	resp, err := chatModel.Generater(ctx, request)
	if err != nil {
		fmt.Printf("non-stream error: %v\n", err)
		return
	}
	if resp != nil {
		fmt.Println("Non-stream:")
		if len(resp.ResponseChoices) > 0 {
			fmt.Println(resp.ResponseChoices[0].Message.Content)
		}
	}

	// Streaming example.
	streamCtx, streamCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer streamCancel()

	responses, errs := chatModel.Stream(streamCtx, request)
	fmt.Println("Stream:")
	var streamed strings.Builder
	for r := range responses {
		if r == nil {
			continue
		}

		// Print partial deltas as they arrive.
		if r.IsPartial {
			if len(r.ResponseChoices) > 0 {
				delta := r.ResponseChoices[0].Message.Content
				if delta != "" {
					streamed.WriteString(delta)
					fmt.Print(delta)
				}
			}
			continue
		}

		// Final response: avoid duplicate printing if we already printed partials.
		if r.Done {
			if streamed.Len() == 0 {
				if len(r.ResponseChoices) > 0 {
					fmt.Print(r.ResponseChoices[0].Message.Content)
				}
			}
			fmt.Println()
		}
	}
	if err, ok := <-errs; ok && err != nil {
		fmt.Printf("stream error: %v\n", err)
	}
}
