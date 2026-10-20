package plan

// The known generators and the places a parameter can go.
const (
	LLMGenerated       Generator = "llm"
	ManualGererated    Generator = "manual"
	HeuristicGenerated Generator = "heuristic"

	Path       Location = "path"
	QueryParam Location = "query"
	Header     Location = "header"
	Body       Location = "body"
)

// Generator names who produced a plan, for auditing only. It is free text
// such as "heuristic/0.1" or "manual"; nothing depends on its value.
type Generator string

// Location says where a parameter goes in a request.
type Location string

// Plan is the file that "specter plan" writes and "specter run" reads. It
// holds the dependencies between operations and the order to run them in.
// Everything else about an operation comes from the OpenAPI document.
type Plan struct {
	Version     int                 `json:"version"`
	Generator   Generator           `json:"generator"`
	Spec        Spec                `json:"spec"`
	Order       []string            `json:"order"`
	Operations  map[string]Operator `json:"operations"`
	SeedsNeeded []SeedNeed          `json:"seeds_needed"`
	Cycles      []Cycle             `json:"cycles"`
}

// Spec points to the OpenAPI document the plan was made from. A relative
// Path is resolved from the directory of the plan file. Hash is the SHA-256
// of the document's bytes and detects a stale plan.
type Spec struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// Operator is what the plan says about one operation. The operation's id,
// "METHOD /path", is the key in Plan.Operations. Requires are all needed
// before the operation can run.
type Operator struct {
	Enabled  bool          `json:"enabled"`
	Tags     []string      `json:"tags"`
	Requires []Requirement `json:"requires"`
}

// Producer is an operation that can supply a value, and where to find the
// value in its response (for example "response.body.id").
type Producer struct {
	Op   string `json:"op"`
	From string `json:"from"`
}

// Requirement is a parameter an operation needs and the operations that can
// produce it. Any one producer is enough.
type Requirement struct {
	Param     string     `json:"param"`
	In        Location   `json:"in"`
	Producers []Producer `json:"producers"`
}

// SeedNeed is a requirement that no operation produces, so its value must
// be supplied by the user.
type SeedNeed struct {
	Op    string   `json:"op"`
	Param string   `json:"param"`
	In    Location `json:"in"`
}

// Cycle lists the operations in a dependency cycle and the edge that was
// dropped to break it.
type Cycle struct {
	Ops     []string `json:"ops"`
	Dropped Edge     `json:"dropped"`
}

// Edge is a dependency from the operation that produces a value to the one
// that consumes it.
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Param string `json:"param"`
}
