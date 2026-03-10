package tools

// import (
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"io"
// 	"os"
// 	"path/filepath"
// 	"strings"
// 	"time"
// )

// const defaultMaxReadBytes = 1 << 20 // 1MB

// var ErrEmptyPath = errors.New("tools: empty path")

// // RegisterBuiltinFileTools registers a set of file operation tools into the registry.
// // If reg is nil, DefaultRegistry is used.
// func RegisterBuiltinFileTools(reg *Registry) error {
// 	if reg == nil {
// 		reg = DefaultRegistry
// 	}

// 	readSchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "File path to read.",
// 			},
// 			"max_bytes": {
// 				Type:        "integer",
// 				Description: "Maximum bytes to read. Default is 1MB.",
// 			},
// 		},
// 		Required:             []string{"path"},
// 		AdditionalProperties: false,
// 	}

// 	writeSchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "File path to write.",
// 			},
// 			"content": {
// 				Type:        "string",
// 				Description: "File content.",
// 			},
// 			"create_dirs": {
// 				Type:        "boolean",
// 				Description: "Create parent directories if missing.",
// 			},
// 		},
// 		Required:             []string{"path", "content"},
// 		AdditionalProperties: false,
// 	}

// 	deleteSchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "File or directory path to delete.",
// 			},
// 			"recursive": {
// 				Type:        "boolean",
// 				Description: "Delete directory recursively.",
// 			},
// 		},
// 		Required:             []string{"path"},
// 		AdditionalProperties: false,
// 	}

// 	listSchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "Directory path to list.",
// 			},
// 			"recursive": {
// 				Type:        "boolean",
// 				Description: "List directory recursively.",
// 			},
// 		},
// 		Required:             []string{"path"},
// 		AdditionalProperties: false,
// 	}

// 	statSchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "File path to stat.",
// 			},
// 		},
// 		Required:             []string{"path"},
// 		AdditionalProperties: false,
// 	}

// 	readOutputSchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "File path.",
// 			},
// 			"content": {
// 				Type:        "string",
// 				Description: "File content (possibly truncated).",
// 			},
// 			"size": {
// 				Type:        "integer",
// 				Description: "Number of bytes returned in content.",
// 			},
// 			"truncated": {
// 				Type:        "boolean",
// 				Description: "Whether the output was truncated by max_bytes.",
// 			},
// 		},
// 		Required:             []string{"path", "content", "size", "truncated"},
// 		AdditionalProperties: false,
// 	}

// 	writeOutputSchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "File path.",
// 			},
// 			"size": {
// 				Type:        "integer",
// 				Description: "File size after write.",
// 			},
// 			"bytes": {
// 				Type:        "integer",
// 				Description: "Number of bytes written.",
// 			},
// 		},
// 		Required:             []string{"path", "size", "bytes"},
// 		AdditionalProperties: false,
// 	}

// 	deleteOutputSchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "Deleted path.",
// 			},
// 		},
// 		Required:             []string{"path"},
// 		AdditionalProperties: false,
// 	}

// 	listEntrySchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "Entry path.",
// 			},
// 			"name": {
// 				Type:        "string",
// 				Description: "Entry name.",
// 			},
// 			"is_dir": {
// 				Type:        "boolean",
// 				Description: "Whether entry is a directory.",
// 			},
// 			"size": {
// 				Type:        "integer",
// 				Description: "Entry size in bytes.",
// 			},
// 			"mod_time": {
// 				Type:        "string",
// 				Description: "Entry modification time (RFC3339).",
// 			},
// 		},
// 		Required:             []string{"path", "name", "is_dir", "size", "mod_time"},
// 		AdditionalProperties: false,
// 	}

// 	listOutputSchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "Listed directory path.",
// 			},
// 			"entries": {
// 				Type:        "array",
// 				Description: "Directory entries.",
// 				Items:       listEntrySchema,
// 			},
// 		},
// 		Required:             []string{"path", "entries"},
// 		AdditionalProperties: false,
// 	}

// 	statOutputSchema := &JSONSchema{
// 		Type: "object",
// 		Properties: map[string]*JSONSchema{
// 			"path": {
// 				Type:        "string",
// 				Description: "File path.",
// 			},
// 			"is_dir": {
// 				Type:        "boolean",
// 				Description: "Whether path is a directory.",
// 			},
// 			"size": {
// 				Type:        "integer",
// 				Description: "File size in bytes.",
// 			},
// 			"mod_time": {
// 				Type:        "string",
// 				Description: "Modification time (RFC3339).",
// 			},
// 		},
// 		Required:             []string{"path", "is_dir", "size", "mod_time"},
// 		AdditionalProperties: false,
// 	}

// 	readTool, err := NewTool(
// 		"file_read",
// 		"Read a file. input JSON: {\"path\": string, \"max_bytes\": int}.",
// 		readSchema,
// 		readOutputSchema,
// 		func(input string) (string, error) { return runFileRead(input) },
// 	)
// 	if err != nil {
// 		return err
// 	}
// 	if err := reg.Register(readTool); err != nil {
// 		return err
// 	}

