package cp_amm

import (
	"math/big"

	"github.com/shopspring/decimal"
)

func GetLiquidityDelta(maxAmountTokenA *big.Int, maxAmountTokenB *big.Int, sqrtMaxPrice *big.Int, sqrtMinPrice *big.Int, sqrtPrice *big.Int) *big.Int {
	liquidityDeltaFromAmountA := GetLiquidityDeltaFromAmountA(
		decimal.NewFromBigInt(maxAmountTokenA, 0),
		decimal.NewFromBigInt(sqrtPrice, 0),
		decimal.NewFromBigInt(sqrtMaxPrice, 0),
	)

	liquidityDeltaFromAmountB := GetLiquidityDeltaFromAmountB(
		decimal.NewFromBigInt(maxAmountTokenB, 0),
		decimal.NewFromBigInt(sqrtMinPrice, 0),
		decimal.NewFromBigInt(sqrtPrice, 0),
	)

	if liquidityDeltaFromAmountA.Cmp(liquidityDeltaFromAmountB) < 0 {
		return liquidityDeltaFromAmountA.BigInt()
	}
	return liquidityDeltaFromAmountB.BigInt()
}

// DepositQuote
type DepositQuote struct {
	ActualInputAmount   *big.Int // Actual input amount (after deducting fees)
	ConsumedInputAmount *big.Int // Original input amount
	LiquidityDelta      *big.Int // Liquidity to be added to the pool
	OutputAmount        *big.Int // Calculated amount of the other token
	MinOutAmount        *big.Int
}

// GetDepositQuote
func GetDepositQuote(
	virtualPool *Pool,
	actualAmountIn *big.Int,
	bAddBase bool,
) (*big.Int, *big.Int, error) {

	var (
		liquidityDelta decimal.Decimal
		amountOut      *big.Int
	)

	if bAddBase {
		liquidityDelta = GetLiquidityDeltaFromAmountA(decimal.NewFromBigInt(actualAmountIn, 0), decimal.NewFromBigInt(virtualPool.SqrtPrice.BigInt(), 0), decimal.NewFromBigInt(virtualPool.SqrtMaxPrice.BigInt(), 0))
		amountOut = GetAmountBFromLiquidityDelta(liquidityDelta.BigInt(), virtualPool.SqrtPrice.BigInt(), virtualPool.SqrtMinPrice.BigInt(), true)
	} else {
		liquidityDelta = GetLiquidityDeltaFromAmountB(decimal.NewFromBigInt(actualAmountIn, 0), decimal.NewFromBigInt(virtualPool.SqrtMinPrice.BigInt(), 0), decimal.NewFromBigInt(virtualPool.SqrtPrice.BigInt(), 0))
		amountOut = GetAmountAFromLiquidityDelta(liquidityDelta.BigInt(), virtualPool.SqrtPrice.BigInt(), virtualPool.SqrtMaxPrice.BigInt(), true)
	}

	return liquidityDelta.BigInt(), amountOut, nil
}
