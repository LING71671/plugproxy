package errtype

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
)

type Type string

const (
	Timeout             Type = "timeout"
	ConnectionError     Type = "connection_error"
	HTTPStatus          Type = "http_status"
	BodyLimit           Type = "body_limit"
	ParseError          Type = "parse_error"
	ContextCancelled    Type = "context_cancelled"
	ProtocolUnsupported Type = "protocol_unsupported"
	ResponseError       Type = "response_error"
	Unknown             Type = "unknown"
)

type Error struct {
	Type Type
	Err  error
}

func (e Error) Error() string {
	if e.Err == nil {
		return string(e.Type)
	}
	return e.Err.Error()
}

func (e Error) Unwrap() error {
	return e.Err
}

func Wrap(kind Type, err error) error {
	if err == nil {
		return nil
	}
	return Error{Type: kind, Err: err}
}

func Classify(err error) Type {
	if err == nil {
		return ""
	}
	var typed Error
	if errors.As(err, &typed) {
		return typed.Type
	}
	if errors.Is(err, context.Canceled) {
		return ContextCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return Timeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return Timeout
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return Classify(urlErr.Err)
	}

	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "unsupported protocol"):
		return ProtocolUnsupported
	case strings.Contains(text, "connection refused"),
		strings.Contains(text, "no such host"),
		strings.Contains(text, "network is unreachable"),
		strings.Contains(text, "connection reset"),
		strings.Contains(text, "connectex"):
		return ConnectionError
	case strings.Contains(text, "timeout"),
		strings.Contains(text, "deadline exceeded"):
		return Timeout
	default:
		return Unknown
	}
}
