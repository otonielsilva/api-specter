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

	// Loop 1 only builds the indexes, so loop 2 does not depend on the order
	// of c.Operations.
	producerByPath, byLastSeg := indexPosts(c)

	for _, op := range c.Operations {
		id := op.BuildID()

		p.Order = append(p.Order, id)

		requires, seeds := pathRequirements(op, producerByPath)
		p.SeedsNeeded = append(p.SeedsNeeded, seeds...)
		requires = append(requires, bodyRequirements(op, requires, producerByPath, byLastSeg)...)

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

// indexPosts indexes the POST operations that create a collection, those
// whose last segment is not a {param}. producerByPath is keyed by path and
// byLastSeg maps the lowercased last segment to the paths that end in it, in
// contract order, so "/animals/{id}/feedings" is under "feedings".
//
// For a contract with POST /animals, POST /animals/{id}/feedings,
// POST /animals/{id} (skipped, it ends in a {param}) and POST /feeds/bulk:
//
//	producerByPath = {
//	    "/animals":               POST /animals,
//	    "/animals/{id}/feedings": POST /animals/{id}/feedings,
//	    "/feeds/bulk":            POST /feeds/bulk,
//	}
//	byLastSeg = {
//	    "animals":  ["/animals"],
//	    "feedings": ["/animals/{id}/feedings"],
//	    "bulk":     ["/feeds/bulk"],
//	}
func indexPosts(c *contract.Contract) (producerByPath map[string]contract.Operation, byLastSeg map[string][]string) {
	producerByPath = make(map[string]contract.Operation)
	byLastSeg = make(map[string][]string)
	for _, op := range c.Operations {
		if op.Method != contract.POST {
			continue
		}
		segments := pathSegments(op.Path)
		last := segments[len(segments)-1]
		if isParam(last) {
			continue
		}
		producerByPath[op.Path] = op
		key := strings.ToLower(strings.TrimPrefix(last, "/"))
		byLastSeg[key] = append(byLastSeg[key], op.Path)
	}
	return producerByPath, byLastSeg
}

// pathRequirements applies the path rule: each {name} segment belongs to the
// collection made of the segments before it, so the prefix is read before the
// segment is added to it. A param whose collection has no POST (or whose
// producer is op itself) is a seed the user must supply.
func pathRequirements(op contract.Operation, producerByPath map[string]contract.Operation) ([]Requirement, []SeedNeed) {
	var requires []Requirement
	var seeds []SeedNeed
	prefix := ""
	for _, segment := range pathSegments(op.Path) {
		if isParam(segment) {
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "/{"), "}")
			if producer, ok := producerByPath[prefix]; ok && producer.BuildID() != op.BuildID() {
				from, match := inferProducerFrom(producer, name)
				requires = append(requires, Requirement{
					Param:     name,
					In:        Path,
					Producers: []Producer{{Op: producer.BuildID(), From: from, Match: match}},
				})
			} else {
				seeds = append(seeds, SeedNeed{Op: op.BuildID(), Param: name, In: Path})
			}
		}
		prefix += segment
	}
	return requires, seeds
}

// isParam reports whether segment, with its leading slash, is a {param}.
func isParam(segment string) bool {
	return strings.HasPrefix(segment, "/{") && strings.HasSuffix(segment, "}")
}

// bodyRequirements finds the fields of the request body of op that refer to
// another collection ("feedId" -> POST /feeds) and returns one requirement for
// each. It is a best-effort guess from names, so it only counts when exactly
// one POST collection matches; otherwise the field is left to the LLM or to
// whoever edits the plan. Media types and fields are visited in sorted order.
// A field is skipped when it is the operation's own collection, when the same
// producer is already in have (it came by path), or when it was already seen
// under another media type.
func bodyRequirements(op contract.Operation, have []Requirement, producerByPath map[string]contract.Operation, byLastSeg map[string][]string) []Requirement {
	var found []Requirement
	seen := make(map[string]struct{})
	required := make(map[string]struct{})
	for _, req := range have {
		for _, producer := range req.Producers {
			required[producer.Op] = struct{}{}
		}
	}

	for _, media := range slices.Sorted(maps.Keys(op.Bodies)) {
		schema := op.Bodies[media]
		if schema == nil || schema.Type != contract.SchemaTypeObject {
			continue
		}
		for _, field := range slices.Sorted(maps.Keys(schema.Properties)) {
			if _, ok := seen[field]; ok {
				continue
			}
			stem, ok := idStem(field)
			if !ok {
				continue
			}
			candidates := collectionCandidates(stem, op.Path, byLastSeg)
			if len(candidates) != 1 {
				continue
			}
			producer := producerByPath[candidates[0]]
			if _, ok := required[producer.BuildID()]; ok {
				continue
			}
			seen[field] = struct{}{}
			from, match := inferProducerFrom(producer, field)
			found = append(found, Requirement{
				Param:     field,
				In:        Body,
				Producers: []Producer{{Op: producer.BuildID(), From: from, Match: match}},
			})
		}
	}
	return found
}

