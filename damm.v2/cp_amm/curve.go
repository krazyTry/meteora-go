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
func pow(base, exp decimal.Decimal) decimal.Decimal {
	invert := exp.Sign() < 0

	// exp == 0 => return ONE
	if exp.Sign() == 0 {
		return decimal.NewFromBigInt(ONE, 0)
	}

	// 取绝对值
	if invert {
		exp = exp.Abs() // new(big.Int).Abs(exp)
	}

	// 如果过大 => 返回 0
	if exp.Cmp(decimal.NewFromBigInt(MAX_EXPONENTIAL, 0)) > 0 {
		return decimal.Zero //big.NewInt(0)
	}

	squaredBase := base                     // new(big.Int).Set(base)
	result := decimal.NewFromBigInt(ONE, 0) // new(big.Int).Set(ONE)

	// 如果 base >= ONE
	if squaredBase.Cmp(result) >= 0 {
		squaredBase = decimal.NewFromBigInt(MAX, 0).Div(squaredBase) //new(big.Int).Div(MAX, squaredBase)
		invert = !invert
	}

	// 相当于循环 unrolled (展开)
	bitChecks := []int64{
		0x1, 0x2, 0x4, 0x8,
		0x10, 0x20, 0x40, 0x80,
		0x100, 0x200, 0x400, 0x800,
		0x1000, 0x2000, 0x4000, 0x8000,
		0x10000, 0x20000, 0x40000,
	}

	for _, mask := range bitChecks {
		// if exp & mask != 0
		if new(big.Int).And(exp.BigInt(), big.NewInt(mask)).Sign() != 0 {
			// 	tmp := new(big.Int).Mul(result, squaredBase)
			// 	result.Rsh(tmp, SCALE_OFFSET)
			result = decimal.NewFromBigInt(new(big.Int).Rsh(result.Mul(squaredBase).BigInt(), SCALE_OFFSET), 0)
		}

		// squaredBase = (squaredBase * squaredBase) >> SCALE_OFFSET
		// tmp := new(big.Int).Mul(squaredBase, squaredBase)
		// squaredBase.Rsh(tmp, SCALE_OFFSET)
		squaredBase = decimal.NewFromBigInt(new(big.Int).Rsh(squaredBase.Mul(squaredBase).BigInt(), SCALE_OFFSET), 0)
	}

	// 如果结果为 0
	if result.Sign() == 0 {
		return decimal.Zero // big.NewInt(0)
	}

	// 如果 invert == true
	if invert {
		result = decimal.NewFromBigInt(MAX, 0).Div(result) // new(big.Int).Div(MAX, result)
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
	var result decimal.Decimal
	if aToB {
		// product = amount * sqrtPrice
		product := amount.Mul(sqrtPrice)

		// denominator = liquidity + product
		denominator := liquidity.Add(product)

		// numerator = liquidity * sqrtPrice
		numerator := liquidity.Mul(sqrtPrice)

		// (numerator + denominator - 1) / denominator
		result = numerator.Add(denominator.Sub(decimal.NewFromInt(1))).Div(denominator)
	} else {
		// quotient = (amount << (SCALE_OFFSET * 2)) / liquidity
		quotient := decimal.NewFromBigInt(new(big.Int).Lsh(amount.BigInt(), SCALE_OFFSET*2), 0).Div(liquidity)

		result = sqrtPrice.Add(quotient)
	}
	return result
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
func GetAmountAFromLiquidityDelta(liquidity, currentSqrtPrice, maxSqrtPrice decimal.Decimal, roundUp bool) decimal.Decimal {

	product := liquidity.Mul(maxSqrtPrice.Sub(currentSqrtPrice)) // new(big.Int).Mul(liquidity, new(big.Int).Sub(maxSqrtPrice, currentSqrtPrice))
	denominator := currentSqrtPrice.Mul(maxSqrtPrice)            // new(big.Int).Mul(currentSqrtPrice, maxSqrtPrice)

	if roundUp {
		// (product + (denominator-1)) / denominator
		return product.Add(denominator.Sub(decimal.NewFromInt(1))).Div(denominator)
		// return new(big.Int).Div(new(big.Int).Add(product, new(big.Int).Sub(denominator, big.NewInt(1))), denominator)
	}
	return product.Div(denominator) //new(big.Int).Div(product, denominator)
}

// GetAmountBFromLiquidityDelta L = Δb / (√P_upper - √P_lower)
func GetAmountBFromLiquidityDelta(liquidity, currentSqrtPrice, minSqrtPrice decimal.Decimal, roundUp bool) decimal.Decimal {
	one := decimal.NewFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128), 0) // new(big.Int).Lsh(big.NewInt(1), 128)
	deltaPrice := currentSqrtPrice.Sub(minSqrtPrice)                      // new(big.Int).Sub(currentSqrtPrice, minSqrtPrice)
	result := liquidity.Mul(deltaPrice)                                   //new(big.Int).Mul(liquidity, deltaPrice)                     // Q128

	if roundUp {
		// (result + (one-1) ) / one
		return result.Add(one.Sub(decimal.NewFromInt(1))).Div(one)
		// return new(big.Int).Div(new(big.Int).Add(result, new(big.Int).Sub(one, big.NewInt(1))), one)
	}
	return decimal.NewFromBigInt(new(big.Int).Rsh(result.BigInt(), 128), 0)
}

