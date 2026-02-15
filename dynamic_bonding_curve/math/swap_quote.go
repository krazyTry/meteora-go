package math

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/math/pool_fees"
	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

type curvePoint struct {
	SqrtPrice *big.Int
	Liquidity *big.Int
}

func curveFromConfig(config *dbc.PoolConfig) []curvePoint {
	curve := make([]curvePoint, 0, len(config.Curve))
	for _, c := range config.Curve {
		curve = append(curve, curvePoint{SqrtPrice: U128ToBig(c.SqrtPrice), Liquidity: U128ToBig(c.Liquidity)})
	}
	return curve
}

// SwapQuote V1
func GetSwapResult(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, amountIn *big.Int, feeMode dbc.FeeMode, tradeDirection dbc.TradeDirection, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapResult, error) {
	var actualProtocolFee = big.NewInt(0)
	var actualTradingFee = big.NewInt(0)
	var actualReferralFee = big.NewInt(0)

	baseFeeHandler, err := pool_fees.GetBaseFeeHandler(new(big.Int).SetUint64(config.PoolFees.BaseFee.CliffFeeNumerator), config.PoolFees.BaseFee.FirstFactor, new(big.Int).SetUint64(config.PoolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(config.PoolFees.BaseFee.ThirdFactor), dbc.BaseFeeMode(config.PoolFees.BaseFee.BaseFeeMode))
	if err != nil {
		return dbc.SwapResult{}, err
	}

	tradeFeeNumerator := baseFeeHandler.GetMinBaseFeeNumerator()
	if !eligibleForFirstSwapWithMinFee {
		tradeFeeNumerator, err = GetTotalFeeNumeratorFromIncludedFeeAmount(config.PoolFees, virtualPool.VolatilityTracker, currentPoint, new(big.Int).SetUint64(virtualPool.ActivationPoint), amountIn, tradeDirection)
		if err != nil {
			return dbc.SwapResult{}, err
		}
	}

	actualAmountIn := amountIn
	if feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(tradeFeeNumerator, amountIn, config.PoolFees, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult{}, err
		}
		actualProtocolFee = feeResult.ProtocolFee
		actualTradingFee = feeResult.TradingFee
		actualReferralFee = feeResult.ReferralFee
		actualAmountIn = feeResult.Amount
	}

	currentSqrtPrice := U128ToBig(virtualPool.SqrtPrice)
	var swapAmount dbc.SwapAmount
	if tradeDirection == dbc.TradeDirectionBaseToQuote {
		swapAmount, err = CalculateBaseToQuoteFromAmountIn(config, currentSqrtPrice, actualAmountIn)
	} else {
		swapAmount, err = CalculateQuoteToBaseFromAmountIn(config, currentSqrtPrice, actualAmountIn, U128ToBig(config.MigrationSqrtPrice))
	}
	if err != nil {
		return dbc.SwapResult{}, err
	}

	if swapAmount.AmountLeft.Sign() != 0 {
		return dbc.SwapResult{}, errors.New("Insufficient Liquidity")
	}

	actualAmountOut := swapAmount.OutputAmount
	if !feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(tradeFeeNumerator, swapAmount.OutputAmount, config.PoolFees, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult{}, err
		}
		actualTradingFee = feeResult.TradingFee
		actualProtocolFee = feeResult.ProtocolFee
		actualReferralFee = feeResult.ReferralFee
		actualAmountOut = feeResult.Amount
	}

	return dbc.SwapResult{
		ActualInputAmount: actualAmountIn,
		OutputAmount:      actualAmountOut,
		NextSqrtPrice:     swapAmount.NextSqrtPrice,
		TradingFee:        actualTradingFee,
		ProtocolFee:       actualProtocolFee,
		ReferralFee:       actualReferralFee,
	}, nil
}

