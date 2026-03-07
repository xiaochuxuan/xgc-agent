package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"time"
	"xgc-agent/message"
	"xgc-agent/model"
	"xgc-agent/tools"
	"xgc-agent/utils"

	openai "github.com/openai/openai-go/v3"
	openaiopt "github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type OpenAIChatModel struct {
	client  *openai.Client
	model   string
	baseURL string
	apikey  string

	channelBufferSize int

	options model.BaseOptions

	tools []tools.Tool
}

// NewOpenAIChatModel creates a new OpenAI chat model with the given options.
func NewOpenAIChatModel(opts ...Option) (*OpenAIChatModel, error) {
	options := &options{}
	for _, o := range opts {
		o(options)
	}

	var clientOptions []openaiopt.RequestOption

	if options.APIKey == "" {
		return nil, model.ErrMissingAPIKey
	}

	clientOptions = append(clientOptions, openaiopt.WithAPIKey(options.APIKey))

	if options.BaseURL != "" {
		clientOptions = append(clientOptions, openaiopt.WithBaseURL(options.BaseURL))
	}

	clientOptions = append(clientOptions, openaiopt.WithHTTPClient(options.HTTPClient))
	clientOptions = append(clientOptions, options.OpenAIOptions...)

	client := openai.NewClient(clientOptions...)

	return &OpenAIChatModel{
		client:  &client,
		model:   options.Model,
		baseURL: options.BaseURL,
		apikey:  options.APIKey,
	}, nil
}

// genRequest generates a model-agnostic Request from the given role and message.
func (m *OpenAIChatModel) genRequest(
	role message.Role,
	msg string,
	opts ...model.BaseOption,
) (*message.Request, error) {
	// TODO:update it
	options := model.GetCommonOptions(func() *model.BaseOptions {
		t := m.options
		return &t
	}(), opts...)

	req := &message.Request{
		Messages: []message.Message{
			{
				Role:    role,
				Content: msg,
			},
		},
		// types translation
		GenerationConfig: message.GenerationConfig{
			MaxTokens:   options.MaxTokens,
			TopP:        utils.Float32PtrToFloat64Ptr(options.TopP),
			Stop:        options.Stop,
			Temperature: utils.Float32PtrToFloat64Ptr(options.Temperature),
		},
	}
	return req, nil
}

func imageToURLOrBase64(image *message.Image) string {
	if image.URL != "" {
		return image.URL
	}
	return "data:image/" + image.Format + ";base64," + base64.StdEncoding.EncodeToString(image.Data)
}

func fileToParams(file *message.File) openai.ChatCompletionContentPartFileFileParam {
	if file.FileID != "" {
		return openai.ChatCompletionContentPartFileFileParam{
			FileID: openai.String(file.FileID),
		}
	}
	return openai.ChatCompletionContentPartFileFileParam{
		FileData: openai.String("data:" + file.MimeType + ";base64," + base64.StdEncoding.EncodeToString(file.Data)),
		Filename: openai.String(file.Filename),
	}
}

func audioToBase64(audio *message.Audio) string {
	return "data:" + audio.Format + ";base64," + base64.StdEncoding.EncodeToString(audio.Data)
}

// convertContentPart converts a single content part to OpenAI format.
func (m *OpenAIChatModel) convertContentPart(part message.ContentPart) (*openai.ChatCompletionContentPartUnionParam, error) {
	switch part.Type {
	case message.ContentTypeText:
		if part.Text != nil {
			return &openai.ChatCompletionContentPartUnionParam{
				OfText: &openai.ChatCompletionContentPartTextParam{
					Text: *part.Text,
				},
			}, nil
		}
	case message.ContentTypeImage:
		if part.Image != nil {
			return &openai.ChatCompletionContentPartUnionParam{
				OfImageURL: &openai.ChatCompletionContentPartImageParam{
					ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
						// The URL from openai-go can be used either as a URL or as a base64-encoded string.
						URL:    imageToURLOrBase64(part.Image),
						Detail: part.Image.Detail,
					},
				},
			}, nil
		}
	case message.ContentTypeAudio:
		if part.Audio != nil {
			return &openai.ChatCompletionContentPartUnionParam{
				OfInputAudio: &openai.ChatCompletionContentPartInputAudioParam{
					InputAudio: openai.ChatCompletionContentPartInputAudioInputAudioParam{
						Data:   audioToBase64(part.Audio),
						Format: part.Audio.Format,
					},
				},
			}, nil
		}
	case message.ContentTypeVideo:
		// OpenAI API does not currently support video content parts.
		return nil, model.ErrUnsupportedProvider
	case message.ContentTypeFile:
		if part.File != nil {
			return &openai.ChatCompletionContentPartUnionParam{
				OfFile: &openai.ChatCompletionContentPartFileParam{
					File: fileToParams(part.File),
				},
			}, nil
		}
	}
	return nil, nil
}

