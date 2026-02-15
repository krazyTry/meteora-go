package math

import (
	"math/big"

	bin "github.com/gagliardetto/binary"
)

func U128ToBig(v bin.Uint128) *big.Int {
	return v.BigInt()
}