func SwapQuote(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, swapBaseForQuote bool, amountIn *big.Int, slippageBps uint16, hasReferral bool, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapQuoteResult, error) {
	if new(big.Int).SetUint64(virtualPool.QuoteReserve).Cmp(new(big.Int).SetUint64(config.MigrationQuoteThreshold)) >= 0 {
		return dbc.SwapQuoteResult{}, errors.New("Virtual pool is completed")
	}
	if amountIn.Sign() == 0 {
		return dbc.SwapQuoteResult{}, errors.New("Amount is zero")
	}

	tradeDirection := dbc.TradeDirectionQuoteToBase
	if swapBaseForQuote {
		tradeDirection = dbc.TradeDirectionBaseToQuote
	}
	feeMode := GetFeeMode(dbc.CollectFeeMode(config.CollectFeeMode), tradeDirection, hasReferral)
	result, err := GetSwapResult(virtualPool, config, amountIn, feeMode, tradeDirection, currentPoint, eligibleForFirstSwapWithMinFee)
	if err != nil {
		return dbc.SwapQuoteResult{}, err
	}

	minimumAmountOut := result.OutputAmount
	if slippageBps > 0 {
		slippageFactor := big.NewInt(int64(10000 - slippageBps))
		denom := big.NewInt(10000)
		minimumAmountOut = new(big.Int).Div(new(big.Int).Mul(result.OutputAmount, slippageFactor), denom)
	}

	return dbc.SwapQuoteResult{SwapResult: result, MinimumAmountOut: minimumAmountOut}, nil
}

// SwapQuote V2