// 	writeTool, err := NewTool(
// 		"file_write",
// 		"Write a file (overwrite). input JSON: {\"path\": string, \"content\": string, \"create_dirs\": bool}.",
// 		writeSchema,
// 		writeOutputSchema,
// 		func(input string) (string, error) { return runFileWrite(input, false) },
// 	)
// 	if err != nil {
// 		return err
// 	}
// 	if err := reg.Register(writeTool); err != nil {
// 		return err
// 	}

// 	appendTool, err := NewTool(
// 		"file_append",
// 		"Append content to a file. input JSON: {\"path\": string, \"content\": string, \"create_dirs\": bool}.",
// 		writeSchema,
// 		writeOutputSchema,
// 		func(input string) (string, error) { return runFileWrite(input, true) },
// 	)
// 	if err != nil {
// 		return err
// 	}
// 	if err := reg.Register(appendTool); err != nil {
// 		return err
// 	}

// 	deleteTool, err := NewTool(
// 		"file_delete",
// 		"Delete a file or directory. input JSON: {\"path\": string, \"recursive\": bool}.",
// 		deleteSchema,
// 		deleteOutputSchema,
// 		func(input string) (string, error) { return runFileDelete(input) },
// 	)
// 	if err != nil {
// 		return err
// 	}
// 	if err := reg.Register(deleteTool); err != nil {
// 		return err
// 	}

// 	listTool, err := NewTool(
// 		"file_list",
// 		"List directory entries. input JSON: {\"path\": string, \"recursive\": bool}.",
// 		listSchema,
// 		listOutputSchema,
// 		func(input string) (string, error) { return runFileList(input) },
// 	)
// 	if err != nil {
// 		return err
// 	}
// 	if err := reg.Register(listTool); err != nil {
// 		return err
// 	}

// 	statTool, err := NewTool(
// 		"file_stat",
// 		"Get file info. input JSON: {\"path\": string}.",
// 		statSchema,
// 		statOutputSchema,
// 		func(input string) (string, error) { return runFileStat(input) },
// 	)
// 	if err != nil {
// 		return err
// 	}
// 	if err := reg.Register(statTool); err != nil {
// 		return err
// 	}

// 	return nil
// }

// // MustRegisterBuiltinFileTools registers file tools and panics on error.
// func MustRegisterBuiltinFileTools(reg *Registry) {
// 	if err := RegisterBuiltinFileTools(reg); err != nil {
// 		panic(err)
// 	}
// }

// type fileReadInput struct {
// 	Path     string `json:"path"`
// 	MaxBytes int    `json:"max_bytes,omitempty"`
// }

// type fileReadOutput struct {
// 	Path      string `json:"path"`
// 	Content   string `json:"content"`
// 	Size      int    `json:"size"`
// 	Truncated bool   `json:"truncated"`
// }

// type fileWriteInput struct {
// 	Path       string `json:"path"`
// 	Content    string `json:"content"`
// 	CreateDirs bool   `json:"create_dirs,omitempty"`
// }

// type fileWriteOutput struct {
// 	Path  string `json:"path"`
// 	Size  int    `json:"size"`
// 	Bytes int    `json:"bytes"`
// }

// type fileDeleteInput struct {
// 	Path      string `json:"path"`
// 	Recursive bool   `json:"recursive,omitempty"`
// }

// type fileDeleteOutput struct {
// 	Path string `json:"path"`
// }

// type fileListInput struct {
// 	Path      string `json:"path"`
// 	Recursive bool   `json:"recursive,omitempty"`
// }

// type fileListEntry struct {
// 	Path    string `json:"path"`
// 	Name    string `json:"name"`
// 	IsDir   bool   `json:"is_dir"`
// 	Size    int64  `json:"size"`
// 	ModTime string `json:"mod_time"`
// }

// type fileListOutput struct {
// 	Path    string          `json:"path"`
// 	Entries []fileListEntry `json:"entries"`
// }

// type fileStatInput struct {
// 	Path string `json:"path"`
// }

// type fileStatOutput struct {
// 	Path    string `json:"path"`
// 	IsDir   bool   `json:"is_dir"`
// 	Size    int64  `json:"size"`
// 	ModTime string `json:"mod_time"`
// }

// func decodeInput[T any](input string, v *T) error {
// 	if strings.TrimSpace(input) == "" {
// 		return errors.New("tools: input is empty")
// 	}
// 	if err := json.Unmarshal([]byte(input), v); err != nil {
// 		return fmt.Errorf("tools: invalid input: %w", err)
// 	}
// 	return nil
// }

// func encodeOutput(v any) (string, error) {
// 	b, err := json.Marshal(v)
// 	if err != nil {
// 		return "", fmt.Errorf("tools: output encode failed: %w", err)
// 	}
// 	return string(b), nil
// }

