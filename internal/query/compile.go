package query

func Compile(q Node, st Stats) Weight {
	switch n := q.(type) {
	case *Term:
		return newTermWeight(st, n.FieldID, n.Text)
	case *Boost:
		if child := Compile(n.Node, st); child != nil {
			return &boostWeight{child: child, factor: n.Factor}
		}
		return nil
	case *MatchAll:
		return matchAllWeight{}
	case *Boolean, *Phrase:
		panic("unimplemented")
	}

	return nil
}
