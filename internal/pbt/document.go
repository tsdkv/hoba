package pbt

import (
	"strings"

	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/schema"
)

var Words = []string{"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog"}

type docConfig struct {
	presentPct int
	minNum     int64
	maxNum     int64
	maxTokens  int
}

type DocumentOption func(*docConfig)

func WithFieldProbability(pct int) DocumentOption {
	return func(c *docConfig) { c.presentPct = pct }
}

func WithNumRange(l, r int64) DocumentOption {
	return func(c *docConfig) { c.minNum, c.maxNum = l, r }
}

// -1 (the default) means uncapped.
func WithMaxTokens(n int) DocumentOption {
	return func(c *docConfig) { c.maxTokens = n }
}

func DocumentGenerator(sch *schema.Schema, vocab []string, opts ...DocumentOption) *rapid.Generator[schema.Document] {
	cfg := docConfig{presentPct: 50, minNum: -1000, maxNum: 1000, maxTokens: -1}
	for _, o := range opts {
		o(&cfg)
	}

	word := rapid.SampledFrom(vocab)
	num := rapid.Int64Range(cfg.minNum, cfg.maxNum)

	// CoinFlip shrinks toward true
	absent := CoinFlip(100 - cfg.presentPct)

	return rapid.Custom(func(rt *rapid.T) schema.Document {
		doc := schema.Document{}

		for i, field := range sch.Fields {
			if absent.Draw(rt, "absent:"+field.Name) {
				continue
			}

			switch field.Type {
			case schema.FieldTypeText:
				if field.Tokenizer == "keyword" {
					doc.Text(i, word.Draw(rt, "text:"+field.Name))
				} else if cfg.maxTokens < 0 {
					doc.Text(i, strings.Join(rapid.SliceOf(word).Draw(rt, "text:"+field.Name), " "))
				} else {
					doc.Text(i, strings.Join(rapid.SliceOfN(word, 0, cfg.maxTokens).Draw(rt, "text:"+field.Name), " "))
				}
			case schema.FieldTypeInt64:
				doc.Int64(i, num.Draw(rt, "num:"+field.Name))
			}
		}

		return doc
	})
}
