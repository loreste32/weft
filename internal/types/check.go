// Package types implements Weft type inference and checking.
//
// Philosophy: annotations are optional. Weft infers types from literals,
// operators, calls, and `:=` bindings. Declared types are checked when present.
package types

// Check is implemented in infer.go (name resolution + type inference).
