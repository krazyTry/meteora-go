package math

import (
	"errors"
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/krazyTry/meteora-go/damm_v2/helpers"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

func getPoolBig(pool *dammv2gen.Pool) (sqrtPrice, sqrtMin, sqrtMax, liquidity, activationPoint, initSqrtPrice *big.Int) {
	sqrtPrice = new(big.Int).Set(pool.SqrtPrice.BigInt())
	sqrtMin = new(big.Int).Set(pool.SqrtMinPrice.BigInt())
	sqrtMax = new(big.Int).Set(pool.SqrtMaxPrice.BigInt())
	liquidity = new(big.Int).Set(pool.Liquidity.BigInt())
	activationPoint = big.NewInt(int64(pool.ActivationPoint))
	initSqrtPrice = new(big.Int).Set(pool.PoolFees.InitSqrtPrice.BigInt())
	return
}

func GetSwapResultFromExactInput(poolState *dammv2gen.Pool, amountIn *big.Int, feeMode shared.FeeMode, tradeDirection shared.TradeDirection, currentPoint *big.Int) (shared.SwapResult2, error) {
	actualProtocolFee := big.NewInt(0)
	actualTradingFee := big.NewInt(0)
	actualReferralFee := big.NewInt(0)
	actualPartnerFee := big.NewInt(0)

	maxFeeNumerator := GetMaxFeeNumerator(shared.PoolVersion(poolState.Version))

	_, _, _, _, activationPoint, initSqrtPrice := getPoolBig(poolState)

	tradeFeeNumerator, err := GetTotalTradingFeeFromIncludedFeeAmount(poolState.PoolFees, currentPoint, activationPoint, amountIn, tradeDirection, maxFeeNumerator, initSqrtPrice, poolState.SqrtPrice.BigInt())
	if err != nil {
		return shared.SwapResult2{}, err
	}

	actualAmountIn := new(big.Int).Set(amountIn)
	if feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(poolState.PoolFees, amountIn, tradeFeeNumerator, feeMode.HasReferral, hasPartner(poolState))
		if err != nil {
			return shared.SwapResult2{}, err
		}
		actualProtocolFee = feeResult.ProtocolFee
		actualTradingFee = feeResult.TradingFee
		actualReferralFee = feeResult.ReferralFee
		actualPartnerFee = feeResult.PartnerFee
		actualAmountIn = feeResult.AmountAfterFee
	}

	var outputAmount, nextSqrtPrice, amountLeft *big.Int
	if tradeDirection == shared.TradeDirectionAtoB {
		var err error
		outputAmount, nextSqrtPrice, amountLeft, err = calculateAtoBFromAmountIn(poolState, actualAmountIn)
		if err != nil {
			return shared.SwapResult2{}, err
		}
	} else {
		var err error
		outputAmount, nextSqrtPrice, amountLeft, err = calculateBtoAFromAmountIn(poolState, actualAmountIn)
		if err != nil {
			return shared.SwapResult2{}, err
		}
	}

	actualAmountOut := new(big.Int).Set(outputAmount)
	if !feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(poolState.PoolFees, outputAmount, tradeFeeNumerator, feeMode.HasReferral, hasPartner(poolState))
		if err != nil {
			return shared.SwapResult2{}, err
		}
		actualProtocolFee = feeResult.ProtocolFee
		actualTradingFee = feeResult.TradingFee
		actualReferralFee = feeResult.ReferralFee
		actualPartnerFee = feeResult.PartnerFee
		actualAmountOut = feeResult.AmountAfterFee
	}

	return shared.SwapResult2{
		IncludedFeeInputAmount: amountIn.Uint64(),
		ExcludedFeeInputAmount: actualAmountIn.Uint64(),
		AmountLeft:             amountLeft.Uint64(),
		OutputAmount:           actualAmountOut.Uint64(),
		NextSqrtPrice:          u128FromBig(nextSqrtPrice),
		TradingFee:             actualTradingFee.Uint64(),
		ProtocolFee:            actualProtocolFee.Uint64(),
		PartnerFee:             actualPartnerFee.Uint64(),
		ReferralFee:            actualReferralFee.Uint64(),
	}, nil
}

