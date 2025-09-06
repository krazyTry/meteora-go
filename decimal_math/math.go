package decimal_math

import (
	"math"

	"github.com/shopspring/decimal"
)

func Pow10(n int) decimal.Decimal {
	return decimal.NewFromFloat(math.Pow10(n))
}
