//go:build !windows

package service

import (
	"context"
	"errors"
	"io"
)

func Command(context.Context, []string, io.Writer, func(context.Context, []string, io.Writer) error) error {
	return errors.New("native service commands are Windows-only; use systemctl on Linux")
}
