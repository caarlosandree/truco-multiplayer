// Comando gen-protocol escreve as uniões TypeScript das mensagens WebSocket e a tabela de cartas.
//
// Uso (a partir de backend/): go run ./cmd/gen-protocol [-out caminho] [-cards caminho]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

func main() {
	out := flag.String("out", "../frontend/src/protocol/union.ts", "arquivo de saída das uniões")
	cards := flag.String("cards", "../frontend/src/protocol/cards.json", "arquivo de saída da tabela de cartas")
	flag.Parse()
	if err := os.WriteFile(*out, []byte(protocol.RenderUnion()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*cards, renderCards(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
