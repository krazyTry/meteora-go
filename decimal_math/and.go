package decimal_math

import (
	"math/big"

	"github.com/shopspring/decimal"
)

func And(x, y decimal.Decimal) decimal.Decimal {
	return decimal.NewFromBigInt(new(big.Int).And(x.BigInt(), y.BigInt()), 0)
}