// convertUserMessageContent converts message content to user message content union.
func (m *OpenAIChatModel) convertUserMessageContent(msg message.Message,
) openai.ChatCompletionUserMessageParamContentUnion {
	// If there are no content parts and Content is not empty, return as string.
	if len(msg.ContentParts) == 0 && msg.Content != "" {
		return openai.ChatCompletionUserMessageParamContentUnion{
			OfString: openai.String(msg.Content),
		}
	}
	var contentParts []openai.ChatCompletionContentPartUnionParam
	// Add Content as a text part if present.
	if msg.Content != "" {
		contentParts = append(
			contentParts,
			openai.ChatCompletionContentPartUnionParam{
				OfText: &openai.ChatCompletionContentPartTextParam{
					Text: msg.Content,
				},
			},
		)
	}
	for _, part := range msg.ContentParts {
		contentPart, err := m.convertContentPart(part)
		if err != nil {
			continue
		}
		if contentPart == nil {
			continue
		}
		// For non-file or non-skipped file types, add to contentParts.
		contentParts = append(contentParts, *contentPart)
	}
	return openai.ChatCompletionUserMessageParamContentUnion{
		OfArrayOfContentParts: contentParts,
	}
}

// convertSystemMessageContent converts message content to system message content union.
// the core logic is to convert our ContentParts to OpenAI's content parts.
func (m *OpenAIChatModel) convertSystemMessageContent(msg message.Message) openai.ChatCompletionSystemMessageParamContentUnion {
	if len(msg.ContentParts) == 0 && msg.Content != "" {
		return openai.ChatCompletionSystemMessageParamContentUnion{
			OfString: openai.String(msg.Content),
		}
	}
	// Convert content parts to OpenAI content parts.
	var contentParts []openai.ChatCompletionContentPartTextParam
	if msg.Content != "" {
		contentParts = append(contentParts, openai.ChatCompletionContentPartTextParam{
			Text: msg.Content,
		})
	}
	for _, part := range msg.ContentParts {
		if part.Type == message.ContentTypeText && part.Text != nil {
			contentParts = append(contentParts, openai.ChatCompletionContentPartTextParam{
				Text: *part.Text,
			})
		}
	}
	return openai.ChatCompletionSystemMessageParamContentUnion{
		OfArrayOfContentParts: contentParts,
	}
}

// convertAssistantMessageContent converts message content to assistant message content union.
func (m *OpenAIChatModel) convertAssistantMessageContent(msg message.Message,
) openai.ChatCompletionAssistantMessageParamContentUnion {
	if len(msg.ContentParts) == 0 && msg.Content != "" {
		return openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: openai.String(msg.Content),
		}
	}
	// Convert content parts to OpenAI content parts.
	var contentParts []openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion
	if msg.Content != "" {
		contentParts = append(contentParts, openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion{
			OfText: &openai.ChatCompletionContentPartTextParam{
				Text: msg.Content,
			},
		})
	}
	for _, part := range msg.ContentParts {
		if part.Type == message.ContentTypeText && part.Text != nil {
			contentParts = append(contentParts,
				openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion{
					OfText: &openai.ChatCompletionContentPartTextParam{
						Text: *part.Text,
					},
				})
		}
	}
	return openai.ChatCompletionAssistantMessageParamContentUnion{
		OfArrayOfContentParts: contentParts,
	}
}

// convertToolCalls converts tool calls to OpenAI format.
func (m *OpenAIChatModel) convertToolCalls(toolCalls []message.ToolCall,
) []openai.ChatCompletionMessageToolCallUnionParam {
	var result []openai.ChatCompletionMessageToolCallUnionParam
	for _, tc := range toolCalls {
		result = append(result, openai.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
				ID: tc.ID,
				Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
					Name:      tc.ToolDefinition.Name,
					Arguments: string(tc.ToolDefinition.Parameters),
				},
			},
		})
	}
	return result
}

