package parser

type Occur int

const (
	OccurNone Occur = iota
	OccurShould
	OccurMust
	OccurMustNot
)

type Node interface{ node() }

type Clause struct {
	Children []OccurNode
}

type OccurNode struct {
	Occur Occur
	Node  Node
}

type Term struct {
	Field string // "" = default fields
	Text  string
}

type Phrase struct {
	Field string
	Text  string // raw text; run the field tokenizer over it
}

type Boost struct {
	Node   Node
	Factor float64
}

func (*Clause) node() {}
func (*Term) node()   {}
func (*Phrase) node() {}
func (*Boost) node()  {}
