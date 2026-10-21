package parser

// Occur is the role of a child node inside a Clause.
type Occur int

const (
	// OccurNone marks a whitespace-separated leaf whose role the parser
	// deliberately leaves undecided; the schema stage resolves it to
	// OccurMust or OccurShould.
	OccurNone Occur = iota
	// OccurShould marks an optional child that only contributes to scoring.
	OccurShould
	// OccurMust marks a required child.
	OccurMust
	// OccurMustNot marks an excluded child.
	OccurMustNot
)

// Node is one AST node produced by Parse: *Term, *Phrase, *Boost or *Clause.
type Node interface{ node() }

// Clause is a boolean combination of child nodes.
type Clause struct {
	Children []OccurNode
}

// OccurNode pairs a child with its role in the parent Clause.
type OccurNode struct {
	Occur Occur
	Node  Node
}

// Term is a single word.
type Term struct {
	Field string // "" = default fields
	Text  string
}

type Phrase struct {
	Field string
	Text  string // unescaped text; run the field tokenizer over it
}

// Boost multiplies its child's score by Factor.
type Boost struct {
	Node   Node
	Factor float64
}

func (*Clause) node() {}
func (*Term) node()   {}
func (*Phrase) node() {}
func (*Boost) node()  {}
