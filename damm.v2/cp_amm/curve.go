package cp_amm

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/krazyTry/meteora-go/solana/token2022"

	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

// pow(base, exponent)
func pow(base decimal.Decimal, exp decimal.Decimal) decimal.Decimal {
	n := exp.IntPart()
	result := decimal.NewFromInt(1)
	var i int64
	for ; i < n; i++ {
		result = result.Mul(base)
	}
	return result
}

func mulDiv(x, y, denominator decimal.Decimal, roundUp bool) (decimal.Decimal, error) {
	if denominator.IsZero() {
		return decimal.Zero, errors.New("MulDiv: division by zero")
	}

	// num = x * y
	num := x.Mul(y)

	// div = num / denominator
	div := num.Div(denominator)
	mod := num.Mod(denominator)

	if roundUp && !mod.IsZero() {
		return div.Add(decimal.NewFromInt(1)), nil
	}

	return div, nil
}

func decimalSqrt(x decimal.Decimal) decimal.Decimal {
	if x.Sign() < 0 {
		panic("sqrt on negative decimal")
	}
	// f, _ := new(big.Float).SetString(x.String())
	// s := new(big.Float).Sqrt(f)
	s := new(big.Float).SetPrec(200).Sqrt(x.BigFloat().SetPrec(200))
	out, _ := decimal.NewFromString(s.Text('f', -1))
	return out
}

func getNextSqrtPrice(amount, sqrtPrice, liquidity decimal.Decimal, aToB bool) decimal.Decimal {
	if aToB {
		// product = amount * sqrtPrice
		product := amount.Mul(sqrtPrice)

		// denominator = liquidity + product
		denominator := liquidity.Add(product)

		// numerator = liquidity * sqrtPrice
		numerator := liquidity.Mul(sqrtPrice)

		// (numerator + denominator - 1) / denominator
		return numerator.Add(denominator.Sub(decimal.NewFromInt(1))).Div(denominator).Ceil()
	} else {
		// quotient = (amount << (SCALE_OFFSET * 2)) / liquidity
		quotient := decimal.NewFromBigInt(new(big.Int).Lsh(amount.BigInt(), SCALE_OFFSET*2), 0)
		quotient = quotient.Div(liquidity)
		return sqrtPrice.Add(quotient)
	}
}

// GetLiquidityDeltaFromAmountA Δa = L * (√P_upper - √P_lower) / (√P_upper * √P_lower)
func GetLiquidityDeltaFromAmountA(amountA, lowerSqrtPrice, upperSqrtPrice decimal.Decimal) decimal.Decimal {
	product := amountA.Mul(lowerSqrtPrice)            //new(big.Int).Mul(amountA, lowerSqrtPrice)
	product = product.Mul(upperSqrtPrice)             //product.Mul(product, upperSqrtPrice)                            // Q128.128
	denominator := upperSqrtPrice.Sub(lowerSqrtPrice) //new(big.Int).Sub(upperSqrtPrice, lowerSqrtPrice) // Q64.64
	return product.Div(denominator)                   //new(big.Int).Div(product, denominator)
}

// GetLiquidityDeltaFromAmountB Δb = L * (√P_upper - √P_lower)
func GetLiquidityDeltaFromAmountB(amountB, lowerSqrtPrice, upperSqrtPrice decimal.Decimal) decimal.Decimal {
	denominator := upperSqrtPrice.Sub(lowerSqrtPrice) //new(big.Int).Sub(upperSqrtPrice, lowerSqrtPrice)
	product := decimal.NewFromBigInt(new(big.Int).Lsh(amountB.BigInt(), 128), 0)
	return product.Div(denominator) //new(big.Int).Div(product, denominator)
}

// GetAmountAFromLiquidityDelta L = Δa * √P_upper * √P_lower / (√P_upper - √P_lower)
func GetAmountAFromLiquidityDelta(liquidity, currentSqrtPrice, maxSqrtPrice *big.Int, roundUp bool) *big.Int {
	product := new(big.Int).Mul(liquidity, new(big.Int).Sub(maxSqrtPrice, currentSqrtPrice))
	denominator := new(big.Int).Mul(currentSqrtPrice, maxSqrtPrice)

	if roundUp {
		// (product + (denominator-1)) / denominator
		return new(big.Int).Div(new(big.Int).Add(product, new(big.Int).Sub(denominator, big.NewInt(1))), denominator)
	}
	return new(big.Int).Div(product, denominator)
}

