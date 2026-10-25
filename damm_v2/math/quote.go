package math

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/helpers"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

type swapResultBig struct {
	includedFeeInputAmount *big.Int
	excludedFeeInputAmount *big.Int
	amountLeft             *big.Int
	outputAmount           *big.Int
	nextSqrtPrice          *big.Int
	claimingFee            *big.Int
	compoundingFee         *big.Int
	protocolFee            *big.Int
	referralFee            *big.Int
}

func toU64Checked(v *big.Int) (uint64, error) {
	if v == nil || v.Sign() < 0 || v.Cmp(shared.U64Max) > 0 {
		return 0, errors.New("math overflow")
	}
	return v.Uint64(), nil
}

func (r swapResultBig) toSwapResult() (shared.SwapResult2, error) {
	included, err := toU64Checked(r.includedFeeInputAmount)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	excluded, err := toU64Checked(r.excludedFeeInputAmount)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	left, err := toU64Checked(r.amountLeft)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	output, err := toU64Checked(r.outputAmount)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	claiming, err := toU64Checked(r.claimingFee)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	protocol, err := toU64Checked(r.protocolFee)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	compounding, err := toU64Checked(r.compoundingFee)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	referral, err := toU64Checked(r.referralFee)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	return shared.SwapResult2{
		IncludedFeeInputAmount: included,
		ExcludedFeeInputAmount: excluded,
		AmountLeft:             left,
		OutputAmount:           output,
		NextSqrtPrice:          u128FromBig(r.nextSqrtPrice),
		ClaimingFee:            claiming,
		ProtocolFee:            protocol,
		CompoundingFee:         compounding,
		ReferralFee:            referral,
	}, nil
}

func applySwapResult(pool *dammv2gen.Pool, result *swapResultBig, feeMode shared.FeeMode, tradeDirection shared.TradeDirection) error {
	if shared.CollectFeeMode(pool.CollectFeeMode) != shared.CollectFeeModeCompounding {
		return nil
	}
	tradingFee := new(big.Int).Add(result.claimingFee, result.compoundingFee)
	includedFeeOutput := new(big.Int).Set(result.outputAmount)
	if !feeMode.FeesOnInput {
		includedFeeOutput.Add(includedFeeOutput, tradingFee)
		includedFeeOutput.Add(includedFeeOutput, result.protocolFee)
		includedFeeOutput.Add(includedFeeOutput, result.referralFee)
	}

	tokenA := new(big.Int).SetUint64(pool.TokenAAmount)
	tokenB := new(big.Int).SetUint64(pool.TokenBAmount)
	var newTokenA, newTokenB *big.Int
	if tradeDirection == shared.TradeDirectionAtoB {
		newTokenA = new(big.Int).Add(tokenA, result.excludedFeeInputAmount)
		newTokenB = new(big.Int).Sub(tokenB, includedFeeOutput)
	} else {
		newTokenB = new(big.Int).Add(tokenB, result.excludedFeeInputAmount)
		newTokenA = new(big.Int).Sub(tokenA, includedFeeOutput)
	}
	newTokenB.Add(newTokenB, result.compoundingFee)
	next, err := GetNextSqrtPriceForCompoundingLiquidity(newTokenA, newTokenB)
	if err != nil {
		return err
	}
	result.nextSqrtPrice = next
	return nil
}

func GetSwapResultFromExactInput(poolState *dammv2gen.Pool, amountIn *big.Int, feeMode shared.FeeMode, tradeDirection shared.TradeDirection, currentPoint *big.Int) (shared.SwapResult2, error) {
	result, err := swapResultFromExactInput(poolState, amountIn, feeMode, tradeDirection, currentPoint)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	return result.toSwapResult()
}

