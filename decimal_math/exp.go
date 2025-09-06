package decimal_math

import (
	"math/big"

	"github.com/shopspring/decimal"
)

func Exp(x, y decimal.Decimal, m decimal.NullDecimal) decimal.Decimal {
	var mm *big.Int
	if m.Valid {
		mm = m.Decimal.BigInt()
	}
	return decimal.NewFromBigInt(new(big.Int).Exp(x.BigInt(), y.BigInt(), mm), 0)
}
