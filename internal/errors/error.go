package errors

import "fmt"

type ErrorMessage struct {
	Code        string `json:"error_code"`
	Message     string `json:"error_message"`
	Description string `json:"error_description"`
	TraceID     string `json:"traceId"`
}

type ClientError struct {
	ErrorMessage
	StatusCode int
}

type ServerError struct {
	ErrorMessage
	Err error
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
}

func (e *ClientError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewServerError(msg ErrorMessage, cause error) *ServerError {
	return &ServerError{
		ErrorMessage: msg,
		Err:          cause,
	}
}

func NewClientError(msg ErrorMessage, code int) *ClientError {
	return &ClientError{
		ErrorMessage: msg,
		StatusCode:   code,
	}
}

func NewClientErrorWithoutCode(msg ErrorMessage) *ClientError {
	return &ClientError{
		ErrorMessage: msg,
	}
}
