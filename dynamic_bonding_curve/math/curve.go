package math

import (
	"errors"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func requireU64(value *big.Int) (*big.Int, error) {
	if value.Cmp(dbc.U64Max) > 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	return value, nil
}

func requireU128(value *big.Int) (*big.Int, error) {
	if value.Cmp(dbc.U128Max) > 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	return value, nil
}

// CurvePoint is one stored bonding-curve point.
type CurvePoint struct {
	SqrtPrice *big.Int
	Liquidity *big.Int
}

// LiveCurve returns the points of a stored curve up to the first zero point.
// PoolConfig.curve holds MAX_CURVE_POINT entries and the unused ones are zero.
func LiveCurve(curve []CurvePoint) []CurvePoint {
	for i, point := range curve {
		if point.SqrtPrice == nil || point.Liquidity == nil || point.SqrtPrice.Sign() == 0 || point.Liquidity.Sign() == 0 {
			return curve[:i]
		}
	}
	return curve
}

// GetInitialLiquidityFromDeltaQuote returns L = Δb / (√P - √P_min), rounded down, bounded by u128.
func GetInitialLiquidityFromDeltaQuote(quoteAmount, sqrtMinPrice, sqrtPrice *big.Int) (*big.Int, error) {
	priceDelta, err := Sub(sqrtPrice, sqrtMinPrice)
	if err != nil {
		return nil, err
	}
	quoteAmountShifted := new(big.Int).Lsh(quoteAmount, 128)
	liquidity, err := Div(quoteAmountShifted, priceDelta)
	if err != nil {
		return nil, err
	}
	return requireU128(liquidity)
}

// GetInitialLiquidityFromDeltaBase returns L = Δa * √P * √P_max / (√P_max - √P), rounded down.
func GetInitialLiquidityFromDeltaBase(baseAmount, sqrtMaxPrice, sqrtPrice *big.Int) (*big.Int, error) {
	priceDelta, err := Sub(sqrtMaxPrice, sqrtPrice)
	if err != nil {
		return nil, err
	}
	prod := Mul(Mul(baseAmount, sqrtPrice), sqrtMaxPrice)
	return Div(prod, priceDelta)
}

// GetDeltaAmountBaseUnsigned returns Δa bounded by u64.
func GetDeltaAmountBaseUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round dbc.Rounding) (*big.Int, error) {
	amount, err := GetDeltaAmountBaseUnsigned256(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
	if err != nil {
		return nil, err
	}
	return requireU64(amount)
}

// GetDeltaAmountBaseUnsigned256 returns Δa = L * (√P_upper - √P_lower) / (√P_upper * √P_lower), unbounded.
func GetDeltaAmountBaseUnsigned256(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round dbc.Rounding) (*big.Int, error) {
	numerator, err := Sub(upperSqrtPrice, lowerSqrtPrice)
	if err != nil {
		return nil, err
	}
	denominator := Mul(lowerSqrtPrice, upperSqrtPrice)
	return MulDiv(liquidity, numerator, denominator, round)
}

// GetDeltaAmountQuoteUnsigned returns Δb bounded by u64.
func GetDeltaAmountQuoteUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round dbc.Rounding) (*big.Int, error) {
	amount, err := GetDeltaAmountQuoteUnsigned256(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
	if err != nil {
		return nil, err
	}
	return requireU64(amount)
}

// GetDeltaAmountQuoteUnsigned256 returns Δb = L * (√P_upper - √P_lower), unbounded.
func GetDeltaAmountQuoteUnsigned256(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round dbc.Rounding) (*big.Int, error) {
	deltaSqrtPrice, err := Sub(upperSqrtPrice, lowerSqrtPrice)
	if err != nil {
		return nil, err
	}
	prod := Mul(liquidity, deltaSqrtPrice)
	if round == dbc.RoundingUp {
		denominator := new(big.Int).Lsh(big.NewInt(1), dbc.Resolution*2)
		numerator := new(big.Int).Add(prod, new(big.Int).Sub(denominator, big.NewInt(1)))
		return Div(numerator, denominator)
	}
	return new(big.Int).Rsh(prod, dbc.Resolution*2), nil
}

func requireSwapInputs(sqrtPrice, liquidity, amount *big.Int) error {
	if _, err := requireU128(sqrtPrice); err != nil {
		return err
	}
	if _, err := requireU128(liquidity); err != nil {
		return err
	}
	if _, err := requireU64(amount); err != nil {
		return err
	}
	if sqrtPrice.Sign() == 0 {
		return errors.New("sqrt_price must be greater than 0")
	}
	if liquidity.Sign() == 0 {
		return errors.New("liquidity must be greater than 0")
	}
	return nil
}

// GetNextSqrtPriceFromInput is the next sqrt price after amountIn of base (baseForQuote) or quote enters the segment.
func GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountIn *big.Int, baseForQuote bool) (*big.Int, error) {
	if err := requireSwapInputs(sqrtPrice, liquidity, amountIn); err != nil {
		return nil, err
	}
	if baseForQuote {
		return GetNextSqrtPriceFromBaseAmountInRoundingUp(sqrtPrice, liquidity, amountIn)
	}
	return GetNextSqrtPriceFromQuoteAmountInRoundingDown(sqrtPrice, liquidity, amountIn)
}

