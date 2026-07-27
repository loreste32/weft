package llmpack

import (
	"fmt"
	"strings"
)

// Expand returns base examples plus paraphrased instruction variants.
// Code outputs stay identical (stable targets for SFT).
func Expand(base []Example) []Example {
	out := make([]Example, 0, len(base)*4)
	for _, e := range base {
		out = append(out, e)
		for i, instr := range paraphrase(e.Instruction) {
			if instr == e.Instruction {
				continue
			}
			ne := e
			ne.ID = fmt.Sprintf("%s_v%d", e.ID, i+1)
			ne.Instruction = instr
			out = append(out, ne)
		}
	}
	return out
}

func paraphrase(s string) []string {
	s = strings.TrimSpace(s)
	variants := []string{
		s,
		"Write Weft code: " + s,
		"Implement in Weft: " + s,
		"Generate a complete Weft script that does this: " + s,
		"Weft only — " + lowercaseFirst(s),
	}
	// de-dupe
	seen := map[string]bool{}
	var out []string
	for _, v := range variants {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func lowercaseFirst(s string) string {
	if s == "" {
		return s
	}
	// keep short phrases natural
	return strings.ToLower(s[:1]) + s[1:]
}

// Stats summarizes the corpus.
type Stats struct {
	Count    int            `json:"count"`
	Expanded int            `json:"expanded,omitempty"`
	Tags     map[string]int `json:"tags"`
	AvgOut   float64        `json:"avg_output_chars"`
}

// ComputeStats returns counts and tag histogram.
func ComputeStats(exs []Example) Stats {
	st := Stats{Count: len(exs), Tags: map[string]int{}}
	var sum int
	for _, e := range exs {
		sum += len(e.Output)
		for _, t := range e.Tags {
			st.Tags[t]++
		}
	}
	if st.Count > 0 {
		st.AvgOut = float64(sum) / float64(st.Count)
	}
	return st
}
