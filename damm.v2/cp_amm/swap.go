package cp_amm

import (
	"github.com/gagliardetto/solana-go"
)

func PrepareSwapParams(
	swapBaseForQuote bool,
	virtualPool *Pool,
) (solana.PublicKey, solana.PublicKey, solana.PublicKey, solana.PublicKey) {
	if swapBaseForQuote {
		return virtualPool.TokenAMint, virtualPool.TokenBMint, GetTokenProgram(virtualPool.TokenAFlag), GetTokenProgram(virtualPool.TokenBFlag)
	} else {
		return virtualPool.TokenBMint, virtualPool.TokenAMint, GetTokenProgram(virtualPool.TokenBFlag), GetTokenProgram(virtualPool.TokenAFlag)
	}
}
