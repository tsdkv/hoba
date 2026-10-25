package parser

type Occur int

const (
	// OccurNone leaves a whitespace-separated leaf's role to the schema stage.
	OccurNone Occur = iota
	OccurShould
	OccurMust
	OccurMustNot
)

type Node interface {
	Pos() int
	String() string
	node()
}

type Term struct {
	Start int
	Field string
	Text  string
}

type Phrase struct {
	Start int
	Field string
	Text  string
}

type Boost struct {
	Node   Node
	Factor float64
}

type Clause struct {
	Start    int
	Children []OccurNode
}

type OccurNode struct {
	Occur Occur
	Node  Node
}

func (t *Term) Pos() int { return t.Start }

func (p *Phrase) Pos() int { return p.Start }

// A boost adds no text of its own, so Pos is the child's.
func (b *Boost) Pos() int { return b.Node.Pos() }

func (c *Clause) Pos() int { return c.Start }

func (*Term) node()   {}
func (*Phrase) node() {}
func (*Boost) node()  {}
func (*Clause) node() {}
