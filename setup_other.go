//go:build !darwin && !windows

package main

import (
	"context"
	"errors"
	"io"
)

func setupMediaTools(_ context.Context, _ setupMirror, _ string, _ io.Writer) (toolset, error) {
	return toolset{}, errors.New("automatic media installation requires macOS 15+ arm64 or Windows 11 x64")
}
