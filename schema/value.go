package schema

type Kind uint8

const (
	Text Kind = iota
	Int64
)

type Value struct {
	Kind Kind
	Str  string // set when Kind == Text
	Num  int64  // set when Kind == Int64
}
