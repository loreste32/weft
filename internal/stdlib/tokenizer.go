package stdlib

import (
	"strings"
	"unicode/utf8"

	"github.com/loreste/weft/internal/runtime"
)

// packageTokenizer: BPE-style tokenizer for prompt fitting, chunking, and token counting.
//
//	tok := tokenizer.new()                    // default whitespace+punct splitter
//	tok := tokenizer.bpe(merges)              // trained BPE merges
//	tokens := tokenizer.encode(tok, text)     // -> [str]
//	text := tokenizer.decode(tok, tokens)     // -> str
//	n := tokenizer.count(tok, text)           // -> int
//	chunks := tokenizer.chunk(text, max_tokens, tok?)  // split text into ≤max_tokens pieces
//	tokenizer.estimate(text) -> int           // fast ~tokens/4 estimate (no tokenizer needed)
func packageTokenizer() runtime.Value {
	p := pkg()

	// tokenizer.new() -> tokenizer handle (whitespace+punct split)
	set(p, "new", func(args []runtime.Value) (runtime.Value, error) {
		return makeSimpleTokenizer(), nil
	}, 0)

	// tokenizer.bpe(merges) -> BPE tokenizer
	// merges: list of [pair, merged] like [["h","e","he"], ...]
	set(p, "bpe", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("tokenizer.bpe(merges_list)", "tokenizer"), nil
		}
		return makeBPETokenizer(args[0]), nil
	}, 1)

	// tokenizer.encode(tok, text) -> [str]
	set(p, "encode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("tokenizer.encode(tok, text)", "tokenizer"), nil
		}
		return tokEncode(args[0], args[1].String()), nil
	}, 2)

	// tokenizer.decode(tok, tokens) -> str
	set(p, "decode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[1].Kind != runtime.KindList {
			return errRes("tokenizer.decode(tok, tokens)", "tokenizer"), nil
		}
		items := args[1].Obj.(*runtime.ListObj).Items
		var b strings.Builder
		for _, it := range items {
			b.WriteString(it.String())
		}
		return runtime.Str(b.String()), nil
	}, 2)

	// tokenizer.count(tok, text) -> int
	set(p, "count", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("tokenizer.count(tok, text)", "tokenizer"), nil
		}
		tokens := tokEncode(args[0], args[1].String())
		return runtime.Int(int64(len(tokens.Obj.(*runtime.ListObj).Items))), nil
	}, 2)

	// tokenizer.estimate(text) -> int  (fast ~tokens estimate, no tokenizer needed)
	// Rule of thumb: ~1 token per 4 chars for English
	set(p, "estimate", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Int(0), nil
		}
		n := utf8.RuneCountInString(args[0].String())
		est := (n + 3) / 4 // ceil(n/4)
		if est < 1 && n > 0 {
			est = 1
		}
		return runtime.Int(int64(est)), nil
	}, 1)

	// tokenizer.chunk(text, max_tokens, tok?) -> [str]
	// Split text into pieces each ≤ max_tokens
	set(p, "chunk", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("tokenizer.chunk(text, max_tokens, tok?)", "tokenizer"), nil
		}
		text := args[0].String()
		maxTok, err := runtime.AsInt(args[1])
		if err != nil || maxTok <= 0 {
			return errRes("max_tokens must be positive int", "tokenizer"), nil
		}
		var tok runtime.Value
		if len(args) >= 3 {
			tok = args[2]
		} else {
			tok = makeSimpleTokenizer()
		}
		chunks := chunkText(tok, text, int(maxTok))
		items := make([]runtime.Value, len(chunks))
		for i, c := range chunks {
			items[i] = runtime.Str(c)
		}
		return runtime.List(items...), nil
	}, 3)

	// tokenizer.words(text) -> [str]  simple word tokenization
	set(p, "words", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.List(), nil
		}
		words := simpleTokenize(args[0].String())
		items := make([]runtime.Value, len(words))
		for i, w := range words {
			items[i] = runtime.Str(w)
		}
		return runtime.List(items...), nil
	}, 1)

	return p
}

func makeSimpleTokenizer() runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"_type"}
	mo.Vals["_type"] = runtime.Str("simple")
	return m
}