func calculateAtoBFromAmountIn(poolState *dammv2gen.Pool, amountIn *big.Int) (*big.Int, *big.Int, *big.Int, error) {
	sqrtPrice, sqrtMin, _, liquidity, _, _ := getPoolBig(poolState)
	nextSqrtPrice, err := GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountIn, true)
	if err != nil {
		return nil, nil, nil, err
	}
	if nextSqrtPrice.Cmp(sqrtMin) < 0 {
		return nil, nil, nil, errors.New("price range is violated")
	}
	outputAmount := GetAmountBFromLiquidityDelta(nextSqrtPrice, sqrtPrice, liquidity, shared.RoundingDown)
	return outputAmount, nextSqrtPrice, big.NewInt(0), nil
}

func calculateBtoAFromAmountIn(poolState *dammv2gen.Pool, amountIn *big.Int) (*big.Int, *big.Int, *big.Int, error) {
	sqrtPrice, _, sqrtMax, liquidity, _, _ := getPoolBig(poolState)
	nextSqrtPrice, err := GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountIn, false)
	if err != nil {
		return nil, nil, nil, err
	}
	if nextSqrtPrice.Cmp(sqrtMax) > 0 {
		return nil, nil, nil, errors.New("price range is violated")
	}
	outputAmount := GetAmountAFromLiquidityDelta(sqrtPrice, nextSqrtPrice, liquidity, shared.RoundingDown)
	return outputAmount, nextSqrtPrice, big.NewInt(0), nil
}

func GetSwapResultFromPartialInput(poolState *dammv2gen.Pool, amountIn *big.Int, feeMode shared.FeeMode, tradeDirection shared.TradeDirection, currentPoint *big.Int) (shared.SwapResult2, error) {
	actualProtocolFee := big.NewInt(0)
	actualTradingFee := big.NewInt(0)
	actualReferralFee := big.NewInt(0)
	actualPartnerFee := big.NewInt(0)

	maxFeeNumerator := GetMaxFeeNumerator(shared.PoolVersion(poolState.Version))
	_, _, _, _, activationPoint, initSqrtPrice := getPoolBig(poolState)

	tradeFeeNumerator, err := GetTotalTradingFeeFromIncludedFeeAmount(poolState.PoolFees, currentPoint, activationPoint, amountIn, tradeDirection, maxFeeNumerator, initSqrtPrice, poolState.SqrtPrice.BigInt())
	if err != nil {
		return shared.SwapResult2{}, err
	}

	actualAmountIn := new(big.Int).Set(amountIn)
	if feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(poolState.PoolFees, amountIn, tradeFeeNumerator, feeMode.HasReferral, hasPartner(poolState))
		if err != nil {
			return shared.SwapResult2{}, err
		}
		actualProtocolFee = feeResult.ProtocolFee
		actualTradingFee = feeResult.TradingFee
		actualReferralFee = feeResult.ReferralFee
		actualPartnerFee = feeResult.PartnerFee
		actualAmountIn = feeResult.AmountAfterFee
	}

	var amountLeft, outputAmount, nextSqrtPrice *big.Int
	if tradeDirection == shared.TradeDirectionAtoB {
		var err error
		outputAmount, nextSqrtPrice, amountLeft, err = calculateAtoBFromPartialAmountIn(poolState, actualAmountIn)
		if err != nil {
			return shared.SwapResult2{}, err
		}
	} else {
		var err error
		outputAmount, nextSqrtPrice, amountLeft, err = calculateBtoAFromPartialAmountIn(poolState, actualAmountIn)
		if err != nil {
			return shared.SwapResult2{}, err
		}
	}

	includedFeeInputAmount := new(big.Int).Set(amountIn)
	if amountLeft.Sign() > 0 {
		actualAmountIn = new(big.Int).Sub(actualAmountIn, amountLeft)
		if feeMode.FeesOnInput {
			tradeFeeNumerator, err := GetTotalTradingFeeFromExcludedFeeAmount(poolState.PoolFees, currentPoint, activationPoint, actualAmountIn, tradeDirection, maxFeeNumerator, initSqrtPrice, poolState.SqrtPrice.BigInt())
			if err != nil {
				return shared.SwapResult2{}, err
			}
			includedFeeAmount, feeAmount, err := GetIncludedFeeAmount(tradeFeeNumerator, actualAmountIn)
			if err != nil {
				return shared.SwapResult2{}, err
			}
			split := SplitFees(poolState.PoolFees, feeAmount, feeMode.HasReferral, hasPartner(poolState))
			actualProtocolFee = split.ProtocolFee
			actualTradingFee = split.TradingFee
			actualReferralFee = split.ReferralFee
			actualPartnerFee = split.PartnerFee
			includedFeeInputAmount = includedFeeAmount
		} else {
			includedFeeInputAmount = actualAmountIn
		}
	}

	actualAmountOut := new(big.Int).Set(outputAmount)
	if !feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(poolState.PoolFees, outputAmount, tradeFeeNumerator, feeMode.HasReferral, hasPartner(poolState))
		if err != nil {
			return shared.SwapResult2{}, err
		}
		actualProtocolFee = feeResult.ProtocolFee
		actualTradingFee = feeResult.TradingFee
		actualReferralFee = feeResult.ReferralFee
		actualPartnerFee = feeResult.PartnerFee
		actualAmountOut = feeResult.AmountAfterFee
	}

	return shared.SwapResult2{
		IncludedFeeInputAmount: includedFeeInputAmount.Uint64(),
		ExcludedFeeInputAmount: actualAmountIn.Uint64(),
		AmountLeft:             amountLeft.Uint64(),
		OutputAmount:           actualAmountOut.Uint64(),
		NextSqrtPrice:          u128FromBig(nextSqrtPrice),
		TradingFee:             actualTradingFee.Uint64(),
		ProtocolFee:            actualProtocolFee.Uint64(),
		PartnerFee:             actualPartnerFee.Uint64(),
		ReferralFee:            actualReferralFee.Uint64(),
	}, nil
}