func GetSwapResultFromExactInput(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, amountIn *big.Int, feeMode dbc.FeeMode, tradeDirection dbc.TradeDirection, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapResult2, error) {
	var actualProtocolFee = big.NewInt(0)
	var actualTradingFee = big.NewInt(0)
	var actualReferralFee = big.NewInt(0)

	baseFeeHandler, err := pool_fees.GetBaseFeeHandler(new(big.Int).SetUint64(config.PoolFees.BaseFee.CliffFeeNumerator), config.PoolFees.BaseFee.FirstFactor, new(big.Int).SetUint64(config.PoolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(config.PoolFees.BaseFee.ThirdFactor), dbc.BaseFeeMode(config.PoolFees.BaseFee.BaseFeeMode))
	if err != nil {
		return dbc.SwapResult2{}, err
	}

	tradeFeeNumerator := baseFeeHandler.GetMinBaseFeeNumerator()
	if !eligibleForFirstSwapWithMinFee {
		tradeFeeNumerator, err = GetTotalFeeNumeratorFromIncludedFeeAmount(config.PoolFees, virtualPool.VolatilityTracker, currentPoint, new(big.Int).SetUint64(virtualPool.ActivationPoint), amountIn, tradeDirection)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
	}

	actualAmountIn := amountIn
	if feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(tradeFeeNumerator, amountIn, config.PoolFees, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		actualProtocolFee = feeResult.ProtocolFee
		actualTradingFee = feeResult.TradingFee
		actualReferralFee = feeResult.ReferralFee
		actualAmountIn = feeResult.Amount
	}

	currentSqrtPrice := U128ToBig(virtualPool.SqrtPrice)
	var swapAmount dbc.SwapAmount
	if tradeDirection == dbc.TradeDirectionBaseToQuote {
		swapAmount, err = CalculateBaseToQuoteFromAmountIn(config, currentSqrtPrice, actualAmountIn)
	} else {
		swapAmount, err = CalculateQuoteToBaseFromAmountIn(config, currentSqrtPrice, actualAmountIn, U128ToBig(config.MigrationSqrtPrice))
	}
	if err != nil {
		return dbc.SwapResult2{}, err
	}

	if swapAmount.AmountLeft.Sign() != 0 {
		return dbc.SwapResult2{}, errors.New("Insufficient Liquidity")
	}

	actualAmountOut := swapAmount.OutputAmount
	if !feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(tradeFeeNumerator, swapAmount.OutputAmount, config.PoolFees, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		actualTradingFee = feeResult.TradingFee
		actualProtocolFee = feeResult.ProtocolFee
		actualReferralFee = feeResult.ReferralFee
		actualAmountOut = feeResult.Amount
	}

	return dbc.SwapResult2{
		AmountLeft:             swapAmount.AmountLeft,
		IncludedFeeInputAmount: amountIn,
		ExcludedFeeInputAmount: actualAmountIn,
		OutputAmount:           actualAmountOut,
		NextSqrtPrice:          swapAmount.NextSqrtPrice,
		TradingFee:             actualTradingFee,
		ProtocolFee:            actualProtocolFee,
		ReferralFee:            actualReferralFee,
	}, nil
}

func GetSwapResultFromPartialInput(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, amountIn *big.Int, feeMode dbc.FeeMode, tradeDirection dbc.TradeDirection, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapResult2, error) {
	var actualProtocolFee = big.NewInt(0)
	var actualTradingFee = big.NewInt(0)
	var actualReferralFee = big.NewInt(0)

	baseFeeHandler, err := pool_fees.GetBaseFeeHandler(new(big.Int).SetUint64(config.PoolFees.BaseFee.CliffFeeNumerator), config.PoolFees.BaseFee.FirstFactor, new(big.Int).SetUint64(config.PoolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(config.PoolFees.BaseFee.ThirdFactor), dbc.BaseFeeMode(config.PoolFees.BaseFee.BaseFeeMode))
	if err != nil {
		return dbc.SwapResult2{}, err
	}

	tradeFeeNumerator := baseFeeHandler.GetMinBaseFeeNumerator()
	if !eligibleForFirstSwapWithMinFee {
		tradeFeeNumerator, err = GetTotalFeeNumeratorFromIncludedFeeAmount(config.PoolFees, virtualPool.VolatilityTracker, currentPoint, new(big.Int).SetUint64(virtualPool.ActivationPoint), amountIn, tradeDirection)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
	}

	actualAmountIn := amountIn
	if feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(tradeFeeNumerator, amountIn, config.PoolFees, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		actualProtocolFee = feeResult.ProtocolFee
		actualTradingFee = feeResult.TradingFee
		actualReferralFee = feeResult.ReferralFee
		actualAmountIn = feeResult.Amount
	}

	currentSqrtPrice := U128ToBig(virtualPool.SqrtPrice)
	var swapAmount dbc.SwapAmount
	if tradeDirection == dbc.TradeDirectionBaseToQuote {
		swapAmount, err = CalculateBaseToQuoteFromAmountIn(config, currentSqrtPrice, actualAmountIn)
	} else {
		swapAmount, err = CalculateQuoteToBaseFromAmountIn(config, currentSqrtPrice, actualAmountIn, U128ToBig(config.MigrationSqrtPrice))
	}
	if err != nil {
		return dbc.SwapResult2{}, err
	}

	includedFeeInputAmount := amountIn
	if swapAmount.AmountLeft.Sign() != 0 {
		actualAmountIn, _ = Sub(actualAmountIn, swapAmount.AmountLeft)
		if feeMode.FeesOnInput {
			tradeFeeNumeratorPartial := baseFeeHandler.GetMinBaseFeeNumerator()
			if !eligibleForFirstSwapWithMinFee {
				tradeFeeNumeratorPartial, err = GetTotalFeeNumeratorFromExcludedFeeAmount(config.PoolFees, virtualPool.VolatilityTracker, currentPoint, new(big.Int).SetUint64(virtualPool.ActivationPoint), actualAmountIn, tradeDirection)
				if err != nil {
					return dbc.SwapResult2{}, err
				}
			}
			includedFeeAmount, feeAmount, err := GetIncludedFeeAmount(tradeFeeNumeratorPartial, actualAmountIn)
			if err != nil {
				return dbc.SwapResult2{}, err
			}
			tradingFee, protocolFee, referralFee, err := SplitFees(config.PoolFees, feeAmount, feeMode.HasReferral)
			if err != nil {
				return dbc.SwapResult2{}, err
			}
			actualTradingFee = tradingFee
			actualProtocolFee = protocolFee
			actualReferralFee = referralFee
			includedFeeInputAmount = includedFeeAmount
		} else {
			includedFeeInputAmount = actualAmountIn
		}
	}

	actualAmountOut := swapAmount.OutputAmount
	if !feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(tradeFeeNumerator, swapAmount.OutputAmount, config.PoolFees, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		actualTradingFee = feeResult.TradingFee
		actualProtocolFee = feeResult.ProtocolFee
		actualReferralFee = feeResult.ReferralFee
		actualAmountOut = feeResult.Amount
	}

	return dbc.SwapResult2{
		AmountLeft:             swapAmount.AmountLeft,
		IncludedFeeInputAmount: includedFeeInputAmount,
		ExcludedFeeInputAmount: actualAmountIn,
		OutputAmount:           actualAmountOut,
		NextSqrtPrice:          swapAmount.NextSqrtPrice,
		TradingFee:             actualTradingFee,
		ProtocolFee:            actualProtocolFee,
		ReferralFee:            actualReferralFee,
	}, nil
}

func CalculateBaseToQuoteFromAmountIn(config *dbc.PoolConfig, currentSqrtPrice, amountIn *big.Int) (dbc.SwapAmount, error) {
	curve := curveFromConfig(config)
	totalOutput := big.NewInt(0)
	current := new(big.Int).Set(currentSqrtPrice)
	amountLeft := new(big.Int).Set(amountIn)

	for i := len(curve) - 2; i >= 0; i-- {
		if curve[i].SqrtPrice.Sign() == 0 || curve[i].Liquidity.Sign() == 0 {
			continue
		}
		if curve[i].SqrtPrice.Cmp(current) < 0 {
			maxAmountIn, err := GetDeltaAmountBaseUnsigned(curve[i].SqrtPrice, current, curve[i+1].Liquidity, dbc.RoundingUp)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			if amountLeft.Cmp(maxAmountIn) < 0 {
				nextSqrtPrice, err := GetNextSqrtPriceFromInput(current, curve[i+1].Liquidity, amountLeft, true)
				if err != nil {
					return dbc.SwapAmount{}, err
				}
				outputAmount, err := GetDeltaAmountQuoteUnsigned(nextSqrtPrice, current, curve[i+1].Liquidity, dbc.RoundingDown)
				if err != nil {
					return dbc.SwapAmount{}, err
				}
				totalOutput.Add(totalOutput, outputAmount)
				current = nextSqrtPrice
				amountLeft = big.NewInt(0)
				break
			}
			nextSqrtPrice := curve[i].SqrtPrice
			outputAmount, err := GetDeltaAmountQuoteUnsigned(nextSqrtPrice, current, curve[i+1].Liquidity, dbc.RoundingDown)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			totalOutput.Add(totalOutput, outputAmount)
			current = nextSqrtPrice
			amountLeft.Sub(amountLeft, maxAmountIn)
		}
	}

	if amountLeft.Sign() != 0 {
		nextSqrtPrice, err := GetNextSqrtPriceFromInput(current, curve[0].Liquidity, amountLeft, true)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		if nextSqrtPrice.Cmp(U128ToBig(config.SqrtStartPrice)) < 0 {
			nextSqrtPrice = U128ToBig(config.SqrtStartPrice)
			amountIn2, err := GetDeltaAmountBaseUnsigned(nextSqrtPrice, current, curve[0].Liquidity, dbc.RoundingUp)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			amountLeft, _ = Sub(amountLeft, amountIn2)
		} else {
			amountLeft = big.NewInt(0)
		}
		outputAmount, err := GetDeltaAmountQuoteUnsigned(nextSqrtPrice, current, curve[0].Liquidity, dbc.RoundingDown)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		totalOutput.Add(totalOutput, outputAmount)
		current = nextSqrtPrice
	}

	return dbc.SwapAmount{OutputAmount: totalOutput, NextSqrtPrice: current, AmountLeft: amountLeft}, nil
}

func CalculateQuoteToBaseFromAmountIn(config *dbc.PoolConfig, currentSqrtPrice, amountIn *big.Int, stopSqrtPrice *big.Int) (dbc.SwapAmount, error) {
	curve := curveFromConfig(config)
	if amountIn.Sign() == 0 {
		return dbc.SwapAmount{OutputAmount: big.NewInt(0), NextSqrtPrice: currentSqrtPrice, AmountLeft: big.NewInt(0)}, nil
	}
	current := new(big.Int).Set(currentSqrtPrice)
	amountLeft := new(big.Int).Set(amountIn)
	totalOutput := big.NewInt(0)

	for i := 0; i < len(curve); i++ {
		if curve[i].SqrtPrice.Sign() == 0 || curve[i].Liquidity.Sign() == 0 {
			break
		}
		reference := minBig(stopSqrtPrice, curve[i].SqrtPrice)
		if reference.Cmp(current) > 0 {
			maxAmountIn, err := GetDeltaAmountQuoteUnsigned(current, reference, curve[i].Liquidity, dbc.RoundingUp)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			if amountLeft.Cmp(maxAmountIn) < 0 {
				nextSqrtPrice, err := GetNextSqrtPriceFromInput(current, curve[i].Liquidity, amountLeft, false)
				if err != nil {
					return dbc.SwapAmount{}, err
				}
				outputAmount, err := GetDeltaAmountBaseUnsigned(current, nextSqrtPrice, curve[i].Liquidity, dbc.RoundingDown)
				if err != nil {
					return dbc.SwapAmount{}, err
				}
				totalOutput.Add(totalOutput, outputAmount)
				current = nextSqrtPrice
				amountLeft = big.NewInt(0)
				break
			}
			nextSqrtPrice := reference
			outputAmount, err := GetDeltaAmountBaseUnsigned(current, nextSqrtPrice, curve[i].Liquidity, dbc.RoundingDown)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			totalOutput.Add(totalOutput, outputAmount)
			current = nextSqrtPrice
			amountLeft.Sub(amountLeft, maxAmountIn)
			if nextSqrtPrice.Cmp(stopSqrtPrice) == 0 {
				break
			}
		}
	}

	return dbc.SwapAmount{OutputAmount: totalOutput, NextSqrtPrice: current, AmountLeft: amountLeft}, nil
}

func GetSwapResultFromExactOutput(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, amountOut *big.Int, feeMode dbc.FeeMode, tradeDirection dbc.TradeDirection, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapResult2, error) {
	var actualProtocolFee = big.NewInt(0)
	var actualTradingFee = big.NewInt(0)
	var actualReferralFee = big.NewInt(0)

	baseFeeHandler, err := pool_fees.GetBaseFeeHandler(new(big.Int).SetUint64(config.PoolFees.BaseFee.CliffFeeNumerator), config.PoolFees.BaseFee.FirstFactor, new(big.Int).SetUint64(config.PoolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(config.PoolFees.BaseFee.ThirdFactor), dbc.BaseFeeMode(config.PoolFees.BaseFee.BaseFeeMode))
	if err != nil {
		return dbc.SwapResult2{}, err
	}

	includedFeeOutAmount := amountOut
	if !feeMode.FeesOnInput {
		tradeFeeNumerator := baseFeeHandler.GetMinBaseFeeNumerator()
		if !eligibleForFirstSwapWithMinFee {
			tradeFeeNumerator, err = GetTotalFeeNumeratorFromExcludedFeeAmount(config.PoolFees, virtualPool.VolatilityTracker, currentPoint, new(big.Int).SetUint64(virtualPool.ActivationPoint), amountOut, tradeDirection)
			if err != nil {
				return dbc.SwapResult2{}, err
			}
		}
		var feeAmount *big.Int
		includedFeeOutAmount, feeAmount, err = GetIncludedFeeAmount(tradeFeeNumerator, amountOut)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		tradingFee, protocolFee, referralFee, err := SplitFees(config.PoolFees, feeAmount, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		actualTradingFee = tradingFee
		actualProtocolFee = protocolFee
		actualReferralFee = referralFee
	}

	currentSqrtPrice := U128ToBig(virtualPool.SqrtPrice)
	var swapAmount dbc.SwapAmount
	if tradeDirection == dbc.TradeDirectionBaseToQuote {
		swapAmount, err = CalculateBaseToQuoteFromAmountOut(config, currentSqrtPrice, includedFeeOutAmount)
	} else {
		swapAmount, err = CalculateQuoteToBaseFromAmountOut(config, currentSqrtPrice, includedFeeOutAmount)
	}
	if err != nil {
		return dbc.SwapResult2{}, err
	}

	amountIn := swapAmount.OutputAmount
	if swapAmount.NextSqrtPrice.Cmp(U128ToBig(config.MigrationSqrtPrice)) > 0 {
		return dbc.SwapResult2{}, errors.New("Insufficient Liquidity")
	}

	excludedFeeInputAmount := amountIn
	includedFeeInputAmount := amountIn
	if feeMode.FeesOnInput {
		tradeFeeNumerator := baseFeeHandler.GetMinBaseFeeNumerator()
		if !eligibleForFirstSwapWithMinFee {
			tradeFeeNumerator, err = GetTotalFeeNumeratorFromExcludedFeeAmount(config.PoolFees, virtualPool.VolatilityTracker, currentPoint, new(big.Int).SetUint64(virtualPool.ActivationPoint), amountIn, tradeDirection)
			if err != nil {
				return dbc.SwapResult2{}, err
			}
		}
		var feeAmount *big.Int
		includedFeeInputAmount, feeAmount, err = GetIncludedFeeAmount(tradeFeeNumerator, amountIn)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		tradingFee, protocolFee, referralFee, err := SplitFees(config.PoolFees, feeAmount, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		actualTradingFee = tradingFee
		actualProtocolFee = protocolFee
		actualReferralFee = referralFee
		excludedFeeInputAmount = amountIn
	}

	return dbc.SwapResult2{
		AmountLeft:             big.NewInt(0),
		IncludedFeeInputAmount: includedFeeInputAmount,
		ExcludedFeeInputAmount: excludedFeeInputAmount,
		OutputAmount:           amountOut,
		NextSqrtPrice:          swapAmount.NextSqrtPrice,
		TradingFee:             actualTradingFee,
		ProtocolFee:            actualProtocolFee,
		ReferralFee:            actualReferralFee,
	}, nil
}

func CalculateBaseToQuoteFromAmountOut(config *dbc.PoolConfig, currentSqrtPrice, outAmount *big.Int) (dbc.SwapAmount, error) {
	curve := curveFromConfig(config)
	current := new(big.Int).Set(currentSqrtPrice)
	amountLeft := new(big.Int).Set(outAmount)
	totalAmountIn := big.NewInt(0)

	for i := len(curve) - 2; i >= 0; i-- {
		if curve[i].SqrtPrice.Sign() == 0 || curve[i].Liquidity.Sign() == 0 {
			continue
		}
		if curve[i].SqrtPrice.Cmp(current) < 0 {
			maxAmountOut, err := GetDeltaAmountQuoteUnsigned(curve[i].SqrtPrice, current, curve[i+1].Liquidity, dbc.RoundingDown)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			if amountLeft.Cmp(maxAmountOut) < 0 {
				nextSqrtPrice, err := GetNextSqrtPriceFromOutput(current, curve[i+1].Liquidity, amountLeft, true)
				if err != nil {
					return dbc.SwapAmount{}, err
				}
				inAmount, err := GetDeltaAmountBaseUnsigned(nextSqrtPrice, current, curve[i+1].Liquidity, dbc.RoundingUp)
				if err != nil {
					return dbc.SwapAmount{}, err
				}
				totalAmountIn.Add(totalAmountIn, inAmount)
				current = nextSqrtPrice
				amountLeft = big.NewInt(0)
				break
			}
			nextSqrtPrice := curve[i].SqrtPrice
			inAmount, err := GetDeltaAmountBaseUnsigned(nextSqrtPrice, current, curve[i+1].Liquidity, dbc.RoundingUp)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			totalAmountIn.Add(totalAmountIn, inAmount)
			current = nextSqrtPrice
			amountLeft.Sub(amountLeft, maxAmountOut)
		}
	}

	if amountLeft.Sign() != 0 {
		maxAmountOut, err := GetDeltaAmountQuoteUnsigned(U128ToBig(config.SqrtStartPrice), current, curve[0].Liquidity, dbc.RoundingDown)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		if amountLeft.Cmp(maxAmountOut) > 0 {
			return dbc.SwapAmount{}, errors.New("Insufficient Liquidity")
		}
		nextSqrtPrice, err := GetNextSqrtPriceFromOutput(current, curve[0].Liquidity, amountLeft, true)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		if nextSqrtPrice.Cmp(U128ToBig(config.SqrtStartPrice)) < 0 {
			return dbc.SwapAmount{}, errors.New("Insufficient Liquidity")
		}
		inAmount, err := GetDeltaAmountBaseUnsigned(nextSqrtPrice, current, curve[0].Liquidity, dbc.RoundingUp)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		totalAmountIn.Add(totalAmountIn, inAmount)
		current = nextSqrtPrice
	}

	return dbc.SwapAmount{OutputAmount: totalAmountIn, NextSqrtPrice: current, AmountLeft: big.NewInt(0)}, nil
}

func CalculateQuoteToBaseFromAmountOut(config *dbc.PoolConfig, currentSqrtPrice, outAmount *big.Int) (dbc.SwapAmount, error) {
	curve := curveFromConfig(config)
	current := new(big.Int).Set(currentSqrtPrice)
	amountLeft := new(big.Int).Set(outAmount)
	totalIn := big.NewInt(0)

	for i := 0; i < len(curve); i++ {
		if curve[i].SqrtPrice.Sign() == 0 || curve[i].Liquidity.Sign() == 0 {
			break
		}
		if curve[i].SqrtPrice.Cmp(current) > 0 {
			maxAmountOut, err := GetDeltaAmountBaseUnsigned(current, curve[i].SqrtPrice, curve[i].Liquidity, dbc.RoundingDown)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			if amountLeft.Cmp(maxAmountOut) < 0 {
				nextSqrtPrice, err := GetNextSqrtPriceFromOutput(current, curve[i].Liquidity, amountLeft, false)
				if err != nil {
					return dbc.SwapAmount{}, err
				}
				inAmount, err := GetDeltaAmountQuoteUnsigned(current, nextSqrtPrice, curve[i].Liquidity, dbc.RoundingUp)
				if err != nil {
					return dbc.SwapAmount{}, err
				}
				totalIn.Add(totalIn, inAmount)
				current = nextSqrtPrice
				amountLeft = big.NewInt(0)
				break
			}
			nextSqrtPrice := curve[i].SqrtPrice
			inAmount, err := GetDeltaAmountQuoteUnsigned(current, nextSqrtPrice, curve[i].Liquidity, dbc.RoundingUp)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			totalIn.Add(totalIn, inAmount)
			current = nextSqrtPrice
			amountLeft.Sub(amountLeft, maxAmountOut)
		}
	}

	if amountLeft.Sign() != 0 {
		return dbc.SwapAmount{}, errors.New("Not enough liquidity")
	}
	return dbc.SwapAmount{OutputAmount: totalIn, NextSqrtPrice: current, AmountLeft: big.NewInt(0)}, nil
}

func SwapQuoteExactIn(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, swapBaseForQuote bool, amountIn *big.Int, slippageBps uint16, hasReferral bool, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapQuote2Result, error) {
	if new(big.Int).SetUint64(virtualPool.QuoteReserve).Cmp(new(big.Int).SetUint64(config.MigrationQuoteThreshold)) >= 0 {
		return dbc.SwapQuote2Result{}, errors.New("Virtual pool is completed")
	}
	if amountIn.Sign() == 0 {
		return dbc.SwapQuote2Result{}, errors.New("Amount is zero")
	}
	tradeDirection := dbc.TradeDirectionQuoteToBase
	if swapBaseForQuote {
		tradeDirection = dbc.TradeDirectionBaseToQuote
	}
	feeMode := GetFeeMode(dbc.CollectFeeMode(config.CollectFeeMode), tradeDirection, hasReferral)
	result, err := GetSwapResultFromExactInput(virtualPool, config, amountIn, feeMode, tradeDirection, currentPoint, eligibleForFirstSwapWithMinFee)
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	minimumAmountOut := result.OutputAmount
	if slippageBps > 0 {
		slippageFactor := big.NewInt(int64(10000 - slippageBps))
		minimumAmountOut = new(big.Int).Div(new(big.Int).Mul(result.OutputAmount, slippageFactor), big.NewInt(10000))
	}
	return dbc.SwapQuote2Result{SwapResult2: result, MinimumAmountOut: minimumAmountOut}, nil
}

func SwapQuotePartialFill(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, swapBaseForQuote bool, amountIn *big.Int, slippageBps uint16, hasReferral bool, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapQuote2Result, error) {
	if new(big.Int).SetUint64(virtualPool.QuoteReserve).Cmp(new(big.Int).SetUint64(config.MigrationQuoteThreshold)) >= 0 {
		return dbc.SwapQuote2Result{}, errors.New("Virtual pool is completed")
	}
	if amountIn.Sign() == 0 {
		return dbc.SwapQuote2Result{}, errors.New("Amount is zero")
	}
	tradeDirection := dbc.TradeDirectionQuoteToBase
	if swapBaseForQuote {
		tradeDirection = dbc.TradeDirectionBaseToQuote
	}
	feeMode := GetFeeMode(dbc.CollectFeeMode(config.CollectFeeMode), tradeDirection, hasReferral)
	result, err := GetSwapResultFromPartialInput(virtualPool, config, amountIn, feeMode, tradeDirection, currentPoint, eligibleForFirstSwapWithMinFee)
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	minimumAmountOut := result.OutputAmount
	if slippageBps > 0 {
		slippageFactor := big.NewInt(int64(10000 - slippageBps))
		minimumAmountOut = new(big.Int).Div(new(big.Int).Mul(result.OutputAmount, slippageFactor), big.NewInt(10000))
	}
	return dbc.SwapQuote2Result{SwapResult2: result, MinimumAmountOut: minimumAmountOut}, nil
}

func SwapQuoteExactOut(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, swapBaseForQuote bool, outAmount *big.Int, slippageBps uint16, hasReferral bool, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapQuote2Result, error) {
	if new(big.Int).SetUint64(virtualPool.QuoteReserve).Cmp(new(big.Int).SetUint64(config.MigrationQuoteThreshold)) >= 0 {
		return dbc.SwapQuote2Result{}, errors.New("Virtual pool is completed")
	}
	if outAmount.Sign() == 0 {
		return dbc.SwapQuote2Result{}, errors.New("Amount is zero")
	}
	tradeDirection := dbc.TradeDirectionQuoteToBase
	if swapBaseForQuote {
		tradeDirection = dbc.TradeDirectionBaseToQuote
	}
	feeMode := GetFeeMode(dbc.CollectFeeMode(config.CollectFeeMode), tradeDirection, hasReferral)
	result, err := GetSwapResultFromExactOutput(virtualPool, config, outAmount, feeMode, tradeDirection, currentPoint, eligibleForFirstSwapWithMinFee)
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	maximumAmountIn := result.IncludedFeeInputAmount
	if slippageBps > 0 {
		slippageFactor := big.NewInt(int64(10000 + slippageBps))
		maximumAmountIn = new(big.Int).Div(new(big.Int).Mul(result.IncludedFeeInputAmount, slippageFactor), big.NewInt(10000))
	}
	return dbc.SwapQuote2Result{SwapResult2: result, MaximumAmountIn: maximumAmountIn}, nil
}

func minBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}

// binFromBig converts big.Int to binary.Uint128 (little endian default).
// no-op placeholder removed