func swapResultFromExactInput(poolState *dammv2gen.Pool, amountIn *big.Int, feeMode shared.FeeMode, tradeDirection shared.TradeDirection, currentPoint *big.Int) (swapResultBig, error) {
	handler := GetLiquidityHandler(poolState)
	maxFeeNumerator, err := GetMaxFeeNumerator(shared.PoolVersion(poolState.FeeVersion))
	if err != nil {
		return swapResultBig{}, err
	}
	activationPoint := new(big.Int).SetUint64(poolState.ActivationPoint)
	initSqrtPrice := poolState.PoolFees.InitSqrtPrice.BigInt()
	currentSqrtPrice := poolState.SqrtPrice.BigInt()

	tradeFeeNumerator, err := GetTotalTradingFeeFromIncludedFeeAmount(poolState.PoolFees, currentPoint, activationPoint, amountIn, tradeDirection, maxFeeNumerator, initSqrtPrice, currentSqrtPrice)
	if err != nil {
		return swapResultBig{}, err
	}

	actualAmountIn := new(big.Int).Set(amountIn)
	claimingFee := big.NewInt(0)
	compoundingFee := big.NewInt(0)
	protocolFee := big.NewInt(0)
	referralFee := big.NewInt(0)
	if feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(poolState.PoolFees, amountIn, tradeFeeNumerator, feeMode.HasReferral)
		if err != nil {
			return swapResultBig{}, err
		}
		claimingFee = feeResult.ClaimingFee
		compoundingFee = feeResult.CompoundingFee
		protocolFee = feeResult.ProtocolFee
		referralFee = feeResult.ReferralFee
		actualAmountIn = feeResult.Amount
	}

	var swapAmount SwapAmountFromInput
	if tradeDirection == shared.TradeDirectionAtoB {
		swapAmount, err = handler.CalculateAtoBFromAmountIn(actualAmountIn)
	} else {
		swapAmount, err = handler.CalculateBtoAFromAmountIn(actualAmountIn)
	}
	if err != nil {
		return swapResultBig{}, err
	}

	actualAmountOut := new(big.Int).Set(swapAmount.OutputAmount)
	if !feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(poolState.PoolFees, swapAmount.OutputAmount, tradeFeeNumerator, feeMode.HasReferral)
		if err != nil {
			return swapResultBig{}, err
		}
		claimingFee = feeResult.ClaimingFee
		compoundingFee = feeResult.CompoundingFee
		protocolFee = feeResult.ProtocolFee
		referralFee = feeResult.ReferralFee
		actualAmountOut = feeResult.Amount
	}

	result := swapResultBig{
		amountLeft:             swapAmount.AmountLeft,
		includedFeeInputAmount: new(big.Int).Set(amountIn),
		excludedFeeInputAmount: actualAmountIn,
		outputAmount:           actualAmountOut,
		nextSqrtPrice:          swapAmount.NextSqrtPrice,
		claimingFee:            claimingFee,
		compoundingFee:         compoundingFee,
		protocolFee:            protocolFee,
		referralFee:            referralFee,
	}
	if err := applySwapResult(poolState, &result, feeMode, tradeDirection); err != nil {
		return swapResultBig{}, err
	}
	return result, nil
}

func GetSwapResultFromPartialInput(poolState *dammv2gen.Pool, amountIn *big.Int, feeMode shared.FeeMode, tradeDirection shared.TradeDirection, currentPoint *big.Int) (shared.SwapResult2, error) {
	result, err := swapResultFromPartialInput(poolState, amountIn, feeMode, tradeDirection, currentPoint)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	return result.toSwapResult()
}

