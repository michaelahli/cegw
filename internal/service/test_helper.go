package service

import (
	"io"

	"github.com/michaelahli/cegw/internal/logger"
)

func newTestLogger() *logger.Logger {
	return logger.New("error", io.Discard)
}
