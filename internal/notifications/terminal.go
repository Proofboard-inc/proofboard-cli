package notifications

import (
	"fmt"
	"io"
)

func Print(out io.Writer, message string) error {
	_, err := fmt.Fprintln(out, message)
	return err
}
