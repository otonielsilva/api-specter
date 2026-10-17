// Package cli implements the specter command line: subcommands, flags and
// output formatting.
//
// It lives outside package main so the whole CLI can be driven from tests:
// the entry point takes a context, the arguments and the output writers
// explicitly instead of reaching for os.Args, os.Stdout and os.Exit.
package cli
