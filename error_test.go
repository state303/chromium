package chromium

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_replaceAbortedError_Replaces_To_Context_Cancel(t *testing.T) {
	err := errors.New(abortedError)
	err = replaceAbortedError(err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, err.Error(), abortedError)
}

func Test_replaceAbortedError_Returns_Error_If_Not_Known(t *testing.T) {
	err := errors.New("test error")
	replaced := replaceAbortedError(err)
	assert.ErrorIs(t, replaced, err)
}

func Test_replaceAbortedError_Returns_Nil_When_Error_Is_Nil(t *testing.T) {
	assert.NotPanics(t, func() {
		assert.Nil(t, replaceAbortedError(nil))
	})
}

func Test_isKnownError_Returns_False_When_Error_Is_Nil(t *testing.T) {
	assert.NotPanics(t, func() {
		assert.False(t, isKnownError(nil))
	})
}
