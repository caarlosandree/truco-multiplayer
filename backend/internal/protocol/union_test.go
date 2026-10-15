package protocol

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regrava os arquivos golden")

func TestRenderUnionGolden(t *testing.T) {
	got := RenderUnion()
	path := filepath.Join("testdata", "union.ts.golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("união TS mudou; revise e rode: go test ./internal/protocol -run Golden -update\n%s", got)
	}
}
