package dammV2

import (
	"github.com/gagliardetto/solana-go"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
)

type Pool struct {
	*cp_amm.Pool
	Address solana.PublicKey
}

type Position struct {
	Position      solana.PublicKey
	PositionState *cp_amm.Position
}

type UserPosition struct {
	Position           solana.PublicKey
	PositionState      *cp_amm.Position
	PositionNftAccount solana.PublicKey
}

type Vesting struct {
	Vesting      solana.PublicKey
	VestingState *cp_amm.Vesting
}