// convertMessages converts our Message format to OpenAI's format.
func (m *OpenAIChatModel) ConvertToOpenAIMessages(messages []message.Message) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, len(messages))

	for i, msg := range messages {
		// The user and system message don't include tool calls.
		// so the conversion is straightforward.
		toUserMessage := func() openai.ChatCompletionMessageParamUnion {
			content := m.convertUserMessageContent(msg)
			userMessage := &openai.ChatCompletionUserMessageParam{
				Content: content,
			}
			return openai.ChatCompletionMessageParamUnion{
				OfUser: userMessage,
			}
		}
		switch msg.Role {
		case message.RoleSystem:
			result[i] = openai.ChatCompletionMessageParamUnion{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: m.convertSystemMessageContent(msg),
				},
			}
		case message.RoleAssistant:
			assistantMsg := &openai.ChatCompletionAssistantMessageParam{
				Content:   m.convertAssistantMessageContent(msg),
				ToolCalls: m.convertToolCalls(msg.ToolCalls),
			}
			result[i] = openai.ChatCompletionMessageParamUnion{
				OfAssistant: assistantMsg,
			}
		case message.RoleTool:
			// TODO: implement tool message conversion
			result[i] = openai.ChatCompletionMessageParamUnion{
				OfTool: &openai.ChatCompletionToolMessageParam{
					Content: openai.ChatCompletionToolMessageParamContentUnion{
						OfString: openai.String(msg.Content),
					},
					ToolCallID: msg.ToolID,
				},
			}
		case message.RoleUser:
			result[i] = toUserMessage()
		default: // Default to user message if role is unknown.
			result[i] = toUserMessage()
		}
	}

	return result
}

// ConvertToOpenAITools converts provider-agnostic ToolDefinitions to OpenAI tools.
func (m *OpenAIChatModel) ConvertToOpenAITools(tools map[string]tools.BaseTool,
) []openai.ChatCompletionToolUnionParam {
	var result []openai.ChatCompletionToolUnionParam
	for _, t := range tools {
		s := t.Schema().Input
		// convert the InputSchema to JSON schema that map to OpenAI format
		schemaBytes, err := json.Marshal(s)
		if err != nil {
			continue
		}
		var params shared.FunctionParameters
		// unmarshal to the openai FunctionDefinitionParam format
		if err := json.Unmarshal(schemaBytes, &params); err != nil {
			continue
		}
		result = append(result, openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: openai.FunctionDefinitionParam{
					Name:        t.Name(),
					Description: openai.String(t.Description()),
					Parameters:  params,
				},
			},
		})
	}
	return result
}

// ConvertToOpenAIRequest converts a provider-agnostic Request to an OpenAI ChatCompletionNewParams.
func (m *OpenAIChatModel) ConvertToOpenAIRequest(req *message.Request) (openai.ChatCompletionNewParams,
	[]openaiopt.RequestOption) {
	chatRequest := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(m.model),
		Messages: m.ConvertToOpenAIMessages(req.Messages),
		Tools:    m.ConvertToOpenAITools(req.Tools),
	}

	if req.Temperature != nil {
		chatRequest.Temperature = openai.Float(*req.Temperature)
	}
	if req.MaxTokens != nil {
		chatRequest.MaxTokens = openai.Int(int64(*req.MaxTokens))
	}
	if req.TopP != nil {
		chatRequest.TopP = openai.Float(*req.TopP)
	}

	if len(req.Stop) > 0 {
		chatRequest.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfString: openai.String(req.Stop[0]),
		}
	}

	return chatRequest, nil
}

func defaultToolType(t string) string {
	if t == "" {
		return "function"
	}
	return t
}

func setStringField(field reflect.Value, value string) bool {
	if !field.IsValid() || !field.CanSet() {
		return false
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
		return true
	case reflect.Pointer:
		if field.Type().Elem().Kind() != reflect.String {
			return false
		}
		if value == "" {
			field.Set(reflect.Zero(field.Type()))
			return true
		}
		v := reflect.New(field.Type().Elem())
		v.Elem().SetString(value)
		field.Set(v)
		return true
	default:
		return false
	}
}

func setJSONField(field reflect.Value, raw json.RawMessage) bool {
	if !field.IsValid() || !field.CanSet() || len(raw) == 0 {
		return false
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(string(raw))
		return true
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.Uint8 {
			field.SetBytes(raw)
			return true
		}
	case reflect.Map, reflect.Struct:
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return false
		}
		val := reflect.ValueOf(v)
		if val.Type().AssignableTo(field.Type()) {
			field.Set(val)
			return true
		}
		if val.Type().ConvertibleTo(field.Type()) {
			field.Set(val.Convert(field.Type()))
			return true
		}
	}
	return false
}

