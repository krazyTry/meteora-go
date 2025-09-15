package dammV2

import (
	"github.com/gagliardetto/solana-go"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
)

type Pool struct {
	// damm v2 pool state
	*cp_amm.Pool
	// damm v2 pool address
	Address solana.PublicKey
}

type Position struct {
	// damm v2 user position
	Position solana.PublicKey
	// damm v2 user position state
	PositionState *cp_amm.Position
}

type UserPosition struct {
	// damm v2 user position
	Position solana.PublicKey
	// damm v2 user position state
	PositionState *cp_amm.Position
	// damm v2 user position nft account
	PositionNftAccount solana.PublicKey
}

type Vesting struct {
	Vesting      solana.PublicKey
	VestingState *cp_amm.Vesting
}
