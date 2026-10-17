// Package generator produces request inputs from the contract model: valid
// values, boundary values, invalid values and mutations.
//
// Every generator draws randomness from an explicitly passed source so that
// a run is fully reproducible from its seed.
package generator