// Generater performs a single non-streaming generation.
func (m *OpenAIChatModel) Generater(
	ctx context.Context,
	req *message.Request,
	opt ...model.BaseOption,
) (*message.Response, error) {
	if req == nil {
		return nil, model.ErrNilRequest
	}

	// convert to OpenAI request
	chatRequest, _ := m.ConvertToOpenAIRequest(req)

	// call OpenAI API
	chatCompletion, err := m.client.Chat.Completions.New(ctx, chatRequest)

	// handle error
	if err != nil {
		errorResponse := &message.Response{
			Error: &message.ResponseError{
				Message: err.Error(),
				Type:    message.ErrorTypeAPIError,
			},
			Time: time.Now(),
			Done: true,
		}
		return errorResponse, err
	}

	// select the first choice as the response
	if len(chatCompletion.Choices) == 0 {
		errorResponse := &message.Response{
			Error: &message.ResponseError{
				Message: "no choices returned from OpenAI",
				Type:    message.ErrorTypeNoResponding,
			},
		}
		return errorResponse, nil
	}

	// generate the response
	response := &message.Response{
		ID:    chatCompletion.ID,
		Model: string(chatCompletion.Model),
		Time:  time.Now(),
		Done:  true,
	}

	// convert the openai choices to our message format
	response.ResponseChoices = make([]message.ResponseChoice, len(chatCompletion.Choices))
	for i, choice := range chatCompletion.Choices {
		response.ResponseChoices[i] = message.ResponseChoice{
			Index: i,
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: choice.Message.Content,
			},
		}

		// extract tool calls if present
		response.ResponseChoices[i].Message.ToolCalls = make([]message.ToolCall, len(choice.Message.ToolCalls))
		for j, tc := range choice.Message.ToolCalls {
			response.ResponseChoices[i].Message.ToolCalls[j] = message.ToolCall{
				ID: tc.ID,
				ToolDefinition: message.ToolDefinition{
					Name:       tc.Function.Name,
					Parameters: json.RawMessage(tc.Function.Arguments),
				},
			}
		}

		if choice.FinishReason != "" {
			finishReason := choice.FinishReason
			response.ResponseChoices[i].FinishReason = &finishReason
		}
	}

	// Convert usage information.
	if chatCompletion.Usage.PromptTokens > 0 || chatCompletion.Usage.CompletionTokens > 0 {
		usage := message.Usage{
			PromptTokens:     int(chatCompletion.Usage.PromptTokens),
			CompletionTokens: int(chatCompletion.Usage.CompletionTokens),
			TotalTokens:      int(chatCompletion.Usage.TotalTokens),
		}
		response.Usage = &usage
	}

	return response, nil
}

// createPartialResponse creates a partial response from a chunk.
func (m *OpenAIChatModel) createPartialResponse(chunk openai.ChatCompletionChunk) *message.Response {
	response := &message.Response{
		ID:        chunk.ID,
		Model:     chunk.Model,
		Time:      time.Now(),
		Done:      false,
		IsPartial: true,
	}

	// Convert choices for partial responses (content streaming).
	if len(chunk.Choices) > 0 {
		if response.ResponseChoices == nil {
			response.ResponseChoices = make([]message.ResponseChoice, len(chunk.Choices))
		}

		for i, choice := range chunk.Choices {
			if response.ResponseChoices[i].Message.Content == "" {
				response.ResponseChoices[i] = message.ResponseChoice{
					Index: i,
					Message: message.Message{
						Role:    message.RoleAssistant,
						Content: choice.Delta.Content,
					},
				}
			}

			if choice.FinishReason != "" {
				finishReason := choice.FinishReason
				response.ResponseChoices[i].FinishReason = &finishReason
			}
		}
	}

	return response
}

// updateToolCallIndexMapping updates the tool call index mapping (multi-choice compatible).
// TODO: 是否有问题
func (m *OpenAIChatModel) updateToolCallIndexMapping(chunk openai.ChatCompletionChunk,
	idToIndexMap map[int]IDMap) {
	for _, ch := range chunk.Choices {
		if len(ch.Delta.ToolCalls) == 0 {
			continue
		}

		choiceIndex := int(ch.Index)
		if idToIndexMap[choiceIndex].mapping == nil {
			idToIndexMap[choiceIndex] = IDMap{
				mapping: make(map[string]int),
			}
		}

		toolCall := ch.Delta.ToolCalls[0]
		index := int(toolCall.Index)
		if toolCall.ID != "" {
			idToIndexMap[choiceIndex].mapping[toolCall.ID] = index
		}
	}
}