// GetNextSqrtPriceFromAmountBRoundingUp √P' = √P - Δy / L
func getNextSqrtPriceFromAmountBRoundingUp(sqrtPrice, liquidity, amount decimal.Decimal) (decimal.Decimal, error) {
	quotient := decimal.NewFromBigInt(new(big.Int).Lsh(amount.BigInt(), 128), 0).Add(liquidity)
	quotient = quotient.Sub(decimal.NewFromInt(1))
	quotient = quotient.Div(liquidity)

	result := sqrtPrice.Sub(quotient)
	if result.Sign() < 0 {
		return decimal.Decimal{}, errors.New("sqrt price cannot be negative")
	}
	return result, nil
}

// GetNextSqrtPriceFromAmountARoundingDown √P' = √P * L / (L - Δx * √P)
func getNextSqrtPriceFromAmountARoundingDown(sqrtPrice, liquidity, amount decimal.Decimal) (decimal.Decimal, error) {
	if amount.Sign() == 0 {
		return sqrtPrice, nil
	}

	// product := new(big.Int).Mul(amount, sqrtPrice)
	// denominator := new(big.Int).Sub(liquidity, product)
	product := amount.Mul(sqrtPrice)
	denominator := liquidity.Sub(product)

	if denominator.Sign() <= 0 {
		return decimal.Decimal{}, errors.New("invalid denominator in sqrt price calculation")
	}

	// numerator := new(big.Int).Mul(liquidity, sqrtPrice)
	numerator := liquidity.Mul(sqrtPrice)
	// return new(big.Int).Div(numerator, denominator), nil
	return numerator.Div(denominator), nil
}

// GetNextSqrtPriceFromOutput
func getNextSqrtPriceFromOutput(sqrtPrice, liquidity, outAmount decimal.Decimal, isB bool) (decimal.Decimal, error) {
	if sqrtPrice.Sign() == 0 {
		return decimal.Decimal{}, errors.New("sqrt price must be greater than 0")
	}
	if isB {
		return getNextSqrtPriceFromAmountBRoundingUp(sqrtPrice, liquidity, outAmount)
	} else {
		return getNextSqrtPriceFromAmountARoundingDown(sqrtPrice, liquidity, outAmount)
	}
}

