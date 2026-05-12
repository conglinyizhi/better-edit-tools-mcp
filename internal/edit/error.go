package edit

import "fmt"

type Error struct {
	Kind string
	Path string
	Err  error
	Msg  string
}

func (e *Error) Error() string {
	switch e.Kind {
	case "read":
		return fmt.Sprintf("read %s: %v", e.Path, e.Err)
	case "write":
		return fmt.Sprintf("write %s: %v", e.Path, e.Err)
	case "json":
		return fmt.Sprintf("parse JSON: %v", e.Err)
	default:
		return e.Msg
	}
}

func InvalidArg(msg string) error {
	return &Error{Kind: "invalid", Msg: msg}
}

func ReadPath(path string, err error) error {
	return &Error{Kind: "read", Path: path, Err: err}
}

func WritePath(path string, err error) error {
	return &Error{Kind: "write", Path: path, Err: err}
}

func JsonParse(err error) error {
	return &Error{Kind: "json", Err: err}
}
