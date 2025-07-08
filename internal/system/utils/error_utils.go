package utils

import (
	"encoding/json"
	"fmt"
	"io"
)

func HandleDecodeError(err error, resourceName string) string {

	var message string
	switch {
	case err == io.EOF:
		message = fmt.Sprintf("Request body for %s is empty.", resourceName)
	case json.UnmarshalTypeError{} != (json.UnmarshalTypeError{}):
		if ute, ok := err.(*json.UnmarshalTypeError); ok {
			message = fmt.Sprintf("Invalid type for field '%s'. Expected %s.", ute.Field, ute.Type)
		} else {
			message = fmt.Sprintf("Invalid type in %s request body.", resourceName)
		}
	case json.SyntaxError{} != (json.SyntaxError{}):
		if se, ok := err.(*json.SyntaxError); ok {
			message = fmt.Sprintf("Malformed JSON at position %d.", se.Offset)
		} else {
			message = fmt.Sprintf("Malformed JSON in %s request body.", resourceName)
		}
	default:
		message = fmt.Sprintf("Invalid JSON payload for %s.", resourceName)
	}
	return message
}
