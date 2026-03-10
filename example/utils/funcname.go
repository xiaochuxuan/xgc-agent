package main

import (
	"fmt"

	"xgc-agent/utils"
)

func main() { functionNameExample() }

func functionNameExample() {
	name, err := utils.FunctionName(functionNameExample)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Function name:", name)
	fmt.Println("Short name:", utils.ShortFunctionName(name))
}
