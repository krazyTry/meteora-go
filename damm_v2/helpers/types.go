package helpers

import (
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
)

// TokenInfo mirrors needed fields for Token2022 fee calculations.
type TokenInfo struct {
	Owner           solanago.PublicKey
	Mint            solanago.PublicKey
	CurrentEpoch    uint64
	Decimals        uint8
	BasisPoints     uint16
	MaximumFee      *big.Int
	HasTransferFee  bool
	HasTransferHook bool
}

// PositionNftAccount represents a position NFT and its token account.
type PositionNftAccount struct {
	PositionNft        solanago.PublicKey
	PositionNftAccount solanago.PublicKey
}
