package model

import (
	"context"
	"errors"
)

// define basic model interface
type BaseModel interface {
	Generater()
	Stream()
}

type ChatModel interface {
	BaseModel
}
