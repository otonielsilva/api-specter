package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

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

	pPaths := postPaths(c)
	for _, op := range c.Operations {
		id := op.BuildID()

		p.Order = append(p.Order, id)

		// Each {name} segment belongs to the collection made of the segments
		// before it, so subpath is read before the segment is added to it.
		var requires []Requirement
		subpath := ""
		for _, segment := range pathSegments(op.Path) {
			if strings.HasPrefix(segment, "/{") && strings.HasSuffix(segment, "}") {
				name := strings.TrimSuffix(strings.TrimPrefix(segment, "/{"), "}")
				if producer, ok := pPaths[subpath]; ok {
					requires = append(requires, Requirement{
						Param:     name,
						In:        Path,
						Producers: []Producer{{Op: producer.BuildID(), From: "response.body.id"}},
					})
				} else {
					p.SeedsNeeded = append(p.SeedsNeeded, SeedNeed{Op: id, Param: name, In: Path})
				}
			}
			subpath += segment
		}

		// Every operation gets an entry, even with nothing to require, so
		// the owner can see where to fill one in. Tags stay empty until the
		// dependency step exists.
		p.Operations[id] = Operator{Enabled: true, Requires: requires}
	}
	// Empty, not null, in the JSON: SeedsNeeded, Cycles and each entry's
	// Tags and Requires.
	p.fillEmpty()
	return p
}

// pathSegments splits path keeping the slash on each segment, so
// "/animal/{id}/feedings/{x}" is "/animal", "/{id}", "/feedings", "/{x}".
func pathSegments(path string) []string {
	names := strings.Split(strings.TrimPrefix(path, "/"), "/")
	segments := make([]string, len(names))
	for i, name := range names {
		segments[i] = "/" + name
	}
	return segments
}

// postPaths returns the set of paths that have a POST operation. Build uses
// it to tell which collections can produce a value. It reads the whole
// contract before any operation is looked at, so the answer does not depend
// on the order of c.Operations.
func postPaths(c *contract.Contract) map[string]contract.Operation {
	paths := make(map[string]contract.Operation)
	for _, op := range c.Operations {
		if op.Method == contract.POST {
			paths[op.Path] = op
		}
	}
	return paths
}
