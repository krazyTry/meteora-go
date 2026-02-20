package math

import (
	"math/big"

	binary "github.com/gagliardetto/binary"
)

func u128FromBig(v *big.Int) binary.Uint128 {
	if v == nil {
		return binary.Uint128{}
	}
	lo := new(big.Int).And(v, new(big.Int).SetUint64(^uint64(0))).Uint64()
	hi := new(big.Int).Rsh(new(big.Int).Set(v), 64).Uint64()
	return binary.Uint128{Lo: lo, Hi: hi}
}
