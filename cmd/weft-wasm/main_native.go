//go:build !js

package main

import "fmt"

func main() {
	fmt.Println("weft-wasm is a browser target. Build with:")
	fmt.Println("  make wasm")
	fmt.Println("  # or: GOOS=js GOARCH=wasm go build -o wasm/weft.wasm ./cmd/weft-wasm")
}