func calculateAtoBFromPartialAmountIn(poolState *dammv2gen.Pool, amountIn *big.Int) (*big.Int, *big.Int, *big.Int, error) {
	sqrtPrice, sqrtMin, _, liquidity, _, _ := getPoolBig(poolState)
	maxAmountIn := GetAmountAFromLiquidityDelta(sqrtMin, sqrtPrice, liquidity, shared.RoundingUp)
	consumedIn := new(big.Int)
	nextSqrt := new(big.Int)
	if amountIn.Cmp(maxAmountIn) >= 0 {
		consumedIn.Set(maxAmountIn)
		nextSqrt.Set(sqrtMin)
	} else {
		next, err := GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountIn, true)
		if err != nil {
			return nil, nil, nil, err
		}
		nextSqrt.Set(next)
		consumedIn.Set(amountIn)
	}
	outputAmount := GetAmountBFromLiquidityDelta(nextSqrt, sqrtPrice, liquidity, shared.RoundingDown)
	amountLeft := new(big.Int).Sub(amountIn, consumedIn)
	return outputAmount, nextSqrt, amountLeft, nil
}

func calculateBtoAFromPartialAmountIn(poolState *dammv2gen.Pool, amountIn *big.Int) (*big.Int, *big.Int, *big.Int, error) {
	sqrtPrice, _, sqrtMax, liquidity, _, _ := getPoolBig(poolState)
	maxAmountIn := GetAmountBFromLiquidityDelta(sqrtPrice, sqrtMax, liquidity, shared.RoundingUp)
	consumedIn := new(big.Int)
	nextSqrt := new(big.Int)
	if amountIn.Cmp(maxAmountIn) >= 0 {
		consumedIn.Set(maxAmountIn)
		nextSqrt.Set(sqrtMax)
	} else {
		next, err := GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountIn, false)
		if err != nil {
			return nil, nil, nil, err
		}
		nextSqrt.Set(next)
		consumedIn.Set(amountIn)
	}
	outputAmount := GetAmountAFromLiquidityDelta(sqrtPrice, nextSqrt, liquidity, shared.RoundingDown)
	amountLeft := new(big.Int).Sub(amountIn, consumedIn)
	return outputAmount, nextSqrt, amountLeft, nil
}

