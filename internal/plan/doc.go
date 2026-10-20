// Package plan defines the plan file: the dependencies between the
// operations of an API and the order to run them in.
//
// "specter plan" writes a plan and "specter run" reads it. This package owns
// the types and the code that loads a plan, and it is the only thing the two
// commands share.
package plan
