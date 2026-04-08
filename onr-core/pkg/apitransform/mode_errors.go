package apitransform

import (
	"fmt"
	"strings"
)

func unsupportedModeError(op, mode string) error {
	return fmt.Errorf("unsupported %s mode %q", strings.TrimSpace(op), strings.TrimSpace(mode))
}