// GetNextSqrtPriceFromOutput is the next sqrt price after amountOut of quote (baseForQuote) or base leaves the segment.
func GetNextSqrtPriceFromOutput(sqrtPrice, liquidity, amountOut *big.Int, baseForQuote bool) (*big.Int, error) {
	if err := requireSwapInputs(sqrtPrice, liquidity, amountOut); err != nil {
		return nil, err
	}
	if baseForQuote {
		return GetNextSqrtPriceFromQuoteAmountOutRoundingDown(sqrtPrice, liquidity, amountOut)
	}
	return GetNextSqrtPriceFromBaseAmountOutRoundingUp(sqrtPrice, liquidity, amountOut)
}

// GetNextSqrtPriceFromQuoteAmountOutRoundingDown returns √P' = √P - ceil(Δy / L).
func GetNextSqrtPriceFromQuoteAmountOutRoundingDown(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	qAmount := new(big.Int).Lsh(amount, 128)
	numerator := new(big.Int).Add(qAmount, new(big.Int).Sub(liquidity, big.NewInt(1)))
	quotient, err := Div(numerator, liquidity)
	if err != nil {
		return nil, err
	}
	next, err := Sub(sqrtPrice, quotient)
	if err != nil {
		return nil, err
	}
	return requireU128(next)
}

// GetNextSqrtPriceFromBaseAmountOutRoundingUp returns √P' = √P * L / (L - Δx * √P), rounded up.
func GetNextSqrtPriceFromBaseAmountOutRoundingUp(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	if amount.Sign() == 0 {
		return new(big.Int).Set(sqrtPrice), nil
	}
	denominator, err := Sub(liquidity, Mul(amount, sqrtPrice))
	if err != nil {
		return nil, err
	}
	if denominator.Sign() == 0 {
		return nil, errors.New(dbc.ErrTypeCastFailed)
	}
	next, err := MulDiv(liquidity, sqrtPrice, denominator, dbc.RoundingUp)
	if err != nil {
		return nil, err
	}
	return requireU128(next)
}

// GetNextSqrtPriceFromBaseAmountInRoundingUp returns √P' = √P * L / (L + Δx * √P), rounded up.
func GetNextSqrtPriceFromBaseAmountInRoundingUp(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	if amount.Sign() == 0 {
		return new(big.Int).Set(sqrtPrice), nil
	}
	denominator := new(big.Int).Add(liquidity, Mul(amount, sqrtPrice))
	next, err := MulDiv(liquidity, sqrtPrice, denominator, dbc.RoundingUp)
	if err != nil {
		return nil, err
	}
	return requireU128(next)
}

// GetNextSqrtPriceFromQuoteAmountInRoundingDown returns √P' = √P + Δy / L, rounded down.
func GetNextSqrtPriceFromQuoteAmountInRoundingDown(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	quotient := new(big.Int).Lsh(amount, dbc.Resolution*2)
	q, err := Div(quotient, liquidity)
	if err != nil {
		return nil, err
	}
	return requireU128(Add(sqrtPrice, q))
}
