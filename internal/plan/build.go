package plan

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/otonielsilva/api-specter/internal/contract"
)

// Build derives a plan from a contract. specPath is stored as written and
// specBytes are the raw bytes of the document the contract was parsed from;
// their SHA-256 goes into Spec.Hash so "run" can detect a stale plan.
func Build(c *contract.Contract, specPath string, specBytes []byte) *Plan {
	sum := sha256.Sum256(specBytes)

	p := &Plan{
		Version:   1,
		Generator: HeuristicGenerated,
		Spec: Spec{
			Path: specPath,
			Hash: "sha256:" + hex.EncodeToString(sum[:]),
		},
		Order:      make([]string, 0, len(c.Operations)),
		Operations: make(map[string]Operator, len(c.Operations)),
	}
	for _, op := range c.Operations {
		id := string(op.Method) + " " + op.Path
		p.Order = append(p.Order, id)
		// Every operation gets an entry, even with nothing to require, so
		// the owner can see where to fill one in. Tags and Requires stay
		// empty until the dependency step exists.
		p.Operations[id] = Operator{Enabled: true}
	}
	// Empty, not null, in the JSON: SeedsNeeded, Cycles and each entry's
	// Tags and Requires.
	p.fillEmpty()
	return p
}