// GetMinAmountWithSlippage
func GetMinAmountWithSlippage(amount decimal.Decimal, slippageBps uint64) decimal.Decimal {
	if slippageBps > 0 {

		slippageFactor := decimal.NewFromInt(10000).Sub(decimal.NewFromUint64(slippageBps))
		// denominator = 10000
		denominator := decimal.NewFromInt(10000)

		// minAmountOut = amountOut * slippageFactor / denominator
		minAmountOut := amount.Mul(slippageFactor).Div(denominator)
		amount = minAmountOut
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
func CalculateTransferFeeExcludedAmount(transferFeeConfig *token2022.TransferFeeConfig, transferFeeIncludedAmount decimal.Decimal, mint solana.PublicKey, currentEpoch uint64) (decimal.Decimal, decimal.Decimal, error) {

	if transferFeeConfig == nil {
		return transferFeeIncludedAmount, decimal.Zero, nil
	}

	transferFee := decimal.NewFromBigInt(token2022.CalculateFee(
		token2022.GetEpochFee(transferFeeConfig, currentEpoch),
		transferFeeIncludedAmount.BigInt(),
	), 0)

	// transferFeeExcludedAmount := new(big.Int).Sub(transferFeeIncludedAmount, transferFee)
	return transferFeeIncludedAmount.Sub(transferFee), transferFee, nil
}

func CalculateTransferFeeIncludedAmount(transferFeeConfig *token2022.TransferFeeConfig, transferFeeExcludedAmount decimal.Decimal, mint solana.PublicKey, currentEpoch uint64) (decimal.Decimal, decimal.Decimal, error) {

	if transferFeeExcludedAmount.IsZero() {
		return decimal.Zero, decimal.Zero, nil
	}

	if transferFeeConfig == nil {
		return transferFeeExcludedAmount, decimal.Zero, nil
	}

	epochFee := token2022.GetEpochFee(transferFeeConfig, currentEpoch)

	var transferFee decimal.Decimal
	if epochFee.BasisPoints == MAX_FEE_BASIS_POINTS {
		transferFee = decimal.NewFromUint64(epochFee.MaximumFee)
	} else {
		transferFee = calculateInverseFee(epochFee, transferFeeExcludedAmount)
	}

	// return new(big.Int).Add(transferFeeExcludedAmount, transferFee), transferFee, nil
	return transferFeeExcludedAmount.Add(transferFee), transferFee, nil
}

// calculateInverseFee
func calculateInverseFee(transferFee token2022.TransferFee, postFeeAmount decimal.Decimal) decimal.Decimal {
	preFeeAmount := calculatePreFeeAmount(transferFee, postFeeAmount)
	return decimal.NewFromBigInt(token2022.CalculateFee(transferFee, preFeeAmount.BigInt()), 0)
}

// calculatePreFeeAmount
func calculatePreFeeAmount(transferFee token2022.TransferFee, postFeeAmount decimal.Decimal) decimal.Decimal {
	// if (postFeeAmount.isZero())
	if postFeeAmount.Sign() == 0 {
		return decimal.Zero
	}

	if transferFee.BasisPoints == 0 {
		return postFeeAmount
	}

	maximumFee := decimal.NewFromUint64(transferFee.MaximumFee)

	// if (transferFee.transferFeeBasisPoints === MAX_FEE_BASIS_POINTS)
	if transferFee.BasisPoints == MAX_FEE_BASIS_POINTS {
		// return new(big.Int).Add(postFeeAmount, maximumFee)
		return postFeeAmount.Add(maximumFee)
	}

	// numerator = postFeeAmount * ONE_IN_BASIS_POINTS
	// oneInBasisPoints := ONE_IN_BASIS_POINTS
	oneInBasisPoints := decimal.NewFromBigInt(ONE_IN_BASIS_POINTS, 0)
	// numerator := new(big.Int).Mul(postFeeAmount, oneInBasisPoints)
	numerator := postFeeAmount.Mul(oneInBasisPoints)

	// denominator = ONE_IN_BASIS_POINTS - transferFeeBasisPoints
	// denominator := new(big.Int).Sub(oneInBasisPoints, big.NewInt(int64(transferFee.BasisPoints)))
	denominator := oneInBasisPoints.Sub(decimal.NewFromUint64(uint64(transferFee.BasisPoints)))

	// rawPreFeeAmount = (numerator + denominator - 1) / denominator
	// rawPreFeeAmount := new(big.Int).Add(numerator, denominator)
	rawPreFeeAmount := numerator.Add(denominator)
	// rawPreFeeAmount.Sub(rawPreFeeAmount, big.NewInt(1))
	rawPreFeeAmount = rawPreFeeAmount.Sub(decimal.NewFromInt(1))
	// rawPreFeeAmount.Div(rawPreFeeAmount, denominator)
	rawPreFeeAmount = rawPreFeeAmount.Div(denominator)

	// if (rawPreFeeAmount - postFeeAmount >= maximumFee)
	// diff := new(big.Int).Sub(rawPreFeeAmount, postFeeAmount)
	diff := rawPreFeeAmount.Sub(postFeeAmount)
	if diff.Cmp(maximumFee) >= 0 {
		// return new(big.Int).Add(postFeeAmount, maximumFee)
		return postFeeAmount.Add(maximumFee)
	}

	return rawPreFeeAmount
}