func swapResultFromPartialInput(poolState *dammv2gen.Pool, amountIn *big.Int, feeMode shared.FeeMode, tradeDirection shared.TradeDirection, currentPoint *big.Int) (swapResultBig, error) {
	handler := GetLiquidityHandler(poolState)
	maxFeeNumerator, err := GetMaxFeeNumerator(shared.PoolVersion(poolState.FeeVersion))
	if err != nil {
		return swapResultBig{}, err
	}
	activationPoint := new(big.Int).SetUint64(poolState.ActivationPoint)
	initSqrtPrice := poolState.PoolFees.InitSqrtPrice.BigInt()
	currentSqrtPrice := poolState.SqrtPrice.BigInt()

	tradeFeeNumerator, err := GetTotalTradingFeeFromIncludedFeeAmount(poolState.PoolFees, currentPoint, activationPoint, amountIn, tradeDirection, maxFeeNumerator, initSqrtPrice, currentSqrtPrice)
	if err != nil {
		return swapResultBig{}, err
	}

	actualAmountIn := new(big.Int).Set(amountIn)
	claimingFee := big.NewInt(0)
	compoundingFee := big.NewInt(0)
	protocolFee := big.NewInt(0)
	referralFee := big.NewInt(0)
	if feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(poolState.PoolFees, amountIn, tradeFeeNumerator, feeMode.HasReferral)
		if err != nil {
			return swapResultBig{}, err
		}
		protocolFee = feeResult.ProtocolFee
		claimingFee = feeResult.ClaimingFee
		compoundingFee = feeResult.CompoundingFee
		referralFee = feeResult.ReferralFee
		actualAmountIn = feeResult.Amount
	}

	var swapAmount SwapAmountFromInput
	if tradeDirection == shared.TradeDirectionAtoB {
		swapAmount, err = handler.CalculateAtoBFromPartialAmountIn(actualAmountIn)
	} else {
		swapAmount, err = handler.CalculateBtoAFromPartialAmountIn(actualAmountIn)
	}
	if err != nil {
		return swapResultBig{}, err
	}

	includedFeeInputAmount := new(big.Int).Set(amountIn)
	if swapAmount.AmountLeft.Sign() > 0 {
		actualAmountIn = new(big.Int).Sub(actualAmountIn, swapAmount.AmountLeft)
		if feeMode.FeesOnInput {
			tradeFeeNumerator, err = GetTotalTradingFeeFromExcludedFeeAmount(poolState.PoolFees, currentPoint, activationPoint, actualAmountIn, tradeDirection, maxFeeNumerator, initSqrtPrice, currentSqrtPrice)
			if err != nil {
				return swapResultBig{}, err
			}
			includedFeeAmount, feeAmount, err := GetIncludedFeeAmount(tradeFeeNumerator, actualAmountIn)
			if err != nil {
				return swapResultBig{}, err
			}
			split := SplitFees(poolState.PoolFees, feeAmount, feeMode.HasReferral)
			protocolFee = split.ProtocolFee
			claimingFee = split.ClaimingFee
			compoundingFee = split.CompoundingFee
			referralFee = split.ReferralFee
			includedFeeInputAmount = includedFeeAmount
		} else {
			includedFeeInputAmount = actualAmountIn
		}
	}

	actualAmountOut := new(big.Int).Set(swapAmount.OutputAmount)
	if !feeMode.FeesOnInput {
		feeResult, err := GetFeeOnAmount(poolState.PoolFees, swapAmount.OutputAmount, tradeFeeNumerator, feeMode.HasReferral)
		if err != nil {
			return swapResultBig{}, err
		}
		protocolFee = feeResult.ProtocolFee
		claimingFee = feeResult.ClaimingFee
		compoundingFee = feeResult.CompoundingFee
		referralFee = feeResult.ReferralFee
		actualAmountOut = feeResult.Amount
	}

	result := swapResultBig{
		includedFeeInputAmount: includedFeeInputAmount,
		excludedFeeInputAmount: actualAmountIn,
		amountLeft:             swapAmount.AmountLeft,
		outputAmount:           actualAmountOut,
		nextSqrtPrice:          swapAmount.NextSqrtPrice,
		claimingFee:            claimingFee,
		compoundingFee:         compoundingFee,
		protocolFee:            protocolFee,
		referralFee:            referralFee,
	}
	if err := applySwapResult(poolState, &result, feeMode, tradeDirection); err != nil {
		return swapResultBig{}, err
	}
	return result, nil
}

