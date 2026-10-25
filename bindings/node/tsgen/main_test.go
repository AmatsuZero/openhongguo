package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTypesUpToDate 保证提交的 types.d.ts 与 Go 源码一致。
// 不一致时运行 go run ./bindings/node/tsgen > bindings/node/types.d.ts。
func TestTypesUpToDate(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	want, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join("..", "types.d.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatal("bindings/node/types.d.ts is stale: run go run ./bindings/node/tsgen > bindings/node/types.d.ts")
	}
}
