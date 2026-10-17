// Package config reads specter's settings from the environment, optionally
// seeded from a .env file.
package config

import (
	"cmp"
	"errors"
	"io/fs"
	"net"
	"os"

	"github.com/joho/godotenv"
)

// LoadEnvFile sets variables from a .env file. Variables already set in the
// environment win over the file, and a missing file is not an error.
func LoadEnvFile(path string) error {
	err := godotenv.Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// ZooAddr returns the address the bug zoo listens on. The zoo only binds to
// loopback; the port comes from ZOO_HTTP_PORT and defaults to 8080.
func ZooAddr() string {
	return net.JoinHostPort("127.0.0.1", cmp.Or(os.Getenv("ZOO_HTTP_PORT"), "8080"))
}
