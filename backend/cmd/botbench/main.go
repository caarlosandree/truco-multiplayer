// Comando botbench mede a força do bot forte contra o simples (Spec 5 §5.5):
// joga N partidas com seeds fixas, imprime as vitórias da dupla forte e sai com
// código 1 se ficarem abaixo de 75%.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/bot"
)

func main() {
	games := flag.Int("games", 400, "número de partidas")
	seed := flag.Uint64("seed", 1, "primeira seed (usa seed..seed+games-1)")
	flag.Parse()
	if *games < 1 {
		fmt.Fprintln(os.Stderr, "botbench: -games deve ser >= 1")
		os.Exit(2)
	}

	strong, simple := bot.DefaultStrong(), bot.Simple{}
	start := time.Now()
	wins := 0
	for s := *seed; s < *seed+uint64(*games); s++ {
		if bot.PlayVersus(strong, simple, s) {
			wins++
		}
	}
	min := (*games*3 + 3) / 4 // 75%, arredondado para cima
	fmt.Printf("bot forte venceu %d de %d partidas contra o simples (meta: >= %d, seeds %d..%d, %s)\n",
		wins, *games, min, *seed, *seed+uint64(*games)-1, time.Since(start).Round(time.Second))
	if wins < min {
		fmt.Fprintf(os.Stderr, "botbench: FALHA: %d vitórias < meta de %d\n", wins, min)
		os.Exit(1)
	}
}
