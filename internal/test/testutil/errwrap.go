package testutil

import "fmt"

func WrapError(err error, msg string) error {
	if err == nil {
		return fmt.Errorf(msg)
	}
	return fmt.Errorf("%w, %+v", err, msg)
}
