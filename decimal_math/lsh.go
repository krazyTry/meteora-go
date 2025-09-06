package decimal_math

import (
	"math/big"

	"github.com/shopspring/decimal"
)

func Lsh(x decimal.Decimal, n uint) decimal.Decimal {
	return decimal.NewFromBigInt(
		new(big.Int).Lsh(
			decimal.Decimal(x).BigInt(),
			n,
		),
		0,
	)
}
