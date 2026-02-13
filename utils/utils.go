package utils

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// float32PtrToFloat64Ptr converts a pointer to float32 to a pointer to float64.
func Float32PtrToFloat64Ptr(p *float32) *float64 {
	if p == nil {
		return nil
	}
	v := float64(*p)
	return &v
}

// FuncName 获取函数名，返回类似 "github.com/acme/pkg.Foo" 的字符串。
func FunctionName(fn any) (string, error) {
	if fn == nil {
		return "", fmt.Errorf("fn is nil")
	}

	// 反射获取函数指针
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return "", fmt.Errorf("fn is not a function: %s", v.Kind())
	}

	pc := v.Pointer()
	rf := runtime.FuncForPC(pc)
	if rf == nil {
		return "", fmt.Errorf("runtime.FuncForPC returned nil")
	}

	full := rf.Name() // e.g. "github.com/acme/pkg.MyFunc" or "github.com/acme/pkg.(*T).Method-fm" or ".../pkg.main.func1"
	return full, nil
}

// ShortFunctionName 可选：把 "github.com/acme/pkg.Foo" 规整成 "Foo"。
func ShortFunctionName(full string) string {
	// 去掉路径，只保留最后一段包名+符号： "pkg.Foo"
	if i := strings.LastIndex(full, "/"); i >= 0 {
		full = full[i+1:]
	}
	// full 可能是 "pkg.Foo" 或 "pkg.(*T).Method-fm"
	if i := strings.LastIndex(full, "."); i >= 0 {
		full = full[i+1:]
	}
	// 去掉 method value 后缀 "-fm"
	full = strings.TrimSuffix(full, "-fm")
	return full
}
