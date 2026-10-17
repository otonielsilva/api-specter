// Package executor sends generated requests to the target API and records
// what came back: status, headers, body and timing.
//
// It owns everything about load: concurrency limits, rate limiting,
// deadlines, cancellation and the scope guard that keeps requests on
// authorized hosts.
package executor
