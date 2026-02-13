// option.go: Provider-agnostic model options and helpers.
package model

// Options is the common options for the model.
type BaseOptions struct {
	// Model is the model name.
	Model *string `json:"model,omitempty"`
	// maximum tokens for response generation
	// Optional. Default: model's maximum tokens
	MaxTokens *int `json:"max_tokens,omitempty"`
	// temperature for response randomness
	// Optional. Default: 1.0. Range: 0.0 to 2.0
	Temperature *float32 `json:"temperature,omitempty"`
	// top_p for nucleus sampling
	// Optional. Default: 1.0. Range: 0.0 to 1.0
	TopP *float32 `json:"top_p,omitempty"`
	// stop sequences to end generation
	// Optional.
	Stop []string `json:"stop,omitempty"`
}

// Option is the call option for ChatModel component.
type BaseOption struct {
	apply func(opts *BaseOptions)

	implSpecificOptFn any
}

// WithModel is the option to set the model name.
func WithModel(name string) BaseOption {
	return BaseOption{
		apply: func(opts *BaseOptions) {
			opts.Model = &name
		},
	}
}

// WithTemperature is the option to set the temperature for the model.
func WithTemperature(temperature float32) BaseOption {
	return BaseOption{
		apply: func(opts *BaseOptions) {
			opts.Temperature = &temperature
		},
	}
}

// WithMaxTokens is the option to set the max tokens for the model.
func WithMaxTokens(maxTokens int) BaseOption {
	return BaseOption{
		apply: func(opts *BaseOptions) {
			opts.MaxTokens = &maxTokens
		},
	}
}

// WithTopP is the option to set the top p for the model.
func WithTopP(topP float32) BaseOption {
	return BaseOption{
		apply: func(opts *BaseOptions) {
			opts.TopP = &topP
		},
	}
}

// WithStop is the option to set the stop words for the model.
func WithStop(stop []string) BaseOption {
	return BaseOption{
		apply: func(opts *BaseOptions) {
			opts.Stop = stop
		},
	}
}

// WrapImplSpecificOptFn is the option to wrap the implementation specific option function.
func WrapImplSpecificOptFn[T any](optFn func(*T)) BaseOption {
	return BaseOption{
		implSpecificOptFn: optFn,
	}
}

// GetCommonOptions extract model Options from Option list, optionally providing a base Options with default values.
func GetCommonOptions(base *BaseOptions, opts ...BaseOption) *BaseOptions {
	if base == nil {
		base = &BaseOptions{}
	}

	for i := range opts {
		opt := opts[i]
		if opt.apply != nil {
			opt.apply(base)
		}
	}

	return base
}

// GetImplSpecificOptions extract the implementation specific options from Option list, optionally providing a base options with default values.
// e.g.
//
//	myOption := &MyOption{
//		Field1: "default_value",
//	}
//
//	myOption := model.GetImplSpecificOptions(myOption, opts...)
func GetImplSpecificOptions[T any](base *T, opts ...BaseOption) *T {
	if base == nil {
		base = new(T)
	}

	for i := range opts {
		opt := opts[i]
		if opt.implSpecificOptFn != nil {
			optFn, ok := opt.implSpecificOptFn.(func(*T))
			if ok {
				optFn(base)
			}
		}
	}

	return base
}