// GetAmountBFromLiquidityDelta L = Δb / (√P_upper - √P_lower)
func GetAmountBFromLiquidityDelta(liquidity, currentSqrtPrice, minSqrtPrice *big.Int, roundUp bool) *big.Int {
	one := new(big.Int).Lsh(big.NewInt(1), 128)
	deltaPrice := new(big.Int).Sub(currentSqrtPrice, minSqrtPrice)
	result := new(big.Int).Mul(liquidity, deltaPrice) // Q128

	if roundUp {
		// (result + (one-1) ) / one
		return new(big.Int).Div(new(big.Int).Add(result, new(big.Int).Sub(one, big.NewInt(1))), one)
	}
	return new(big.Int).Rsh(result, 128)
}

// GetNextSqrtPriceFromAmountBRoundingUp √P' = √P - Δy / L
func getNextSqrtPriceFromAmountBRoundingUp(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	quotient := new(big.Int).Add(new(big.Int).Lsh(amount, 128), liquidity)
	quotient.Sub(quotient, big.NewInt(1))
	quotient.Div(quotient, liquidity)

	result := new(big.Int).Sub(sqrtPrice, quotient)
	if result.Sign() < 0 {
		return nil, errors.New("sqrt price cannot be negative")
	}
	return result, nil
}

// GetNextSqrtPriceFromAmountARoundingDown √P' = √P * L / (L - Δx * √P)
func getNextSqrtPriceFromAmountARoundingDown(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	if amount.Sign() == 0 {
		return new(big.Int).Set(sqrtPrice), nil
	}

	product := new(big.Int).Mul(amount, sqrtPrice)
	denominator := new(big.Int).Sub(liquidity, product)

	if denominator.Sign() <= 0 {
		return nil, errors.New("invalid denominator in sqrt price calculation")
	}

	numerator := new(big.Int).Mul(liquidity, sqrtPrice)
	return new(big.Int).Div(numerator, denominator), nil
}

// GetNextSqrtPriceFromOutput
func getNextSqrtPriceFromOutput(sqrtPrice, liquidity, outAmount *big.Int, isB bool) (*big.Int, error) {
	if sqrtPrice.Sign() == 0 {
		return nil, errors.New("sqrt price must be greater than 0")
	}
	if isB {
		return getNextSqrtPriceFromAmountBRoundingUp(sqrtPrice, liquidity, outAmount)
	} else {
		return getNextSqrtPriceFromAmountARoundingDown(sqrtPrice, liquidity, outAmount)
	}
}

// GetMinAmountWithSlippage
func GetMinAmountWithSlippage(amount *big.Int, slippageBps uint64) *big.Int {
	if slippageBps > 0 {

		slippageFactor := decimal.NewFromInt(10000).Sub(decimal.NewFromInt(int64(slippageBps)))
		// denominator = 10000
		denominator := decimal.NewFromInt(10000)

		// minAmountOut = amountOut * slippageFactor / denominator
		minAmountOut := decimal.NewFromBigInt(amount, 0).Mul(slippageFactor).Div(denominator)
		amount = minAmountOut.BigInt()
	}
	return amount
}

// GetPriceFromSqrtPrice
// (sqrtPrice^2 * 10^(tokenADecimal - tokenBDecimal)) / 2^128
func getPriceFromSqrtPrice(sqrtPrice decimal.Decimal, tokenADecimal, tokenBDecimal uint8) decimal.Decimal {

	// (sqrtPrice)^2
	price := sqrtPrice.Mul(sqrtPrice)

	// * 10^(tokenADecimal - tokenBDecimal)
	expDiff := int64(tokenADecimal) - int64(tokenBDecimal)
	if expDiff != 0 {
		power := decimal.New(1, int32(expDiff)) // 10^expDiff
		price = price.Mul(power)
	}

	// / 2^128
	denominator := new(big.Int).Lsh(big.NewInt(1), 128) // 2^128
	price = price.Div(decimal.NewFromBigInt(denominator, 0))

	return price
}

