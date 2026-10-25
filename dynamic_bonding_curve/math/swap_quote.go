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

func addU64(total, amount *big.Int) (*big.Int, error) {
	sum := new(big.Int).Add(total, amount)
	if sum.Cmp(dbc.U64Max) > 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	return sum, nil
}

func assertSlippageBps(slippageBps uint64) error {
	if slippageBps > dbc.MaxBasisPoint {
		return errors.New("slippageBps must be between 0 and 10000")
	}
	return nil
}

func minimumAmountOutWithSlippage(amount *big.Int, slippageBps uint64) (*big.Int, error) {
	if err := assertSlippageBps(slippageBps); err != nil {
		return nil, err
	}
	return new(big.Int).Div(
		new(big.Int).Mul(amount, big.NewInt(int64(dbc.MaxBasisPoint-slippageBps))),
		big.NewInt(dbc.MaxBasisPoint),
	), nil
}

func maximumAmountInWithSlippage(amount *big.Int, slippageBps uint64) (*big.Int, error) {
	if err := assertSlippageBps(slippageBps); err != nil {
		return nil, err
	}
	return new(big.Int).Div(
		new(big.Int).Mul(amount, big.NewInt(int64(dbc.MaxBasisPoint+slippageBps))),
		big.NewInt(dbc.MaxBasisPoint),
	), nil
}

func isFirstSwapWithMinFee(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, hasReferral, eligible bool) bool {
	return eligible && !hasReferral && config.EnableFirstSwapWithMinFee == 1 && virtualPool.PoolState.HasSwap == 0
}

func tradeFeeNumerator(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, currentPoint *big.Int, tradeDirection dbc.TradeDirection, amount *big.Int, amountExcludesFee, eligibleForFirstSwapWithMinFee bool) (*big.Int, error) {
	baseFeeHandler, err := pool_fees.GetBaseFeeHandler(
		new(big.Int).SetUint64(config.PoolFees.BaseFee.CliffFeeNumerator),
		config.PoolFees.BaseFee.FirstFactor,
		new(big.Int).SetUint64(config.PoolFees.BaseFee.SecondFactor),
		new(big.Int).SetUint64(config.PoolFees.BaseFee.ThirdFactor),
		dbc.BaseFeeMode(config.PoolFees.BaseFee.BaseFeeMode),
	)
	if err != nil {
		return nil, err
	}
	if eligibleForFirstSwapWithMinFee {
		return baseFeeHandler.GetMinBaseFeeNumerator(), nil
	}
	activationPoint := new(big.Int).SetUint64(virtualPool.PoolState.ActivationPoint)
	if amountExcludesFee {
		return GetTotalFeeNumeratorFromExcludedFeeAmount(config.PoolFees, virtualPool.PoolState.VolatilityTracker, currentPoint, activationPoint, amount, tradeDirection)
	}
	return GetTotalFeeNumeratorFromIncludedFeeAmount(config.PoolFees, virtualPool.PoolState.VolatilityTracker, currentPoint, activationPoint, amount, tradeDirection)
}

type swapFees struct {
	trading  *big.Int
	protocol *big.Int
	referral *big.Int
}

func takeFee(feeNumerator, includedFeeAmount *big.Int, hasReferral bool) (amount *big.Int, fees swapFees, err error) {
	result, err := GetFeeOnAmount(feeNumerator, includedFeeAmount, hasReferral)
	if err != nil {
		return nil, swapFees{}, err
	}
	return result.Amount, swapFees{trading: result.TradingFee, protocol: result.ProtocolFee, referral: result.ReferralFee}, nil
}

func addFee(feeNumerator, excludedFeeAmount *big.Int, hasReferral bool) (amount *big.Int, fees swapFees, err error) {
	included, feeAmount, err := GetIncludedFeeAmount(feeNumerator, excludedFeeAmount)
	if err != nil {
		return nil, swapFees{}, err
	}
	trading, protocol, referral, err := SplitFees(feeAmount, hasReferral)
	if err != nil {
		return nil, swapFees{}, err
	}
	return included, swapFees{trading: trading, protocol: protocol, referral: referral}, nil
}

func zeroFees() swapFees {
	return swapFees{trading: big.NewInt(0), protocol: big.NewInt(0), referral: big.NewInt(0)}
}

