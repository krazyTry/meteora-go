package dynamic_bonding_curve

import (
	"testing"

	solanago "github.com/gagliardetto/solana-go"
)

func TestXxx(t *testing.T) {
	NewSwapInstruction(
		SwapParameters{
			AmountIn:         100,
			MinimumAmountOut: 1,
		},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		solanago.PublicKey{},
		nil,
	)
}
