package decimal_math

import (
	"math/big"

	"github.com/shopspring/decimal"
)

func Quo(x, y decimal.Decimal) decimal.Decimal {
	return decimal.NewFromBigInt(new(big.Int).Quo(x.BigInt(), y.BigInt()), 0)
}