func swapFromInput(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, tradeDirection dbc.TradeDirection, amountIn *big.Int) (dbc.SwapAmount, error) {
	currentSqrtPrice := U128ToBig(virtualPool.PoolState.SqrtPrice)
	if tradeDirection == dbc.TradeDirectionBaseToQuote {
		return CalculateBaseToQuoteFromAmountIn(config, currentSqrtPrice, amountIn)
	}
	return CalculateQuoteToBaseFromAmountIn(config, currentSqrtPrice, amountIn, U128ToBig(config.MigrationSqrtPrice))
}

func beginSwapQuote(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, swapBaseForQuote bool, amount *big.Int, hasReferral bool, transferFees *dbc.QuoteTransferFees) (dbc.TradeDirection, dbc.FeeMode, *dbc.EpochTransferFee, *dbc.EpochTransferFee, error) {
	if amount.Sign() == 0 {
		return 0, dbc.FeeMode{}, nil, nil, errors.New(dbc.ErrAmountIsZero)
	}
	if amount.Cmp(dbc.U64Max) > 0 {
		return 0, dbc.FeeMode{}, nil, nil, errors.New(dbc.ErrMathOverflow)
	}
	if new(big.Int).SetUint64(virtualPool.PoolState.QuoteReserve).Cmp(new(big.Int).SetUint64(config.MigrationQuoteThreshold)) >= 0 {
		return 0, dbc.FeeMode{}, nil, nil, errors.New(dbc.ErrPoolIsCompleted)
	}
	tradeDirection := dbc.TradeDirectionQuoteToBase
	if swapBaseForQuote {
		tradeDirection = dbc.TradeDirectionBaseToQuote
	}
	feeMode, err := GetFeeMode(dbc.CollectFeeMode(config.CollectFeeMode), tradeDirection, hasReferral)
	if err != nil {
		return 0, dbc.FeeMode{}, nil, nil, err
	}
	input, output, err := ResolveSwapTransferFees(config, swapBaseForQuote, transferFees)
	if err != nil {
		return 0, dbc.FeeMode{}, nil, nil, err
	}
	return tradeDirection, feeMode, input, output, nil
}

func swapResult2(amountLeft, includedIn, excludedIn, output, next *big.Int, fees swapFees) dbc.SwapResult2 {
	return dbc.SwapResult2{
		AmountLeft:             amountLeft,
		IncludedFeeInputAmount: includedIn,
		ExcludedFeeInputAmount: excludedIn,
		OutputAmount:           output,
		NextSqrtPrice:          next,
		TradingFee:             fees.trading,
		ProtocolFee:            fees.protocol,
		ReferralFee:            fees.referral,
	}
}

