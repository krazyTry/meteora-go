package decimal_math

import (
	"math/big"

	"github.com/shopspring/decimal"
)

func Rsh(x decimal.Decimal, n uint) decimal.Decimal {
	return decimal.NewFromBigInt(
		new(big.Int).Rsh(
			decimal.Decimal(x).BigInt(),
			n,
		),
		0,
	)
}