// getSqrtPriceFromPrice computes sqrt(price / 10^(tokenADecimal - tokenBDecimal)) * 2^64
func GetSqrtPriceFromPrice(price string, tokenADecimal, tokenBDecimal uint8) (*big.Int, error) {
	decimalPrice, ok := new(big.Float).SetString(price)
	if !ok {
		return nil, fmt.Errorf("invalid price: %s", price)
	}

	// 计算 10^(tokenADecimal - tokenBDecimal)
	decDiff := tokenADecimal - tokenBDecimal
	pow10 := new(big.Float).SetFloat64(math.Pow10(int(decDiff)))

	// price / 10^(diff)
	adjustedByDecimals := new(big.Float).Quo(decimalPrice, pow10)

	// sqrt(adjustedByDecimals)
	sqrtValue := new(big.Float).Sqrt(adjustedByDecimals)

	// sqrtValue * 2^64
	scale := new(big.Float).SetInt(new(big.Int).Lsh(big.NewInt(1), 64))
	sqrtValueQ64 := new(big.Float).Mul(sqrtValue, scale)

	// floor
	result := new(big.Int)
	sqrtValueQ64.Int(result)

	return result, nil
}

// CalculateTransferFeeExcludedAmount
func CalculateTransferFeeExcludedAmount(transferFeeConfig *token2022.TransferFeeConfig, transferFeeIncludedAmount *big.Int, mint solana.PublicKey, currentEpoch uint64) (*big.Int, *big.Int, error) {

	if transferFeeConfig == nil {
		return transferFeeIncludedAmount, big.NewInt(0), nil
	}

	transferFee := token2022.CalculateFee(
		token2022.GetEpochFee(transferFeeConfig, currentEpoch),
		transferFeeIncludedAmount,
	)

	transferFeeExcludedAmount := new(big.Int).Sub(transferFeeIncludedAmount, transferFee)
	return transferFeeExcludedAmount, transferFee, nil
}

func CalculateTransferFeeIncludedAmount(transferFeeConfig *token2022.TransferFeeConfig, transferFeeExcludedAmount *big.Int, mint solana.PublicKey, currentEpoch uint64) (*big.Int, *big.Int, error) {

	if transferFeeExcludedAmount.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0), big.NewInt(0), nil
	}

	if transferFeeConfig == nil {
		return transferFeeExcludedAmount, big.NewInt(0), nil
	}

	epochFee := token2022.GetEpochFee(transferFeeConfig, currentEpoch)

	var transferFee *big.Int
	if epochFee.BasisPoints == MAX_FEE_BASIS_POINTS {
		transferFee = new(big.Int).SetUint64(epochFee.MaximumFee)
	} else {
		transferFee = calculateInverseFee(epochFee, transferFeeExcludedAmount)
	}

	return new(big.Int).Add(transferFeeExcludedAmount, transferFee), transferFee, nil
}

// calculateInverseFee
func calculateInverseFee(transferFee token2022.TransferFee, postFeeAmount *big.Int) *big.Int {
	preFeeAmount := calculatePreFeeAmount(transferFee, postFeeAmount)
	return token2022.CalculateFee(transferFee, preFeeAmount)
}

// calculatePreFeeAmount
func calculatePreFeeAmount(transferFee token2022.TransferFee, postFeeAmount *big.Int) *big.Int {
	// if (postFeeAmount.isZero())
	if postFeeAmount.Sign() == 0 {
		return big.NewInt(0)
	}

	if transferFee.BasisPoints == 0 {
		return new(big.Int).Set(postFeeAmount)
	}

	maximumFee := big.NewInt(int64(transferFee.MaximumFee))

	// if (transferFee.transferFeeBasisPoints === MAX_FEE_BASIS_POINTS)
	if transferFee.BasisPoints == MAX_FEE_BASIS_POINTS {
		return new(big.Int).Add(postFeeAmount, maximumFee)
	}

	// numerator = postFeeAmount * ONE_IN_BASIS_POINTS
	oneInBasisPoints := ONE_IN_BASIS_POINTS
	numerator := new(big.Int).Mul(postFeeAmount, oneInBasisPoints)

	// denominator = ONE_IN_BASIS_POINTS - transferFeeBasisPoints
	denominator := new(big.Int).Sub(oneInBasisPoints, big.NewInt(int64(transferFee.BasisPoints)))

	// rawPreFeeAmount = (numerator + denominator - 1) / denominator
	rawPreFeeAmount := new(big.Int).Add(numerator, denominator)
	rawPreFeeAmount.Sub(rawPreFeeAmount, big.NewInt(1))
	rawPreFeeAmount.Div(rawPreFeeAmount, denominator)

	// if (rawPreFeeAmount - postFeeAmount >= maximumFee)
	diff := new(big.Int).Sub(rawPreFeeAmount, postFeeAmount)
	if diff.Cmp(maximumFee) >= 0 {
		return new(big.Int).Add(postFeeAmount, maximumFee)
	}

	return rawPreFeeAmount
}