func makeBPETokenizer(merges runtime.Value) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"_type", "merges"}
	mo.Vals["_type"] = runtime.Str("bpe")
	mo.Vals["merges"] = merges
	return m
}

func tokEncode(tok runtime.Value, text string) runtime.Value {
	tokType := ""
	if tok.Kind == runtime.KindMap {
		if v, ok := tok.Obj.(*runtime.MapObj).Vals["_type"]; ok {
			tokType = v.String()
		}
	}
	var tokens []string
	switch tokType {
	case "bpe":
		merges := tok.Obj.(*runtime.MapObj).Vals["merges"]
		tokens = bpeEncode(text, merges)
	default:
		tokens = simpleTokenize(text)
	}
	items := make([]runtime.Value, len(tokens))
	for i, t := range tokens {
		items[i] = runtime.Str(t)
	}
	return runtime.List(items...)
}

// simpleTokenize splits on whitespace and punctuation boundaries.
func simpleTokenize(text string) []string {
	var tokens []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	for _, r := range text {
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
			tokens = append(tokens, string(r))
		case isPunct(r):
			flush()
			tokens = append(tokens, string(r))
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return tokens
}

func isPunct(r rune) bool {
	switch r {
	case '.', ',', '!', '?', ':', ';', '(', ')', '[', ']', '{', '}',
		'"', '\'', '-', '/', '\\', '@', '#', '$', '%', '&', '*', '+',
		'=', '<', '>', '|', '~', '^', '`':
		return true
	}
	return false
}

// bpeEncode applies BPE merges to text.
func bpeEncode(text string, mergesVal runtime.Value) []string {
	// Start with character-level tokens
	words := simpleTokenize(text)
	var allTokens []string
	for _, word := range words {
		chars := make([]string, 0, len(word))
		for _, r := range word {
			chars = append(chars, string(r))
		}
		// Apply merges
		if mergesVal.Kind == runtime.KindList {
			merges := mergesVal.Obj.(*runtime.ListObj).Items
			for _, merge := range merges {
				if merge.Kind != runtime.KindList {
					continue
				}
				ml := merge.Obj.(*runtime.ListObj).Items
				if len(ml) < 3 {
					continue
				}
				a, b, merged := ml[0].String(), ml[1].String(), ml[2].String()
				chars = applyMerge(chars, a, b, merged)
			}
		}
		allTokens = append(allTokens, chars...)
	}
	return allTokens
}

func applyMerge(tokens []string, a, b, merged string) []string {
	var out []string
	i := 0
	for i < len(tokens) {
		if i+1 < len(tokens) && tokens[i] == a && tokens[i+1] == b {
			out = append(out, merged)
			i += 2
		} else {
			out = append(out, tokens[i])
			i++
		}
	}
	return out
}

// chunkText splits text into pieces each ≤ maxTokens.
func chunkText(tok runtime.Value, text string, maxTokens int) []string {
	// Split by sentences/lines first, then merge
	lines := strings.Split(text, "\n")
	var chunks []string
	var cur strings.Builder
	curCount := 0

	for _, line := range lines {
		lineTokens := tokEncode(tok, line)
		lineCount := len(lineTokens.Obj.(*runtime.ListObj).Items)

		if lineCount > maxTokens {
			// Line itself exceeds limit — split by words
			if cur.Len() > 0 {
				chunks = append(chunks, cur.String())
				cur.Reset()
				curCount = 0
			}
			words := strings.Fields(line)
			for _, w := range words {
				wTokens := tokEncode(tok, w)
				wCount := len(wTokens.Obj.(*runtime.ListObj).Items)
				if curCount+wCount > maxTokens && cur.Len() > 0 {
					chunks = append(chunks, cur.String())
					cur.Reset()
					curCount = 0
				}
				if cur.Len() > 0 {
					cur.WriteByte(' ')
				}
				cur.WriteString(w)
				curCount += wCount
			}
			continue
		}

		if curCount+lineCount > maxTokens && cur.Len() > 0 {
			chunks = append(chunks, cur.String())
			cur.Reset()
			curCount = 0
		}
		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(line)
		curCount += lineCount
	}
	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}
	return chunks
}