func GetSwapResultFromExactOutput(poolState *dammv2gen.Pool, amountOut *big.Int, feeMode shared.FeeMode, tradeDirection shared.TradeDirection, currentPoint *big.Int) (shared.SwapResult2, error) {
	actualProtocolFee := big.NewInt(0)
	actualTradingFee := big.NewInt(0)
	actualReferralFee := big.NewInt(0)
	actualPartnerFee := big.NewInt(0)

	maxFeeNumerator := GetMaxFeeNumerator(shared.PoolVersion(poolState.Version))
	_, _, _, _, activationPoint, initSqrtPrice := getPoolBig(poolState)

	includedFeeAmountOut := new(big.Int).Set(amountOut)
	if !feeMode.FeesOnInput {
		tradeFeeNumerator, err := GetTotalTradingFeeFromExcludedFeeAmount(poolState.PoolFees, currentPoint, activationPoint, amountOut, tradeDirection, maxFeeNumerator, initSqrtPrice, poolState.SqrtPrice.BigInt())
		if err != nil {
			return shared.SwapResult2{}, err
		}
		includedFeeAmount, feeAmount, err := GetIncludedFeeAmount(tradeFeeNumerator, amountOut)
		if err != nil {
			return shared.SwapResult2{}, err
		}
		split := SplitFees(poolState.PoolFees, feeAmount, feeMode.HasReferral, hasPartner(poolState))
		actualTradingFee = split.TradingFee
		actualProtocolFee = split.ProtocolFee
		actualReferralFee = split.ReferralFee
		actualPartnerFee = split.PartnerFee
		includedFeeAmountOut = includedFeeAmount
	}

	var inputAmount, nextSqrtPrice *big.Int
	if tradeDirection == shared.TradeDirectionAtoB {
		var err error
		inputAmount, nextSqrtPrice, err = calculateAtoBFromAmountOut(poolState, includedFeeAmountOut)
		if err != nil {
			return shared.SwapResult2{}, err
		}
	} else {
		var err error
		inputAmount, nextSqrtPrice, err = calculateBtoAFromAmountOut(poolState, includedFeeAmountOut)
		if err != nil {
			return shared.SwapResult2{}, err
		}
	}

	includedFeeInputAmount := new(big.Int).Set(inputAmount)
	if feeMode.FeesOnInput {
		tradeFeeNumerator, err := GetTotalTradingFeeFromExcludedFeeAmount(poolState.PoolFees, currentPoint, activationPoint, inputAmount, tradeDirection, maxFeeNumerator, initSqrtPrice, poolState.SqrtPrice.BigInt())
		if err != nil {
			return shared.SwapResult2{}, err
		}
		includedFeeAmount, feeAmount, err := GetIncludedFeeAmount(tradeFeeNumerator, inputAmount)
		if err != nil {
			return shared.SwapResult2{}, err
		}
		split := SplitFees(poolState.PoolFees, feeAmount, feeMode.HasReferral, hasPartner(poolState))
		actualTradingFee = split.TradingFee
		actualProtocolFee = split.ProtocolFee
		actualReferralFee = split.ReferralFee
		actualPartnerFee = split.PartnerFee
		includedFeeInputAmount = includedFeeAmount
	}

	return shared.SwapResult2{
		AmountLeft:             0,
		IncludedFeeInputAmount: includedFeeInputAmount.Uint64(),
		ExcludedFeeInputAmount: inputAmount.Uint64(),
		OutputAmount:           amountOut.Uint64(),
		NextSqrtPrice:          u128FromBig(nextSqrtPrice),
		TradingFee:             actualTradingFee.Uint64(),
		ProtocolFee:            actualProtocolFee.Uint64(),
		PartnerFee:             actualPartnerFee.Uint64(),
		ReferralFee:            actualReferralFee.Uint64(),
	}, nil
}

