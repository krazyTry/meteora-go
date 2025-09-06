package decimal_math

import (
	"math/big"

	"github.com/shopspring/decimal"
)

func Sqrt(x decimal.Decimal, prec uint) decimal.Decimal {
	if x.Sign() < 0 {
		panic("sqrt on negative decimal")
	}

	out, _ := decimal.NewFromString(
		new(big.Float).SetPrec(prec).Sqrt(
			x.BigFloat().SetPrec(prec),
		).Text('f', -1),
	)
	return out
}
