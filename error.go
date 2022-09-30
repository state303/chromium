package chromium

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// defined errors for uniform error handling.

var (
	ElementMissing = errors.New("element missing")
	InputFailed    = errors.New("input failed")
	WaitFailed     = errors.New("wait failed")
	ClickFailed    = errors.New("click failed")
	TaskTimeout    = errors.New("task timeout")
)

// wrapError wraps an error with given topic, such that the type of error to be consistent.
func wrap(err error, topic string) error {
	return fmt.Errorf("%w, %+v", replaceAbortedError(err), topic)
}

func replaceAbortedError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "ABORTED") {
		return context.Canceled
	}
	return err
}

func isKnownError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ElementMissing) ||
		errors.Is(err, InputFailed) ||
		errors.Is(err, WaitFailed) ||
		errors.Is(err, ClickFailed) ||
		errors.Is(err, TaskTimeout) ||
		errors.Is(err, context.Canceled)
}