// func runFileRead(input string) (string, error) {
// 	var in fileReadInput
// 	if err := decodeInput(input, &in); err != nil {
// 		return "", err
// 	}
// 	path := strings.TrimSpace(in.Path)
// 	if path == "" {
// 		return "", ErrEmptyPath
// 	}

// 	maxBytes := in.MaxBytes
// 	if maxBytes <= 0 {
// 		maxBytes = defaultMaxReadBytes
// 	}

// 	f, err := os.Open(path)
// 	if err != nil {
// 		return "", err
// 	}
// 	defer f.Close()

// 	limited := io.LimitReader(f, int64(maxBytes)+1)
// 	data, err := io.ReadAll(limited)
// 	if err != nil {
// 		return "", err
// 	}

// 	truncated := len(data) > maxBytes
// 	if truncated {
// 		data = data[:maxBytes]
// 	}

// 	out := fileReadOutput{
// 		Path:      path,
// 		Content:   string(data),
// 		Size:      len(data),
// 		Truncated: truncated,
// 	}
// 	return encodeOutput(out)
// }

// func runFileWrite(input string, appendMode bool) (string, error) {
// 	var in fileWriteInput
// 	if err := decodeInput(input, &in); err != nil {
// 		return "", err
// 	}
// 	path := strings.TrimSpace(in.Path)
// 	if path == "" {
// 		return "", ErrEmptyPath
// 	}

// 	if in.CreateDirs {
// 		dir := filepath.Dir(path)
// 		if dir != "." && dir != "" {
// 			if err := os.MkdirAll(dir, 0o755); err != nil {
// 				return "", err
// 			}
// 		}
// 	}

// 	var f *os.File
// 	var err error
// 	if appendMode {
// 		f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
// 	} else {
// 		f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
// 	}
// 	if err != nil {
// 		return "", err
// 	}
// 	defer f.Close()

// 	n, err := f.WriteString(in.Content)
// 	if err != nil {
// 		return "", err
// 	}

// 	info, err := f.Stat()
// 	if err != nil {
// 		return "", err
// 	}

// 	out := fileWriteOutput{
// 		Path:  path,
// 		Size:  int(info.Size()),
// 		Bytes: n,
// 	}
// 	return encodeOutput(out)
// }

// func runFileDelete(input string) (string, error) {
// 	var in fileDeleteInput
// 	if err := decodeInput(input, &in); err != nil {
// 		return "", err
// 	}
// 	path := strings.TrimSpace(in.Path)
// 	if path == "" {
// 		return "", ErrEmptyPath
// 	}

// 	var err error
// 	if in.Recursive {
// 		err = os.RemoveAll(path)
// 	} else {
// 		err = os.Remove(path)
// 	}
// 	if err != nil {
// 		return "", err
// 	}

// 	return encodeOutput(fileDeleteOutput{Path: path})
// }

// func runFileList(input string) (string, error) {
// 	var in fileListInput
// 	if err := decodeInput(input, &in); err != nil {
// 		return "", err
// 	}
// 	path := strings.TrimSpace(in.Path)
// 	if path == "" {
// 		return "", ErrEmptyPath
// 	}

// 	var entries []fileListEntry
// 	if in.Recursive {
// 		err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
// 			if err != nil {
// 				return err
// 			}
// 			if p == path {
// 				return nil
// 			}
// 			info, err := d.Info()
// 			if err != nil {
// 				return err
// 			}
// 			entries = append(entries, fileListEntry{
// 				Path:    p,
// 				Name:    d.Name(),
// 				IsDir:   d.IsDir(),
// 				Size:    info.Size(),
// 				ModTime: info.ModTime().UTC().Format(time.RFC3339),
// 			})
// 			return nil
// 		})
// 		if err != nil {
// 			return "", err
// 		}
// 	} else {
// 		items, err := os.ReadDir(path)
// 		if err != nil {
// 			return "", err
// 		}
// 		for _, it := range items {
// 			info, err := it.Info()
// 			if err != nil {
// 				return "", err
// 			}
// 			entries = append(entries, fileListEntry{
// 				Path:    filepath.Join(path, it.Name()),
// 				Name:    it.Name(),
// 				IsDir:   it.IsDir(),
// 				Size:    info.Size(),
// 				ModTime: info.ModTime().UTC().Format(time.RFC3339),
// 			})
// 		}
// 	}

// 	return encodeOutput(fileListOutput{Path: path, Entries: entries})
// }

// func runFileStat(input string) (string, error) {
// 	var in fileStatInput
// 	if err := decodeInput(input, &in); err != nil {
// 		return "", err
// 	}
// 	path := strings.TrimSpace(in.Path)
// 	if path == "" {
// 		return "", ErrEmptyPath
// 	}

// 	info, err := os.Stat(path)
// 	if err != nil {
// 		return "", err
// 	}

// 	return encodeOutput(fileStatOutput{
// 		Path:    path,
// 		IsDir:   info.IsDir(),
// 		Size:    info.Size(),
// 		ModTime: info.ModTime().UTC().Format(time.RFC3339),
// 	})
// }
