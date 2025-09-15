package cp_amm

import (
	"math/big"

	"github.com/shopspring/decimal"
)

func GetLiquidityDelta(maxAmountTokenA, maxAmountTokenB, sqrtMaxPrice, sqrtMinPrice, sqrtPrice decimal.Decimal) decimal.Decimal {
	liquidityDeltaFromAmountA := GetLiquidityDeltaFromAmountA(maxAmountTokenA, sqrtPrice, sqrtMaxPrice)

	liquidityDeltaFromAmountB := GetLiquidityDeltaFromAmountB(maxAmountTokenB, sqrtMinPrice, sqrtPrice)

	if liquidityDeltaFromAmountA.Cmp(liquidityDeltaFromAmountB) < 0 {
		return liquidityDeltaFromAmountA.Floor()
	}
	return liquidityDeltaFromAmountB.Floor()
}

// DepositQuote represents a quote for depositing liquidity into a pool
type DepositQuote struct {
	// ActualInputAmount is the actual input amount after deducting fees
	ActualInputAmount *big.Int
	// ConsumedInputAmount is the original input amount before fee deduction
	ConsumedInputAmount *big.Int
	// LiquidityDelta is the amount of liquidity to be added to the pool
	LiquidityDelta *big.Int
	// OutputAmount is the calculated amount of the other token to be deposited
	OutputAmount *big.Int
	// MinOutAmount is the minimum output amount expected
	MinOutAmount *big.Int
}

// GetDepositQuote
func GetDepositQuote(
	poolState *Pool,
	actualAmountIn decimal.Decimal,
	bAddBase bool,
) (decimal.Decimal, decimal.Decimal, error) {

	var (
		liquidityDelta decimal.Decimal
		amountOut      decimal.Decimal
	)

	if bAddBase {
		liquidityDelta = GetLiquidityDeltaFromAmountA(
			actualAmountIn,
			decimal.NewFromBigInt(poolState.SqrtPrice.BigInt(), 0),
			decimal.NewFromBigInt(poolState.SqrtMaxPrice.BigInt(), 0),
		)

		amountOut = GetAmountBFromLiquidityDelta(
			liquidityDelta,
			decimal.NewFromBigInt(poolState.SqrtPrice.BigInt(), 0),
			decimal.NewFromBigInt(poolState.SqrtMinPrice.BigInt(), 0),
			true,
		)

	} else {
		liquidityDelta = GetLiquidityDeltaFromAmountB(
			actualAmountIn,
			decimal.NewFromBigInt(poolState.SqrtMinPrice.BigInt(), 0),
			decimal.NewFromBigInt(poolState.SqrtPrice.BigInt(), 0),
		)
		amountOut = GetAmountAFromLiquidityDelta(
			liquidityDelta,
			decimal.NewFromBigInt(poolState.SqrtPrice.BigInt(), 0),
			decimal.NewFromBigInt(poolState.SqrtMaxPrice.BigInt(), 0),
			true,
		)
	}

	return liquidityDelta, amountOut, nil
}
