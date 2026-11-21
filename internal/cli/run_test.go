package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"net/url"
	"path/filepath"
	"testing"
)

// isLoopBack is the scope guard: it says whether the target of a run is on
// this machine. It judges the literal host and never resolves DNS, so a name
// other than localhost is never loopback.
func TestIsLoopBack(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		// Loopback.
		{"ipv4", "http://127.0.0.1", true},
		{"ipv4 with port", "http://127.0.0.1:8080", true},
		{"ipv4 elsewhere in 127/8", "http://127.0.0.2:8080", true},
		{"ipv6", "http://[::1]", true},
		{"ipv6 with port", "http://[::1]:8080", true},
		{"ipv4-mapped ipv6", "http://[::ffff:127.0.0.1]:8080", true},
		{"localhost", "http://localhost", true},
		{"localhost with port", "http://localhost:8080", true},
		{"localhost in upper case", "http://LOCALHOST:8080", true},
		{"https and path", "https://localhost:8443/v1", true},
		{"subdomain of localhost", "http://api.localhost:8080", true},

		// Remote.
		{"domain", "https://api.example.com", false},
		{"private ipv4", "http://192.168.1.10:8080", false},
		{"private ipv4 in 10/8", "http://10.0.0.1", false},
		{"public ipv4", "http://8.8.8.8", false},
		{"unspecified ipv4", "http://0.0.0.0:8080", false},
		{"public ipv6", "http://[2001:4860:4860::8888]", false},

		// Names that only look local. These catch Contains and HasPrefix.
		{"localhost as a subdomain prefix", "http://localhost.evil.com", false},
		{"localhost as a suffix", "http://notlocalhost", false},
		{"127.0.0.1 as a subdomain prefix", "http://127.0.0.1.evil.com", false},
		{"loopback in userinfo", "http://127.0.0.1@evil.com", false},
		{"loopback in the path", "http://evil.com/127.0.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatalf("url.Parse(%q) error = %v", tt.raw, err)
			}
			if got := isLoopBack(u); got != tt.want {
				t.Errorf("isLoopBack(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

// A remote target is refused unless --allow-remote is given. The remote host
// uses the reserved .test TLD and every case ends at the missing spec, so no
// request is ever sent: the guard either stops the run (ErrUsage) or lets it
// reach the spec (fs.ErrNotExist).
func TestRunCmdScopeGuard(t *testing.T) {
	zooPlan := filepath.Join("..", "plan", "testdata", "zoo.plan.json")
	missingSpec := filepath.Join(t.TempDir(), "nope.yaml")
	const remote = "http://api.example.test:8080"

	tests := []struct {
		name       string
		baseURL    string
		extraArgs  []string
		wantErr    error
		wantStderr string
	}{
		{name: "remote without the flag", baseURL: remote, wantErr: ErrUsage, wantStderr: `host "api.example.test" is not local`},
		{name: "remote with the flag", baseURL: remote, extraArgs: []string{"--allow-remote"}, wantErr: fs.ErrNotExist},
		{name: "loopback without the flag", baseURL: testBaseURL, wantErr: fs.ErrNotExist},
		{name: "loopback with the flag", baseURL: testBaseURL, extraArgs: []string{"--allow-remote"}, wantErr: fs.ErrNotExist},
		{name: "localhost without the flag", baseURL: "http://localhost:8080", wantErr: fs.ErrNotExist},
		{name: "localhost lookalike without the flag", baseURL: "http://localhost.evil.test", wantErr: ErrUsage, wantStderr: `host "localhost.evil.test" is not local`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--plan", zooPlan, "--spec", missingSpec, "--base-url", tt.baseURL}, tt.extraArgs...)

			var stdout, stderr bytes.Buffer
			err := runCmd(t.Context(), args, &stdout, &stderr)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("runCmd(%v) = %v, want %v", args, err, tt.wantErr)
			}
			if tt.wantStderr != "" && !bytes.Contains(stderr.Bytes(), []byte(tt.wantStderr)) {
				t.Errorf("runCmd(%v) stderr = %q, want it to contain %q", args, stderr.String(), tt.wantStderr)
			}
			if stdout.Len() > 0 {
				t.Errorf("runCmd(%v) stdout = %q, want it empty: the run must stop before the summary line", args, stdout.String())
			}
		})
	}
}