func GetSwapResultFromExactOutput(poolState *dammv2gen.Pool, amountOut *big.Int, feeMode shared.FeeMode, tradeDirection shared.TradeDirection, currentPoint *big.Int) (shared.SwapResult2, error) {
	result, err := swapResultFromExactOutput(poolState, amountOut, feeMode, tradeDirection, currentPoint)
	if err != nil {
		return shared.SwapResult2{}, err
	}
	return result.toSwapResult()
}

func swapResultFromExactOutput(poolState *dammv2gen.Pool, amountOut *big.Int, feeMode shared.FeeMode, tradeDirection shared.TradeDirection, currentPoint *big.Int) (swapResultBig, error) {
	handler := GetLiquidityHandler(poolState)
	maxFeeNumerator, err := GetMaxFeeNumerator(shared.PoolVersion(poolState.FeeVersion))
	if err != nil {
		return swapResultBig{}, err
	}
	activationPoint := new(big.Int).SetUint64(poolState.ActivationPoint)
	initSqrtPrice := poolState.PoolFees.InitSqrtPrice.BigInt()
	currentSqrtPrice := poolState.SqrtPrice.BigInt()

	claimingFee := big.NewInt(0)
	compoundingFee := big.NewInt(0)
	protocolFee := big.NewInt(0)
	referralFee := big.NewInt(0)
	includedFeeAmountOut := new(big.Int).Set(amountOut)
	if !feeMode.FeesOnInput {
		tradeFeeNumerator, err := GetTotalTradingFeeFromExcludedFeeAmount(poolState.PoolFees, currentPoint, activationPoint, amountOut, tradeDirection, maxFeeNumerator, initSqrtPrice, currentSqrtPrice)
		if err != nil {
			return swapResultBig{}, err
		}
		includedFeeAmount, feeAmount, err := GetIncludedFeeAmount(tradeFeeNumerator, amountOut)
		if err != nil {
			return swapResultBig{}, err
		}
		split := SplitFees(poolState.PoolFees, feeAmount, feeMode.HasReferral)
		protocolFee = split.ProtocolFee
		claimingFee = split.ClaimingFee
		compoundingFee = split.CompoundingFee
		referralFee = split.ReferralFee
		includedFeeAmountOut = includedFeeAmount
	}

	var swapAmount SwapAmountFromOutput
	if tradeDirection == shared.TradeDirectionAtoB {
		swapAmount, err = handler.CalculateAtoBFromAmountOut(includedFeeAmountOut)
	} else {
		swapAmount, err = handler.CalculateBtoAFromAmountOut(includedFeeAmountOut)
	}
	if err != nil {
		return swapResultBig{}, err
	}

	includedFeeInputAmount := new(big.Int).Set(swapAmount.InputAmount)
	if feeMode.FeesOnInput {
		tradeFeeNumerator, err := GetTotalTradingFeeFromExcludedFeeAmount(poolState.PoolFees, currentPoint, activationPoint, swapAmount.InputAmount, tradeDirection, maxFeeNumerator, initSqrtPrice, currentSqrtPrice)
		if err != nil {
			return swapResultBig{}, err
		}
		includedFeeAmount, feeAmount, err := GetIncludedFeeAmount(tradeFeeNumerator, swapAmount.InputAmount)
		if err != nil {
			return swapResultBig{}, err
		}
		split := SplitFees(poolState.PoolFees, feeAmount, feeMode.HasReferral)
		protocolFee = split.ProtocolFee
		claimingFee = split.ClaimingFee
		compoundingFee = split.CompoundingFee
		referralFee = split.ReferralFee
		includedFeeInputAmount = includedFeeAmount
	}

	result := swapResultBig{
		amountLeft:             big.NewInt(0),
		includedFeeInputAmount: includedFeeInputAmount,
		excludedFeeInputAmount: swapAmount.InputAmount,
		outputAmount:           new(big.Int).Set(amountOut),
		nextSqrtPrice:          swapAmount.NextSqrtPrice,
		claimingFee:            claimingFee,
		compoundingFee:         compoundingFee,
		protocolFee:            protocolFee,
		referralFee:            referralFee,
	}
	if err := applySwapResult(poolState, &result, feeMode, tradeDirection); err != nil {
		return swapResultBig{}, err
	}
	return result, nil
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
	feeMode, err := GetFeeMode(shared.CollectFeeMode(pool.CollectFeeMode), tradeDirection, hasReferral)
	if err != nil {
		return shared.Quote2Result{}, err
	}
	actualAmountIn := new(big.Int).Set(amountIn)
	if inputTokenInfo != nil {
		actualAmountIn = helpers.CalculateTransferFeeExcludedAmount(amountIn, inputTokenInfo).Amount
	}
	raw, err := swapResultFromExactInput(pool, actualAmountIn, feeMode, tradeDirection, currentPoint)
	if err != nil {
		return shared.Quote2Result{}, err
	}
	swapResult, err := raw.toSwapResult()
	if err != nil {
		return shared.Quote2Result{}, err
	}
	actualAmountOut := new(big.Int).Set(raw.outputAmount)
	if outputTokenInfo != nil {
		actualAmountOut = helpers.CalculateTransferFeeExcludedAmount(raw.outputAmount, outputTokenInfo).Amount
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
	feeMode, err := GetFeeMode(shared.CollectFeeMode(pool.CollectFeeMode), tradeDirection, hasReferral)
	if err != nil {
		return shared.Quote2Result{}, err
	}
	actualAmountOut := new(big.Int).Set(amountOut)
	if outputTokenInfo != nil {
		actualAmountOut = helpers.CalculateTransferFeeIncludedAmount(amountOut, outputTokenInfo).Amount
	}
	raw, err := swapResultFromExactOutput(pool, actualAmountOut, feeMode, tradeDirection, currentPoint)
	if err != nil {
		return shared.Quote2Result{}, err
	}
	swapResult, err := raw.toSwapResult()
	if err != nil {
		return shared.Quote2Result{}, err
	}
	actualAmountIn := new(big.Int).Set(raw.includedFeeInputAmount)
	if inputTokenInfo != nil {
		actualAmountIn = helpers.CalculateTransferFeeIncludedAmount(raw.includedFeeInputAmount, inputTokenInfo).Amount
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
	feeMode, err := GetFeeMode(shared.CollectFeeMode(pool.CollectFeeMode), tradeDirection, hasReferral)
	if err != nil {
		return shared.Quote2Result{}, err
	}
	actualAmountIn := new(big.Int).Set(amountIn)
	if inputTokenInfo != nil {
		actualAmountIn = helpers.CalculateTransferFeeExcludedAmount(amountIn, inputTokenInfo).Amount
	}
	raw, err := swapResultFromPartialInput(pool, actualAmountIn, feeMode, tradeDirection, currentPoint)
	if err != nil {
		return shared.Quote2Result{}, err
	}
	swapResult, err := raw.toSwapResult()
	if err != nil {
		return shared.Quote2Result{}, err
	}
	actualAmountOut := new(big.Int).Set(raw.outputAmount)
	if outputTokenInfo != nil {
		actualAmountOut = helpers.CalculateTransferFeeExcludedAmount(raw.outputAmount, outputTokenInfo).Amount
	}
	minimumAmountOut := helpers.GetAmountWithSlippage(actualAmountOut, slippage, shared.SwapModePartialFill)
	priceImpact, _ := helpers.GetPriceImpact(raw.includedFeeInputAmount, actualAmountOut, pool.SqrtPrice.BigInt(), aToB, tokenADecimal, tokenBDecimal)
	return shared.Quote2Result{SwapResult2: swapResult, MinimumAmountOut: minimumAmountOut, PriceImpact: priceImpact}, nil
}

func isSwapEnabled(pool *dammv2gen.Pool, currentPoint *big.Int) bool {
	return pool.PoolStatus == uint8(shared.PoolStatusEnable) && currentPoint.Cmp(new(big.Int).SetUint64(pool.ActivationPoint)) >= 0
}
