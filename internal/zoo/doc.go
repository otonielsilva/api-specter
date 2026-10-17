// Package zoo is a deliberately buggy HTTP API used as specter's benchmark
// target.
//
// Every planted bug has a stable ID, is described in BUGS.md and is marked in
// the code with a "BUG-NNN" comment. The package tests assert that each bug
// still reproduces: they are the ground truth that fuzzing results are
// measured against, so a failing zoo test means the benchmark broke, not that
// a bug got fixed (see ADR 0004).
package zoo
