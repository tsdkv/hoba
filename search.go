package hoba

import (
	"github.com/tsdkv/hoba/internal/collector"
	"github.com/tsdkv/hoba/internal/index"
	"github.com/tsdkv/hoba/internal/query"
	"github.com/tsdkv/hoba/schema"
)

func Search(seg *index.MemSegment, qs string, k int) ([]collector.Hit, error) {
	s := seg.Schema()
	var defaults []string
	for _, f := range s.Fields {
		if f.Type == schema.FieldTypeText && f.Indexed {
			defaults = append(defaults, f.Name)
		}
	}
	p, err := query.NewParser(
		s,
		query.Tokenizers(seg.TokenizerManager()),
		query.DefaultFields(defaults...),
	)
	if err != nil {
		return nil, err
	}
	n, err := p.Parse(qs)
	if err != nil {
		return nil, err
	}
	scorer := query.Compile(n, seg)
	if scorer == nil {
		return nil, nil
	}
	top := collector.NewTopK(k)
	for doc := scorer.Advance(); doc != index.Terminated; doc = scorer.Advance() {
		top.Collect(doc, scorer.Score())
	}
	return top.Hits(), nil
}
