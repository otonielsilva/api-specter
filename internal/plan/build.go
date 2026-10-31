package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"slices"
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
	byLastSegment := make(map[string][]*contract.Operation)

	for _, op := range c.Operations {
		id := op.BuildID()

		p.Order = append(p.Order, id)

		segments := pathSegments(op.Path)
		lastsegment := segments[len(segments)-1]
		s := strings.TrimPrefix(lastsegment, "/")
		byLastSegment[s] = append(byLastSegment[s], &op)

		// Each {name} segment belongs to the collection made of the segments
		// before it, so subpath is read before the segment is added to it.
		var requires []Requirement
		subpath := ""
		for _, segment := range segments {
			if strings.HasPrefix(segment, "/{") && strings.HasSuffix(segment, "}") {
				name := strings.TrimSuffix(strings.TrimPrefix(segment, "/{"), "}")
				if producer, ok := pPaths[subpath]; ok {
					from, match := inferProducerFrom(producer, name)
					requires = append(requires, Requirement{
						Param:     name,
						In:        Path,
						Producers: []Producer{{Op: producer.BuildID(), From: from, Match: match}},
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

// defaultFrom is where the id is read when the producer's response gives no
// better hint.
const defaultFrom = "response.body.id"

// inferProducerFrom says where in the response of producer the value for the
// path parameter param is, and which rule found it. It looks at the properties
// of the first 2xx response body, in a fixed order so the plan does not change
// between runs. The rules, in priority order, are the parameter's own name
// (MatchParam, "animalId"), "<collection>Id" (MatchCollection, "animals" ->
// "animalId") and "id" (MatchID). Names are compared ignoring case and "_", so
// animal_id and animalID match too. Without a match it is defaultFrom with
// Unresolved.
func inferProducerFrom(producer contract.Operation, param string) (string, Match) {
	props := successProperties(producer)

	collection := strings.TrimPrefix(producer.Path[strings.LastIndex(producer.Path, "/"):], "/")
	rules := []struct {
		name  string
		match Match
	}{
		{param, MatchParam},
		{strings.TrimSuffix(collection, "s") + "id", MatchCollection},
		{"id", MatchID},
	}

	for _, rule := range rules {
		// A param called "id" is the generic rule, not a match by its name.
		if rule.match == MatchParam && normalize(param) == "id" {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(props)) {
			if normalize(name) == normalize(rule.name) {
				return "response.body." + name, rule.match
			}
		}
	}
	return defaultFrom, Unresolved
}

// successProperties returns the properties of the response body of the lowest
// 2xx status, preferring application/json, or nil when there are none.
func successProperties(op contract.Operation) map[string]*contract.Schema {
	for _, status := range slices.Sorted(maps.Keys(op.Responses)) {
		response := op.Responses[status]
		if !strings.HasPrefix(status, "2") || response == nil {
			continue
		}
		media := slices.Sorted(maps.Keys(response.Bodies))
		// application/json first, the rest keep their sorted order.
		if i := slices.Index(media, "application/json"); i > 0 {
			media = slices.Insert(slices.Delete(media, i, i+1), 0, "application/json")
		}
		for _, m := range media {
			if schema := response.Bodies[m]; schema != nil && schema.Type == contract.SchemaTypeObject {
				return schema.Properties
			}
		}
	}
	return nil
}

// normalize lowercases name and drops underscores.
func normalize(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "")
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
