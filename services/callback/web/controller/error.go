package controller

import (
	"errors"
	"fmt"
)

var (
	ErrCallbackBodyDecoding = errors.New("could not decode a callback body")
	ErrInvalidFileId        = errors.New("invalid file id request parameter")
)

type CallbackJwtVerificationError struct {
	Token  string
	Reason string
}

func (e *CallbackJwtVerificationError) Error() string {
	return fmt.Sprintf("could not verify callback jwt (%s). Reason: %s", e.Token, e.Reason)
}

type GdriveError struct {
	Operation string
	Reason    string
}

func (e *GdriveError) Error() string {
	return fmt.Sprintf("could not %s. Reason: %s", e.Operation, e.Reason)
}
