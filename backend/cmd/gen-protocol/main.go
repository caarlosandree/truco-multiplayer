// Comando gen-protocol escreve as uniões TypeScript das mensagens WebSocket.
//
// Uso (a partir de backend/): go run ./cmd/gen-protocol [-out caminho]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

func main() {
	out := flag.String("out", "../frontend/src/protocol/union.ts", "arquivo de saída")
	flag.Parse()
	if err := os.WriteFile(*out, []byte(protocol.RenderUnion()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
