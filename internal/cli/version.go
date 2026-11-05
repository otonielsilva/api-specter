package cli

import (
	"fmt"
	"io"
)

var version = "dev"

// versionCmd implements "specter version".
func versionCmd(stdout io.Writer) error {
	fmt.Fprintln(stdout, version)

	return nil
}