// GetSwapResultFromExactInput is the program's get_swap_result_from_exact_input.
func GetSwapResultFromExactInput(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, amountIn *big.Int, feeMode dbc.FeeMode, tradeDirection dbc.TradeDirection, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapResult2, error) {
	feeNumerator, err := tradeFeeNumerator(virtualPool, config, currentPoint, tradeDirection, amountIn, false, eligibleForFirstSwapWithMinFee)
	if err != nil {
		return dbc.SwapResult2{}, err
	}
	fees := zeroFees()
	actualAmountIn := new(big.Int).Set(amountIn)
	if feeMode.FeesOnInput {
		actualAmountIn, fees, err = takeFee(feeNumerator, amountIn, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
	}
	swapAmount, err := swapFromInput(virtualPool, config, tradeDirection, actualAmountIn)
	if err != nil {
		return dbc.SwapResult2{}, err
	}
	if swapAmount.AmountLeft.Sign() != 0 {
		return dbc.SwapResult2{}, errors.New(dbc.ErrInsufficientLiquidity)
	}
	actualAmountOut := swapAmount.OutputAmount
	if !feeMode.FeesOnInput {
		actualAmountOut, fees, err = takeFee(feeNumerator, swapAmount.OutputAmount, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
	}
	return swapResult2(swapAmount.AmountLeft, amountIn, actualAmountIn, actualAmountOut, swapAmount.NextSqrtPrice, fees), nil
}

// GetSwapResult is the legacy swap result derived from an exact-input quote.
func GetSwapResult(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, amountIn *big.Int, feeMode dbc.FeeMode, tradeDirection dbc.TradeDirection, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapResult, error) {
	result, err := GetSwapResultFromExactInput(virtualPool, config, amountIn, feeMode, tradeDirection, currentPoint, eligibleForFirstSwapWithMinFee)
	if err != nil {
		return dbc.SwapResult{}, err
	}
	return dbc.SwapResult{
		ActualInputAmount: result.ExcludedFeeInputAmount,
		OutputAmount:      result.OutputAmount,
		NextSqrtPrice:     result.NextSqrtPrice,
		TradingFee:        result.TradingFee,
		ProtocolFee:       result.ProtocolFee,
		ReferralFee:       result.ReferralFee,
	}, nil
}

// GetSwapResultFromPartialInput is the program's get_swap_result_from_partial_input.
func GetSwapResultFromPartialInput(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, amountIn *big.Int, feeMode dbc.FeeMode, tradeDirection dbc.TradeDirection, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapResult2, error) {
	feeNumerator, err := tradeFeeNumerator(virtualPool, config, currentPoint, tradeDirection, amountIn, false, eligibleForFirstSwapWithMinFee)
	if err != nil {
		return dbc.SwapResult2{}, err
	}
	fees := zeroFees()
	actualAmountIn := new(big.Int).Set(amountIn)
	if feeMode.FeesOnInput {
		actualAmountIn, fees, err = takeFee(feeNumerator, amountIn, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
	}
	swapAmount, err := swapFromInput(virtualPool, config, tradeDirection, actualAmountIn)
	if err != nil {
		return dbc.SwapResult2{}, err
	}
	includedFeeInputAmount := new(big.Int).Set(amountIn)
	if swapAmount.AmountLeft.Sign() != 0 {
		actualAmountIn, err = Sub(actualAmountIn, swapAmount.AmountLeft)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		if feeMode.FeesOnInput {
			partialFee, err := tradeFeeNumerator(virtualPool, config, currentPoint, tradeDirection, actualAmountIn, true, eligibleForFirstSwapWithMinFee)
			if err != nil {
				return dbc.SwapResult2{}, err
			}
			includedFeeInputAmount, fees, err = addFee(partialFee, actualAmountIn, feeMode.HasReferral)
			if err != nil {
				return dbc.SwapResult2{}, err
			}
		} else {
			includedFeeInputAmount = actualAmountIn
		}
	}
	actualAmountOut := swapAmount.OutputAmount
	if !feeMode.FeesOnInput {
		actualAmountOut, fees, err = takeFee(feeNumerator, swapAmount.OutputAmount, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
	}
	return swapResult2(swapAmount.AmountLeft, includedFeeInputAmount, actualAmountIn, actualAmountOut, swapAmount.NextSqrtPrice, fees), nil
}

// GetSwapResultFromExactOutput is the program's get_swap_result_from_exact_output.
func GetSwapResultFromExactOutput(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, amountOut *big.Int, feeMode dbc.FeeMode, tradeDirection dbc.TradeDirection, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool) (dbc.SwapResult2, error) {
	fees := zeroFees()
	includedFeeOutAmount := new(big.Int).Set(amountOut)
	var err error
	if !feeMode.FeesOnInput {
		feeNumerator, err := tradeFeeNumerator(virtualPool, config, currentPoint, tradeDirection, amountOut, true, eligibleForFirstSwapWithMinFee)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		includedFeeOutAmount, fees, err = addFee(feeNumerator, amountOut, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
	}

	currentSqrtPrice := U128ToBig(virtualPool.PoolState.SqrtPrice)
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
		return dbc.SwapResult2{}, errors.New(dbc.ErrInsufficientLiquidity)
	}

	includedFeeInputAmount := new(big.Int).Set(amountIn)
	if feeMode.FeesOnInput {
		feeNumerator, err := tradeFeeNumerator(virtualPool, config, currentPoint, tradeDirection, amountIn, true, eligibleForFirstSwapWithMinFee)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
		includedFeeInputAmount, fees, err = addFee(feeNumerator, amountIn, feeMode.HasReferral)
		if err != nil {
			return dbc.SwapResult2{}, err
		}
	}
	return swapResult2(big.NewInt(0), includedFeeInputAmount, amountIn, amountOut, swapAmount.NextSqrtPrice, fees), nil
}

// CalculateBaseToQuoteFromAmountIn is the quote received for amountIn base, walking the curve down.
func CalculateBaseToQuoteFromAmountIn(config *dbc.PoolConfig, currentSqrtPrice, amountIn *big.Int) (dbc.SwapAmount, error) {
	curve := curveFromConfig(config)
	totalOutput := big.NewInt(0)
	sqrtPrice := new(big.Int).Set(currentSqrtPrice)
	amountLeft := new(big.Int).Set(amountIn)
	var err error

	for i := len(curve) - 2; i >= 0; i-- {
		point := curve[i]
		liquidity := curve[i+1].Liquidity
		if point.SqrtPrice.Sign() == 0 || point.Liquidity.Sign() == 0 {
			continue
		}
		if point.SqrtPrice.Cmp(sqrtPrice) >= 0 {
			continue
		}
		maxAmountIn, err := GetDeltaAmountBaseUnsigned256(point.SqrtPrice, sqrtPrice, liquidity, dbc.RoundingUp)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		if amountLeft.Cmp(maxAmountIn) < 0 {
			nextSqrtPrice, err := GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountLeft, true)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			outputAmount, err := GetDeltaAmountQuoteUnsigned(nextSqrtPrice, sqrtPrice, liquidity, dbc.RoundingDown)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			totalOutput, err = addU64(totalOutput, outputAmount)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			sqrtPrice = nextSqrtPrice
			amountLeft = big.NewInt(0)
			break
		}
		outputAmount, err := GetDeltaAmountQuoteUnsigned(point.SqrtPrice, sqrtPrice, liquidity, dbc.RoundingDown)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		totalOutput, err = addU64(totalOutput, outputAmount)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		capped, err := requireU64(maxAmountIn)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		amountLeft, err = Sub(amountLeft, capped)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		sqrtPrice = point.SqrtPrice
	}

	if amountLeft.Sign() != 0 {
		liquidity := curve[0].Liquidity
		nextSqrtPrice, err := GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountLeft, true)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		start := U128ToBig(config.SqrtStartPrice)
		if nextSqrtPrice.Cmp(start) < 0 {
			nextSqrtPrice = start
			consumed, err := GetDeltaAmountBaseUnsigned(nextSqrtPrice, sqrtPrice, liquidity, dbc.RoundingUp)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			amountLeft, err = Sub(amountLeft, consumed)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
		} else {
			amountLeft = big.NewInt(0)
		}
		outputAmount, err := GetDeltaAmountQuoteUnsigned(nextSqrtPrice, sqrtPrice, liquidity, dbc.RoundingDown)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		totalOutput, err = addU64(totalOutput, outputAmount)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		sqrtPrice = nextSqrtPrice
	}

	return dbc.SwapAmount{OutputAmount: totalOutput, NextSqrtPrice: sqrtPrice, AmountLeft: amountLeft}, err
}

// CalculateQuoteToBaseFromAmountIn is the base received for amountIn quote, walking the curve up to stopSqrtPrice.
func CalculateQuoteToBaseFromAmountIn(config *dbc.PoolConfig, currentSqrtPrice, amountIn, stopSqrtPrice *big.Int) (dbc.SwapAmount, error) {
	curve := curveFromConfig(config)
	totalOutput := big.NewInt(0)
	sqrtPrice := new(big.Int).Set(currentSqrtPrice)
	amountLeft := new(big.Int).Set(amountIn)

	for i := range curve {
		point := curve[i]
		if point.SqrtPrice.Sign() == 0 || point.Liquidity.Sign() == 0 {
			break
		}
		reference := stopSqrtPrice
		if point.SqrtPrice.Cmp(reference) < 0 {
			reference = point.SqrtPrice
		}
		if reference.Cmp(sqrtPrice) <= 0 {
			continue
		}
		maxAmountIn, err := GetDeltaAmountQuoteUnsigned256(sqrtPrice, reference, point.Liquidity, dbc.RoundingUp)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		if amountLeft.Cmp(maxAmountIn) < 0 {
			nextSqrtPrice, err := GetNextSqrtPriceFromInput(sqrtPrice, point.Liquidity, amountLeft, false)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			outputAmount, err := GetDeltaAmountBaseUnsigned(sqrtPrice, nextSqrtPrice, point.Liquidity, dbc.RoundingDown)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			totalOutput, err = addU64(totalOutput, outputAmount)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			return dbc.SwapAmount{OutputAmount: totalOutput, NextSqrtPrice: nextSqrtPrice, AmountLeft: big.NewInt(0)}, nil
		}
		outputAmount, err := GetDeltaAmountBaseUnsigned(sqrtPrice, reference, point.Liquidity, dbc.RoundingDown)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		totalOutput, err = addU64(totalOutput, outputAmount)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		capped, err := requireU64(maxAmountIn)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		amountLeft, err = Sub(amountLeft, capped)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		sqrtPrice = new(big.Int).Set(reference)
		if sqrtPrice.Cmp(stopSqrtPrice) == 0 {
			break
		}
	}
	return dbc.SwapAmount{OutputAmount: totalOutput, NextSqrtPrice: sqrtPrice, AmountLeft: amountLeft}, nil
}

// CalculateBaseToQuoteFromAmountOut is the base needed for outAmount quote. OutputAmount is the required base input.
func CalculateBaseToQuoteFromAmountOut(config *dbc.PoolConfig, currentSqrtPrice, outAmount *big.Int) (dbc.SwapAmount, error) {
	curve := curveFromConfig(config)
	sqrtPrice := new(big.Int).Set(currentSqrtPrice)
	amountLeft := new(big.Int).Set(outAmount)
	totalAmountIn := big.NewInt(0)

	for i := len(curve) - 2; i >= 0; i-- {
		point := curve[i]
		liquidity := curve[i+1].Liquidity
		if point.SqrtPrice.Sign() == 0 || point.Liquidity.Sign() == 0 {
			continue
		}
		if point.SqrtPrice.Cmp(sqrtPrice) >= 0 {
			continue
		}
		maxAmountOut, err := GetDeltaAmountQuoteUnsigned256(point.SqrtPrice, sqrtPrice, liquidity, dbc.RoundingDown)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		if amountLeft.Cmp(maxAmountOut) < 0 {
			nextSqrtPrice, err := GetNextSqrtPriceFromOutput(sqrtPrice, liquidity, amountLeft, true)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			inAmount, err := GetDeltaAmountBaseUnsigned(nextSqrtPrice, sqrtPrice, liquidity, dbc.RoundingUp)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			totalAmountIn, err = addU64(totalAmountIn, inAmount)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			sqrtPrice = nextSqrtPrice
			amountLeft = big.NewInt(0)
			break
		}
		inAmount, err := GetDeltaAmountBaseUnsigned(point.SqrtPrice, sqrtPrice, liquidity, dbc.RoundingUp)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		totalAmountIn, err = addU64(totalAmountIn, inAmount)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		capped, err := requireU64(maxAmountOut)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		amountLeft, err = Sub(amountLeft, capped)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		sqrtPrice = point.SqrtPrice
	}

	if amountLeft.Sign() != 0 {
		liquidity := curve[0].Liquidity
		start := U128ToBig(config.SqrtStartPrice)
		maxAmountOut, err := GetDeltaAmountQuoteUnsigned256(start, sqrtPrice, liquidity, dbc.RoundingDown)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		if amountLeft.Cmp(maxAmountOut) > 0 {
			return dbc.SwapAmount{}, errors.New(dbc.ErrInsufficientLiquidity)
		}
		nextSqrtPrice, err := GetNextSqrtPriceFromOutput(sqrtPrice, liquidity, amountLeft, true)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		if nextSqrtPrice.Cmp(start) < 0 {
			return dbc.SwapAmount{}, errors.New(dbc.ErrInsufficientLiquidity)
		}
		inAmount, err := GetDeltaAmountBaseUnsigned(nextSqrtPrice, sqrtPrice, liquidity, dbc.RoundingUp)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		totalAmountIn, err = addU64(totalAmountIn, inAmount)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		sqrtPrice = nextSqrtPrice
	}
	return dbc.SwapAmount{OutputAmount: totalAmountIn, NextSqrtPrice: sqrtPrice, AmountLeft: big.NewInt(0)}, nil
}

// CalculateQuoteToBaseFromAmountOut is the quote needed for outAmount base. OutputAmount is the required quote input.
func CalculateQuoteToBaseFromAmountOut(config *dbc.PoolConfig, currentSqrtPrice, outAmount *big.Int) (dbc.SwapAmount, error) {
	curve := curveFromConfig(config)
	totalIn := big.NewInt(0)
	sqrtPrice := new(big.Int).Set(currentSqrtPrice)
	amountLeft := new(big.Int).Set(outAmount)

	for i := range curve {
		point := curve[i]
		if point.SqrtPrice.Sign() == 0 || point.Liquidity.Sign() == 0 {
			break
		}
		if point.SqrtPrice.Cmp(sqrtPrice) <= 0 {
			continue
		}
		maxAmountOut, err := GetDeltaAmountBaseUnsigned256(sqrtPrice, point.SqrtPrice, point.Liquidity, dbc.RoundingDown)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		if amountLeft.Cmp(maxAmountOut) < 0 {
			nextSqrtPrice, err := GetNextSqrtPriceFromOutput(sqrtPrice, point.Liquidity, amountLeft, false)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			inAmount, err := GetDeltaAmountQuoteUnsigned(sqrtPrice, nextSqrtPrice, point.Liquidity, dbc.RoundingUp)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			totalIn, err = addU64(totalIn, inAmount)
			if err != nil {
				return dbc.SwapAmount{}, err
			}
			return dbc.SwapAmount{OutputAmount: totalIn, NextSqrtPrice: nextSqrtPrice, AmountLeft: big.NewInt(0)}, nil
		}
		inAmount, err := GetDeltaAmountQuoteUnsigned(sqrtPrice, point.SqrtPrice, point.Liquidity, dbc.RoundingUp)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		totalIn, err = addU64(totalIn, inAmount)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		capped, err := requireU64(maxAmountOut)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		amountLeft, err = Sub(amountLeft, capped)
		if err != nil {
			return dbc.SwapAmount{}, err
		}
		sqrtPrice = point.SqrtPrice
	}
	if amountLeft.Sign() != 0 {
		return dbc.SwapAmount{}, errors.New(dbc.ErrAmountLeftIsNotZero)
	}
	return dbc.SwapAmount{OutputAmount: totalIn, NextSqrtPrice: sqrtPrice, AmountLeft: big.NewInt(0)}, nil
}

func quoteExactIn(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, swapBaseForQuote bool, amountIn *big.Int, slippageBps uint16, hasReferral bool, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool, transferFees *dbc.QuoteTransferFees) (dbc.SwapQuote2Result, error) {
	tradeDirection, feeMode, input, output, err := beginSwapQuote(virtualPool, config, swapBaseForQuote, amountIn, hasReferral, transferFees)
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	excludedAmountIn, _ := CalculateTransferFeeExcludedAmount(input, amountIn)
	if excludedAmountIn.Sign() == 0 {
		return dbc.SwapQuote2Result{}, errors.New(dbc.ErrAmountIsZero)
	}
	result, err := GetSwapResultFromExactInput(virtualPool, config, excludedAmountIn, feeMode, tradeDirection, currentPoint, isFirstSwapWithMinFee(virtualPool, config, hasReferral, eligibleForFirstSwapWithMinFee))
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	excludedAmountOut, _ := CalculateTransferFeeExcludedAmount(output, result.OutputAmount)
	minimum, err := minimumAmountOutWithSlippage(excludedAmountOut, uint64(slippageBps))
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	return dbc.SwapQuote2Result{
		SwapResult2:                  result,
		MinimumAmountOut:             minimum,
		IncludedTransferFeeAmountIn:  new(big.Int).Set(amountIn),
		ExcludedTransferFeeAmountOut: excludedAmountOut,
	}, nil
}

// SwapQuote is an exact-in quote in the legacy SwapResult shape.
func SwapQuote(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, swapBaseForQuote bool, amountIn *big.Int, slippageBps uint16, hasReferral bool, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool, transferFees *dbc.QuoteTransferFees) (dbc.SwapQuoteResult, error) {
	result, err := quoteExactIn(virtualPool, config, swapBaseForQuote, amountIn, slippageBps, hasReferral, currentPoint, eligibleForFirstSwapWithMinFee, transferFees)
	if err != nil {
		return dbc.SwapQuoteResult{}, err
	}
	return dbc.SwapQuoteResult{
		SwapResult: dbc.SwapResult{
			ActualInputAmount: result.ExcludedFeeInputAmount,
			OutputAmount:      result.OutputAmount,
			NextSqrtPrice:     result.NextSqrtPrice,
			TradingFee:        result.TradingFee,
			ProtocolFee:       result.ProtocolFee,
			ReferralFee:       result.ReferralFee,
		},
		MinimumAmountOut:             result.MinimumAmountOut,
		IncludedTransferFeeAmountIn:  result.IncludedTransferFeeAmountIn,
		ExcludedTransferFeeAmountOut: result.ExcludedTransferFeeAmountOut,
	}, nil
}

// SwapQuoteExactIn quotes an exact input. amountIn includes any transfer fee on the input mint.
func SwapQuoteExactIn(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, swapBaseForQuote bool, amountIn *big.Int, slippageBps uint16, hasReferral bool, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool, transferFees *dbc.QuoteTransferFees) (dbc.SwapQuote2Result, error) {
	return quoteExactIn(virtualPool, config, swapBaseForQuote, amountIn, slippageBps, hasReferral, currentPoint, eligibleForFirstSwapWithMinFee, transferFees)
}

// SwapQuotePartialFill quotes a partial fill. The input the pool can take is IncludedTransferFeeAmountIn.
func SwapQuotePartialFill(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, swapBaseForQuote bool, amountIn *big.Int, slippageBps uint16, hasReferral bool, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool, transferFees *dbc.QuoteTransferFees) (dbc.SwapQuote2Result, error) {
	tradeDirection, feeMode, input, output, err := beginSwapQuote(virtualPool, config, swapBaseForQuote, amountIn, hasReferral, transferFees)
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	excludedAmountIn, _ := CalculateTransferFeeExcludedAmount(input, amountIn)
	if excludedAmountIn.Sign() == 0 {
		return dbc.SwapQuote2Result{}, errors.New(dbc.ErrAmountIsZero)
	}
	result, err := GetSwapResultFromPartialInput(virtualPool, config, excludedAmountIn, feeMode, tradeDirection, currentPoint, isFirstSwapWithMinFee(virtualPool, config, hasReferral, eligibleForFirstSwapWithMinFee))
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	includedAmountIn := new(big.Int).Set(amountIn)
	if result.AmountLeft.Sign() != 0 {
		includedAmountIn, _, err = CalculateTransferFeeIncludedAmount(input, result.IncludedFeeInputAmount)
		if err != nil {
			return dbc.SwapQuote2Result{}, err
		}
	}
	if includedAmountIn.Cmp(amountIn) > 0 {
		return dbc.SwapQuote2Result{}, errors.New(dbc.ErrExceededInputAmount)
	}
	excludedAmountOut, _ := CalculateTransferFeeExcludedAmount(output, result.OutputAmount)
	minimum, err := minimumAmountOutWithSlippage(excludedAmountOut, uint64(slippageBps))
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	return dbc.SwapQuote2Result{
		SwapResult2:                  result,
		MinimumAmountOut:             minimum,
		IncludedTransferFeeAmountIn:  includedAmountIn,
		ExcludedTransferFeeAmountOut: excludedAmountOut,
	}, nil
}

// SwapQuoteExactOut quotes an exact output. outAmount is what the user receives after the output transfer fee.
func SwapQuoteExactOut(virtualPool *dbc.VirtualPool, config *dbc.PoolConfig, swapBaseForQuote bool, outAmount *big.Int, slippageBps uint16, hasReferral bool, currentPoint *big.Int, eligibleForFirstSwapWithMinFee bool, transferFees *dbc.QuoteTransferFees) (dbc.SwapQuote2Result, error) {
	tradeDirection, feeMode, input, output, err := beginSwapQuote(virtualPool, config, swapBaseForQuote, outAmount, hasReferral, transferFees)
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	includedAmountOut, _, err := CalculateTransferFeeIncludedAmount(output, outAmount)
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	if includedAmountOut.Sign() == 0 {
		return dbc.SwapQuote2Result{}, errors.New(dbc.ErrAmountIsZero)
	}
	result, err := GetSwapResultFromExactOutput(virtualPool, config, includedAmountOut, feeMode, tradeDirection, currentPoint, isFirstSwapWithMinFee(virtualPool, config, hasReferral, eligibleForFirstSwapWithMinFee))
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	includedAmountIn, _, err := CalculateTransferFeeIncludedAmount(input, result.IncludedFeeInputAmount)
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	maximum, err := maximumAmountInWithSlippage(includedAmountIn, uint64(slippageBps))
	if err != nil {
		return dbc.SwapQuote2Result{}, err
	}
	return dbc.SwapQuote2Result{
		SwapResult2:                  result,
		MaximumAmountIn:              maximum,
		IncludedTransferFeeAmountIn:  includedAmountIn,
		ExcludedTransferFeeAmountOut: new(big.Int).Set(outAmount),
	}, nil
}
