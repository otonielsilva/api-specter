package cli

import "testing"

func TestRun(t *testing.T) {
	t.Skip("TODO: table-driven; no args and unknown command return ErrUsage, help/-h/--help print usage on stdout and return nil")
}

func TestPlanCmd(t *testing.T) {
	t.Skip("TODO: -h returns nil; missing --spec returns ErrUsage; unknown flag returns ErrUsage; missing spec file returns a non-usage error")
}

func TestRunCmd(t *testing.T) {
	t.Skip("TODO: -h returns nil; missing --plan returns ErrUsage; zoo plan with internal/zoo/openapi.yaml succeeds; missing plan file and non-JSON plan fail")
}

func TestSpecLocation(t *testing.T) {
	t.Skip("TODO: --spec wins; absolute path in the plan is kept; relative path is resolved from the plan's directory")
}