// generateToolCalls generates tool calls from the accumulator for one choice.
func (m *OpenAIChatModel) generateToolCalls(choice openai.ChatCompletionChoice,
	idToIndexMap map[string]int) []message.ToolCall {

	accumulatedToolCalls := make([]message.ToolCall, 0, len(choice.Message.ToolCalls))

	for i, tc := range choice.Message.ToolCalls {
		// process each tool call
		if tc.Function.Name == "" && tc.ID == "" {
			continue
		}

		// Use the original index from ID->Index mapping if available, otherwise use loop index.
		// 为什么需要映射ID
		Index := i
		if tc.ID != "" {
			if mappedIndex, ok := idToIndexMap[tc.ID]; ok {
				Index = mappedIndex
			}
		}
		accumulatedToolCalls = append(accumulatedToolCalls, message.ToolCall{
			ID:    tc.ID,
			Index: func() *int { idx := Index; return &idx }(),
			ToolDefinition: message.ToolDefinition{
				Name:       tc.Function.Name,
				Parameters: json.RawMessage(tc.Function.Arguments),
			},
		})
	}

	return accumulatedToolCalls
}

// Stream performs a streaming generation.
func (m *OpenAIChatModel) Stream(
	ctx context.Context,
	req *message.Request,
	opts ...model.BaseOption,
) (<-chan *message.Response, <-chan error) {
	// 需要使用gochannels来实现streaming
	if req == nil {
		errs := make(chan error, 1)
		errs <- model.ErrNilRequest
		return nil, errs
	}

	// convert to OpenAI request
	chatRequest, _ := m.ConvertToOpenAIRequest(req)

	// create channels for response and errors
	response_chan := make(chan *message.Response, m.channelBufferSize)
	errs := make(chan error, 1)

	go func() {
		defer close(response_chan)
		defer close(errs)

		// call OpenAI API for streaming
		stream := m.client.Chat.Completions.NewStreaming(ctx, chatRequest)
		defer stream.Close()

		// Helper to accumulate chunks from a stream
		acc := openai.ChatCompletionAccumulator{}

		idToIndexMap := make(map[int]IDMap)

		for stream.Next() {
			// process each chunk
			chunk := stream.Current()

			acc.AddChunk(chunk)

			// record tool call index mapping
			m.updateToolCallIndexMapping(chunk, idToIndexMap)

			response := m.createPartialResponse(chunk)
			select {
			case response_chan <- response:
				// sent successfully
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
		}

		// Process the last response after streaming ends
		if stream.Err() == nil {
			// add the usage info if available
			finalResponse := &message.Response{
				ID:              acc.ID,
				Model:           acc.Model,
				Time:            time.Now(),
				ResponseChoices: make([]message.ResponseChoice, len(acc.Choices)),
				Usage: &message.Usage{
					PromptTokens:     int(acc.Usage.PromptTokens),
					CompletionTokens: int(acc.Usage.CompletionTokens),
					TotalTokens:      int(acc.Usage.TotalTokens),
				},
				Done:      true,
				IsPartial: false,
			}
			// usually only the first choice contains tool calls
			if len(acc.Choices[0].Message.ToolCalls) > 0 {
				finalResponse.Done = false
			}

			if len(acc.Choices) > 0 {

				for i, choice := range acc.Choices {
					choiceIndex := int(choice.Index)

					finalResponse.ResponseChoices[i] = message.ResponseChoice{
						Index: choiceIndex,
						Message: message.Message{
							Role:    message.RoleAssistant,
							Content: choice.Message.Content,
							// generate tool calls for each choice
							ToolCalls: m.generateToolCalls(choice, idToIndexMap[choiceIndex].mapping),
						},
					}
				}
			}
			select {
			case response_chan <- finalResponse:
				// sent successfully
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
		} else {
			// send the error if any
			errorResponse := &message.Response{
				Error: &message.ResponseError{
					Message: stream.Err().Error(),
					Type:    message.ErrorTypeAPIError,
				},
				Time: time.Now(),
				Done: true,
			}
			select {
			case response_chan <- errorResponse:
				// sent successfully
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
		}

	}()

	return response_chan, errs
}

// IDMap maps tool call IDs to their indices within a choice.
type IDMap struct {
	mapping map[string]int
}
