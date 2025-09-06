package decimal_math

import (
	"math"

	"github.com/shopspring/decimal"
)

func Pow1(base, exponent decimal.Decimal) decimal.Decimal {
	// 如果指数是整数
	if exponent.Equal(exponent.Truncate(0)) {
		return base.Pow(exponent)
	}

	// 小数指数：退化为 math.Pow
	baseFloat, _ := base.Float64()
	expFloat, _ := exponent.Float64()
	return decimal.NewFromFloat(math.Pow(baseFloat, expFloat))
}

// 高精度幂函数（整数和小数指数统一）
func Pow(base, exponent decimal.Decimal, scale int32) decimal.Decimal {
	// 零处理
	if base.IsZero() {
		if exponent.IsZero() {
			panic("0^0 = 1")
		}
		return decimal.Zero
	}

	// 处理负数底数
	if base.IsNegative() {
		if !exponent.Equal(exponent.Truncate(0)) {
			panic("negative base with non-integer exponent")
		}
		// 整数指数，直接调用 decimal.Pow
		return base.Pow(exponent)
	}
	if exponent.Cmp(decimal.NewFromInt(1)) < 0 {
		return nth(base, exponent.InexactFloat64(), scale)
	}

	// 整数指数，直接调用 decimal.Pow
	if exponent.Equal(exponent.Truncate(0)) {
		return base.Pow(exponent)
	}

	// -----------------------------
	// 小数指数：使用 exp(ln(base) * exponent)
	// -----------------------------

	// 高精度指数 exp(x)
	expDecimal := func(x decimal.Decimal) decimal.Decimal {
		term := decimal.NewFromInt(1)
		sum := decimal.NewFromInt(1)
		for i := 1; i < 200; i++ {
			term = term.Mul(x).Div(decimal.NewFromInt(int64(i)))
			if term.Abs().LessThan(decimal.New(1, -scale)) {
				break
			}
			sum = sum.Add(term)
		}
		return sum.Round(scale)
	}

	// 高精度自然对数 ln(x)
	lnDecimal := func(x decimal.Decimal) decimal.Decimal {
		if x.LessThanOrEqual(decimal.Zero) {
			panic("ln undefined for <= 0")
		}

		y := decimal.NewFromFloat(0.0)
		epsilon := decimal.New(1, -scale)
		maxIter := 200

		for i := 0; i < maxIter; i++ {
			// f(y) = exp(y) - x, 牛顿迭代 f(y)/f'(y)
			expY := expDecimal(y)
			f := expY.Sub(x)
			fPrime := expY
			next := y.Sub(f.Div(fPrime))
			if next.Sub(y).Abs().LessThan(epsilon) {
				return next.Round(scale)
			}
			y = next
		}
		return y.Round(scale)
	}

	lnBase := lnDecimal(base)

	result := expDecimal(lnBase.Mul(exponent))
	return result.Round(scale)
}

func nth(x decimal.Decimal, y float64, scale int32) decimal.Decimal {

	n := int64(1.0 / y)

	if n <= 0 {
		panic("n must be positive")
	}
	if x.IsNegative() && n%2 == 0 {
		panic("cannot take even root of negative number")
	}
	if x.IsZero() {
		return decimal.Zero
	}

	f, _ := x.Float64()
	initGuess := decimal.NewFromFloat(math.Pow(f, 1/float64(n)))
	// initGuess.Pow()

	guess := initGuess
	last := decimal.Zero

	maxIter := 200
	epsilon := decimal.New(1, -int32(scale)) // 10^-scale

	for i := 0; i < maxIter; i++ {
		// guess = ((n-1)*guess + x/(guess^(n-1))) / n
		guessPow := guess.Pow(decimal.NewFromInt(n - 1))

		if guessPow.IsZero() {
			panic("division by zero in iteration")
		}

		term1 := guess.Mul(decimal.NewFromInt(n - 1))

		term2 := x.DivRound(guessPow, 20)
		next := term1.Add(term2).DivRound(decimal.NewFromInt(n), 20)
		if next.Sub(guess).Abs().LessThan(epsilon) {
			return next.Round(int32(scale))
		}
		last = guess
		guess = next
	}

	return last.Round(int32(scale))
}
