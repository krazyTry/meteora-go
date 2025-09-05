package cp_amm

import (
	"github.com/gagliardetto/solana-go"
)

func PrepareSwapParams(
	swapBaseForQuote bool,
	poolState *Pool,
) (solana.PublicKey, solana.PublicKey, solana.PublicKey, solana.PublicKey) {
	if swapBaseForQuote {
		return poolState.TokenAMint, poolState.TokenBMint, GetTokenProgram(poolState.TokenAFlag), GetTokenProgram(poolState.TokenBFlag)
	} else {
		return poolState.TokenBMint, poolState.TokenAMint, GetTokenProgram(poolState.TokenBFlag), GetTokenProgram(poolState.TokenAFlag)
	}
}
