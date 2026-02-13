package main

import (
	"encoding/json"
	"fmt"
	"reflect"

	"xgc-agent/tools"
)

type Nested struct {
	Age  int     `json:"age"`
	Nick *string `json:"nick,omitempty"`
}

type Input struct {
	Path     string         `json:"path"`
	MaxBytes int            `json:"max_bytes,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Meta     map[string]int `json:"meta,omitempty"`
	Nested   Nested         `json:"nested"`
	Opt      *Nested        `json:"opt,omitempty"`
	Ignored  string         `json:"-"`
	Private  string         // unexported fields can't appear; kept exported here intentionally
}

type WithPtr struct {
	Count *int `json:"count"`
}

func printSchema(label string, t reflect.Type) {
	s := tools.GenerateJSONSchema(t)
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Printf("[%s] marshal error: %v\n", label, err)
		return
	}
	fmt.Printf("=== %s (%s) ===\n%s\n\n", label, t.String(), string(b))
}

func main() {
	printSchema("Input", reflect.TypeOf(Input{}))
	printSchema("*Input", reflect.TypeOf(&Input{}))
	printSchema("WithPtr", reflect.TypeOf(WithPtr{}))
	printSchema("[]string", reflect.TypeOf([]string{}))
	printSchema("map[string]int", reflect.TypeOf(map[string]int{}))
}
