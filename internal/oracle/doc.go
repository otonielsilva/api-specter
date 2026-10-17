// Package oracle decides whether an exchange (request + response) is a
// failure: server errors, responses that violate the contract, and
// explicit rules.
//
// Oracles only judge; they never send requests.
package oracle
