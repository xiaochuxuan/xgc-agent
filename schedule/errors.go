package schedule

import "errors"

var (
	ErrUnknowNodeState     = errors.New("unknown node state")
	ErrInodeRequired       = errors.New("input Inode required")
	ErrNodeRequired        = errors.New("node required")
	ErrFromNodeTypeInvalid = errors.New("from node type invalid")
)
