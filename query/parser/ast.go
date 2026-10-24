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
// Walking the tree is a type switch over those four types.
type Node interface {
	// Pos is the byte offset where the node starts in the query string.
	Pos() int
	// String renders the node in canonical syntax that re-parses to the
	// same tree.
	String() string
	node()
}

// Term is a single word.
type Term struct {
	Start int    // byte offset, including the field prefix
	Field string // "" = default fields
	Text  string // unescaped
}

// Phrase is a quoted string. Text is unescaped; run the field's analyzer
// over it.
type Phrase struct {
	Start int    // byte offset, including the field prefix
	Field string // "" = default fields
	Text  string // unescaped
}

// Boost multiplies its child's score by Factor.
type Boost struct {
	Node   Node
	Factor float64
}

// Clause is a boolean combination of child nodes.
type Clause struct {
	Start    int // byte offset of the first operand, including its + / - / NOT
	Children []OccurNode
}

// OccurNode pairs a child with its role in the parent Clause.
type OccurNode struct {
	Occur Occur
	Node  Node
}

// Pos returns the term's byte offset.
func (t *Term) Pos() int { return t.Start }

// Pos returns the phrase's byte offset.
func (p *Phrase) Pos() int { return p.Start }

// Pos returns the child node's byte offset: a boost adds no text of its own
// before the child.
func (b *Boost) Pos() int { return b.Node.Pos() }

// Pos returns the clause's byte offset.
func (c *Clause) Pos() int { return c.Start }

func (*Term) node()   {}
func (*Phrase) node() {}
func (*Boost) node()  {}
func (*Clause) node() {}
