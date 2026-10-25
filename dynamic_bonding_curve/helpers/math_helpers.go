package helpers

import (
	"math/big"

	dbcmath "github.com/krazyTry/meteora-go/dynamic_bonding_curve/math"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func Add(a, b *big.Int) *big.Int {
	return dbcmath.Add(a, b)
}

func Sub(a, b *big.Int) (*big.Int, error) {
	return dbcmath.Sub(a, b)
}

func Mul(a, b *big.Int) *big.Int {
	return dbcmath.Mul(a, b)
}

func Div(a, b *big.Int) (*big.Int, error) {
	return dbcmath.Div(a, b)
}

func MulDiv(x, y, denominator *big.Int, rounding shared.Rounding) (*big.Int, error) {
	return dbcmath.MulDiv(x, y, denominator, rounding)
}

func GetInitialLiquidityFromDeltaQuote(quoteAmount, sqrtMinPrice, sqrtPrice *big.Int) (*big.Int, error) {
	return dbcmath.GetInitialLiquidityFromDeltaQuote(quoteAmount, sqrtMinPrice, sqrtPrice)
}

func GetInitialLiquidityFromDeltaBase(baseAmount, sqrtMaxPrice, sqrtPrice *big.Int) (*big.Int, error) {
	return dbcmath.GetInitialLiquidityFromDeltaBase(baseAmount, sqrtMaxPrice, sqrtPrice)
}

func GetDeltaAmountBaseUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round shared.Rounding) (*big.Int, error) {
	return dbcmath.GetDeltaAmountBaseUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
}

// GetDeltaAmountBaseUnsignedUnchecked is the unbounded delta. New code should call the 256 variant.
func GetDeltaAmountBaseUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round shared.Rounding) (*big.Int, error) {
	return dbcmath.GetDeltaAmountBaseUnsigned256(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
}

func GetDeltaAmountQuoteUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round shared.Rounding) (*big.Int, error) {
	return dbcmath.GetDeltaAmountQuoteUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
}

// GetDeltaAmountQuoteUnsignedUnchecked is the unbounded delta. New code should call the 256 variant.
func GetDeltaAmountQuoteUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round shared.Rounding) (*big.Int, error) {
	return dbcmath.GetDeltaAmountQuoteUnsigned256(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
}

func GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountIn *big.Int, baseForQuote bool) (*big.Int, error) {
	return dbcmath.GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountIn, baseForQuote)
}

func GetNextSqrtPriceFromBaseAmountInRoundingUp(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	return dbcmath.GetNextSqrtPriceFromBaseAmountInRoundingUp(sqrtPrice, liquidity, amount)
}

func GetNextSqrtPriceFromQuoteAmountInRoundingDown(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	return dbcmath.GetNextSqrtPriceFromQuoteAmountInRoundingDown(sqrtPrice, liquidity, amount)
}