func calculateAtoBFromAmountOut(poolState *dammv2gen.Pool, amountOut *big.Int) (*big.Int, *big.Int, error) {
	sqrtPrice, sqrtMin, _, liquidity, _, _ := getPoolBig(poolState)
	nextSqrt, err := GetNextSqrtPriceFromOutput(sqrtPrice, liquidity, amountOut, true)
	if err != nil {
		return nil, nil, err
	}
	if nextSqrt.Cmp(sqrtMin) < 0 {
		return nil, nil, errors.New("price range violation")
	}
	inputAmount := GetAmountAFromLiquidityDelta(nextSqrt, sqrtPrice, liquidity, shared.RoundingUp)
	return inputAmount, nextSqrt, nil
}

func calculateBtoAFromAmountOut(poolState *dammv2gen.Pool, amountOut *big.Int) (*big.Int, *big.Int, error) {
	sqrtPrice, _, sqrtMax, liquidity, _, _ := getPoolBig(poolState)
	nextSqrt, err := GetNextSqrtPriceFromOutput(sqrtPrice, liquidity, amountOut, false)
	if err != nil {
		return nil, nil, err
	}
	if nextSqrt.Cmp(sqrtMax) > 0 {
		return nil, nil, errors.New("price range violation")
	}
	inputAmount := GetAmountBFromLiquidityDelta(sqrtPrice, nextSqrt, liquidity, shared.RoundingUp)
	return inputAmount, nextSqrt, nil
}

func SwapQuoteExactInput(pool *dammv2gen.Pool, currentPoint, amountIn *big.Int, slippage uint16, aToB bool, hasReferral bool, tokenADecimal, tokenBDecimal uint8, inputTokenInfo, outputTokenInfo *helpers.TokenInfo) (shared.Quote2Result, error) {
	if amountIn.Sign() <= 0 {
		return shared.Quote2Result{}, errors.New("amount in must be greater than 0")
	}
	if !isSwapEnabled(pool, currentPoint) {
		return shared.Quote2Result{}, errors.New("swap is disabled")
	}
	tradeDirection := shared.TradeDirectionAtoB
	if !aToB {
		tradeDirection = shared.TradeDirectionBtoA
	}
	feeMode := GetFeeMode(shared.CollectFeeMode(pool.CollectFeeMode), tradeDirection, hasReferral)
	actualAmountIn := new(big.Int).Set(amountIn)
	if inputTokenInfo != nil {
		actualAmountIn = helpers.CalculateTransferFeeExcludedAmount(amountIn, inputTokenInfo).Amount
	}
	swapResult, err := GetSwapResultFromExactInput(pool, actualAmountIn, feeMode, tradeDirection, currentPoint)
	if err != nil {
		return shared.Quote2Result{}, err
	}
	actualAmountOut := new(big.Int).SetUint64(swapResult.OutputAmount)
	if outputTokenInfo != nil {
		actualAmountOut = helpers.CalculateTransferFeeExcludedAmount(new(big.Int).SetUint64(swapResult.OutputAmount), outputTokenInfo).Amount
	}
	minimumAmountOut := helpers.GetAmountWithSlippage(actualAmountOut, slippage, shared.SwapModeExactIn)
	priceImpact, _ := helpers.GetPriceImpact(actualAmountIn, actualAmountOut, pool.SqrtPrice.BigInt(), aToB, tokenADecimal, tokenBDecimal)
	return shared.Quote2Result{SwapResult2: swapResult, MinimumAmountOut: minimumAmountOut, PriceImpact: priceImpact}, nil
}

