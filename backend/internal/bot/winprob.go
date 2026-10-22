package bot

// winProb[a][b] é a probabilidade aproximada de a dupla com a pontos vencer a
// partida contra a dupla com b pontos, entre duplas de mesma força. É o valor
// que o Strong maximiza: ele pesa pontos pelo placar (um ponto vale mais perto
// de 12; a mão de 11 e a de ferro têm regras próprias) em vez de somar pontos
// de mão. Calculada uma vez, por indução de trás para frente, com um modelo
// simples de mão: vence com 50%, a aposta segue a distribuição handStake.
var winProb = buildWinProb()

// handStake: probabilidade de uma mão normal valer 1, 3, 6, 9 ou 12 pontos.
var (
	handStakeValue = [5]int{1, 3, 6, 9, 12}
	handStakeProb  = [5]float64{0.58, 0.24, 0.11, 0.04, 0.03}
)

// elevenWinProb é a chance da dupla de 11 vencer a mão jogada (ela vê as duas
// mãos da dupla ao decidir, então fica um pouco acima de 50%).
const elevenWinProb = 0.55

func buildWinProb() (w [13][13]float64) {
	for a := 12; a >= 0; a-- {
		for b := 12; b >= 0; b-- {
			switch {
			case a == 12:
				w[a][b] = 1
			case b == 12:
				w[a][b] = 0
			case a == 11 && b == 11:
				w[a][b] = 0.5 // mão de ferro: quem vencer a mão vence a partida
			case a == 11: // mão de 11 da dupla A: jogar (vale 3) ou correr (B ganha 1)
				play := elevenWinProb + (1-elevenWinProb)*w[a][min(b+3, 12)]
				run := w[a][b+1]
				w[a][b] = max(play, run)
			case b == 11: // mão de 11 da dupla B: se B joga e perde, A soma 3
				play := (1 - elevenWinProb) * w[min(a+3, 12)][b]
				run := w[a+1][b]
				w[a][b] = min(play, run) // B escolhe o que minimiza a chance de A
			default:
				var v float64
				for i, pts := range handStakeValue {
					win := w[min(a+pts, 12)][b]
					lose := w[a][min(b+pts, 12)]
					v += handStakeProb[i] * (win + lose) / 2
				}
				w[a][b] = v
			}
		}
	}
	return w
}

// utility é o valor de uma mão terminada para a dupla team, dado o placar.
func utility(score [2]int8, team int8) float64 {
	return winProb[score[team]][score[1-team]]
}