// idStem cuts the suffix Id, ID or _id from field. It reports false when
// there is no suffix or nothing is left, so a bare "id" is never a reference.
func idStem(field string) (string, bool) {
	for _, suffix := range []string{"_id", "Id", "ID"} {
		if stem, ok := strings.CutSuffix(field, suffix); ok && stem != "" {
			return stem, true
		}
	}
	return "", false
}

// collectionCandidates returns the POST paths whose last segment is stem as
// is, or with s, es or y -> ies, ignoring case. self is never a candidate.
// The result is sorted and has no duplicates.
func collectionCandidates(stem, self string, byLastSeg map[string][]string) []string {
	stem = strings.ToLower(stem)
	names := []string{stem, stem + "s", stem + "es"}
	if base, ok := strings.CutSuffix(stem, "y"); ok {
		names = append(names, base+"ies")
	}
	var candidates []string
	for _, name := range names {
		for _, path := range byLastSeg[name] {
			if path != self {
				candidates = append(candidates, path)
			}
		}
	}
	slices.Sort(candidates)
	return slices.Compact(candidates)
}

// defaultFrom is where the id is read when the producer's response gives no
// better hint.
const defaultFrom = "response.body.id"

// inferProducerFrom says where in the response of producer the value for param
// is, and which rule found it. It reads the properties of the response with
// status 201, then 200, then 2XX (the first one that has properties), and
// tries in this order "id" (MatchID), "<singular collection>Id"
// (MatchCollection, "animals" -> "animalId") and the param's own name
// (MatchParam). Names are compared ignoring case and "_", so animal_id and
// animalID match too. Without a match it is defaultFrom with Unresolved.
func inferProducerFrom(producer contract.Operation, param string) (string, Match) {
	collection := producer.Path[strings.LastIndex(producer.Path, "/")+1:]
	rules := []struct {
		name  string
		match Match
	}{
		{"id", MatchID},
		{singular(collection) + "id", MatchCollection},
		{param, MatchParam},
	}

	for _, status := range []string{"201", "200", "2XX"} {
		props := responseProperties(producer, status)
		if len(props) == 0 {
			continue
		}
		for _, rule := range rules {
			for _, name := range slices.Sorted(maps.Keys(props)) {
				if normalize(name) == normalize(rule.name) {
					return "response.body." + name, rule.match
				}
			}
		}
	}
	return defaultFrom, Unresolved
}

// singular turns a collection name into its singular: "categories" ->
// "category", "animals" -> "animal".
func singular(name string) string {
	if base, ok := strings.CutSuffix(name, "ies"); ok {
		return base + "y"
	}
	return strings.TrimSuffix(name, "s")
}

// responseProperties returns the properties of the body of the response with
// the given status, preferring application/json, or nil when there are none.
func responseProperties(op contract.Operation, status string) map[string]*contract.Schema {
	response := op.Responses[status]
	if response == nil {
		return nil
	}
	media := slices.Sorted(maps.Keys(response.Bodies))
	// application/json first, the rest keep their sorted order.
	if i := slices.Index(media, "application/json"); i > 0 {
		media = slices.Insert(slices.Delete(media, i, i+1), 0, "application/json")
	}
	for _, m := range media {
		if schema := response.Bodies[m]; schema != nil && schema.Type == contract.SchemaTypeObject && len(schema.Properties) > 0 {
			return schema.Properties
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