func SwapQuoteExactOutput(pool *dammv2gen.Pool, currentPoint, amountOut *big.Int, slippage uint16, aToB bool, hasReferral bool, tokenADecimal, tokenBDecimal uint8, inputTokenInfo, outputTokenInfo *helpers.TokenInfo) (shared.Quote2Result, error) {
	if amountOut.Sign() <= 0 {
		return shared.Quote2Result{}, errors.New("amount out must be greater than 0")
	}
	if !isSwapEnabled(pool, currentPoint) {
		return shared.Quote2Result{}, errors.New("swap is disabled")
	}
	tradeDirection := shared.TradeDirectionAtoB
	if !aToB {
		tradeDirection = shared.TradeDirectionBtoA
	}
	feeMode := GetFeeMode(shared.CollectFeeMode(pool.CollectFeeMode), tradeDirection, hasReferral)
	actualAmountOut := new(big.Int).Set(amountOut)
	if outputTokenInfo != nil {
		actualAmountOut = helpers.CalculateTransferFeeIncludedAmount(amountOut, outputTokenInfo).Amount
	}
	swapResult, err := GetSwapResultFromExactOutput(pool, actualAmountOut, feeMode, tradeDirection, currentPoint)
	if err != nil {
		return shared.Quote2Result{}, err
	}
	actualAmountIn := new(big.Int).SetUint64(swapResult.IncludedFeeInputAmount)
	if inputTokenInfo != nil {
		actualAmountIn = helpers.CalculateTransferFeeIncludedAmount(new(big.Int).SetUint64(swapResult.IncludedFeeInputAmount), inputTokenInfo).Amount
	}
	maximumAmountIn := helpers.GetAmountWithSlippage(actualAmountIn, slippage, shared.SwapModeExactOut)
	priceImpact, _ := helpers.GetPriceImpact(actualAmountIn, actualAmountOut, pool.SqrtPrice.BigInt(), aToB, tokenADecimal, tokenBDecimal)
	return shared.Quote2Result{SwapResult2: swapResult, MaximumAmountIn: maximumAmountIn, PriceImpact: priceImpact}, nil
}

func SwapQuotePartialInput(pool *dammv2gen.Pool, currentPoint, amountIn *big.Int, slippage uint16, aToB bool, hasReferral bool, tokenADecimal, tokenBDecimal uint8, inputTokenInfo, outputTokenInfo *helpers.TokenInfo) (shared.Quote2Result, error) {
	if amountIn.Sign() <= 0 {
		return shared.Quote2Result{}, errors.New("amount in must be greater than 0")
	}
	if !isSwapEnabled(pool, currentPoint) {
		return shared.Quote2Result{}, errors.New("swap is disabled")
	}
	tradeDirection := shared.TradeDirectionAtoB
	if !aToB {
		tradeDirection = shared.TradeDirectionBtoA
	}
	feeMode := GetFeeMode(shared.CollectFeeMode(pool.CollectFeeMode), tradeDirection, hasReferral)
	actualAmountIn := new(big.Int).Set(amountIn)
	if inputTokenInfo != nil {
		actualAmountIn = helpers.CalculateTransferFeeExcludedAmount(amountIn, inputTokenInfo).Amount
	}
	swapResult, err := GetSwapResultFromPartialInput(pool, actualAmountIn, feeMode, tradeDirection, currentPoint)
	if err != nil {
		return shared.Quote2Result{}, err
	}
	actualAmountOut := new(big.Int).SetUint64(swapResult.OutputAmount)
	if outputTokenInfo != nil {
		actualAmountOut = helpers.CalculateTransferFeeExcludedAmount(new(big.Int).SetUint64(swapResult.OutputAmount), outputTokenInfo).Amount
	}
	minimumAmountOut := helpers.GetAmountWithSlippage(actualAmountOut, slippage, shared.SwapModePartialFill)
	priceImpact, _ := helpers.GetPriceImpact(actualAmountIn, actualAmountOut, pool.SqrtPrice.BigInt(), aToB, tokenADecimal, tokenBDecimal)
	return shared.Quote2Result{SwapResult2: swapResult, MinimumAmountOut: minimumAmountOut, PriceImpact: priceImpact}, nil
}

func hasPartner(poolState *dammv2gen.Pool) bool {
	return !poolState.Partner.Equals(solanago.PublicKey{})
}

func isSwapEnabled(pool *dammv2gen.Pool, currentPoint *big.Int) bool {
	return pool.PoolStatus == uint8(shared.PoolStatusEnable) && currentPoint.Cmp(big.NewInt(int64(pool.ActivationPoint))) >= 0
}
