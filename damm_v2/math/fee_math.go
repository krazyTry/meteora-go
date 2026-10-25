package math

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/math/pool_fees"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

func ToNumerator(bps, feeDenominator *big.Int) *big.Int {
	return MulDiv(bps, feeDenominator, big.NewInt(shared.BasisPointMax), shared.RoundingDown)
}

func GetFeeInPeriod(cliffFeeNumerator, reductionFactor *big.Int, passedPeriod int) *big.Int {
	if reductionFactor.Sign() == 0 {
		return new(big.Int).Set(cliffFeeNumerator)
	}
	bps := new(big.Int).Lsh(reductionFactor, shared.ScaleOffset)
	bps.Div(bps, big.NewInt(shared.BasisPointMax))
	base := new(big.Int).Sub(shared.OneQ64, bps)
	result := Pow(base, big.NewInt(int64(passedPeriod)))
	if result.Cmp(shared.U128Max) > 0 {
		return big.NewInt(0)
	}
	fee := new(big.Int).Mul(result, cliffFeeNumerator)
	fee.Rsh(fee, shared.ScaleOffset)
	return fee
}

func GetFeeMode(collectFeeMode shared.CollectFeeMode, tradeDirection shared.TradeDirection, hasReferral bool) (shared.FeeMode, error) {
	feesOnInput := false
	feesOnTokenA := false
	switch collectFeeMode {
	case shared.CollectFeeModeBothToken:
		if tradeDirection == shared.TradeDirectionBtoA {
			feesOnTokenA = true
		}
	case shared.CollectFeeModeOnlyB, shared.CollectFeeModeCompounding:
		if tradeDirection == shared.TradeDirectionBtoA {
			feesOnInput = true
		}
	default:
		return shared.FeeMode{}, errors.New("invalid collect fee mode")
	}
	return shared.FeeMode{FeesOnInput: feesOnInput, FeesOnTokenA: feesOnTokenA, HasReferral: hasReferral}, nil
}

func GetTotalFeeNumerator(poolFees dammv2gen.PoolFeesStruct, baseFeeNumerator, maxFeeNumerator *big.Int) *big.Int {
	dynamicFeeNumerator := big.NewInt(0)
	if poolFees.DynamicFee.Initialized != 0 {
		dynamicFeeNumerator = pool_fees.GetDynamicFeeNumerator(
			poolFees.DynamicFee.VolatilityAccumulator.BigInt(),
			big.NewInt(int64(poolFees.DynamicFee.BinStep)),
			big.NewInt(int64(poolFees.DynamicFee.VariableFeeControl)),
		)
	}
	totalFee := new(big.Int).Add(dynamicFeeNumerator, baseFeeNumerator)
	if totalFee.Cmp(maxFeeNumerator) > 0 {
		return new(big.Int).Set(maxFeeNumerator)
	}
	return totalFee
}

func GetTotalTradingFeeFromIncludedFeeAmount(poolFees dammv2gen.PoolFeesStruct, currentPoint, activationPoint, includedFeeAmount *big.Int, tradeDirection shared.TradeDirection, maxFeeNumerator *big.Int, initSqrtPrice, currentSqrtPrice *big.Int) (*big.Int, error) {
	baseFeeHandler, err := pool_fees.GetBaseFeeHandler(poolFees.BaseFee.BaseFeeInfo.Data[:])
	if err != nil {
		return nil, err
	}
	baseFeeNumerator, err := baseFeeHandler.GetBaseFeeNumeratorFromIncludedFeeAmount(currentPoint, activationPoint, tradeDirection, includedFeeAmount, initSqrtPrice, currentSqrtPrice)
	if err != nil {
		return nil, err
	}
	return GetTotalFeeNumerator(poolFees, baseFeeNumerator, maxFeeNumerator), nil
}

func GetTotalTradingFeeFromExcludedFeeAmount(poolFees dammv2gen.PoolFeesStruct, currentPoint, activationPoint, excludedFeeAmount *big.Int, tradeDirection shared.TradeDirection, maxFeeNumerator *big.Int, initSqrtPrice, currentSqrtPrice *big.Int) (*big.Int, error) {
	baseFeeHandler, err := pool_fees.GetBaseFeeHandler(poolFees.BaseFee.BaseFeeInfo.Data[:])
	if err != nil {
		return nil, err
	}
	baseFeeNumerator, err := baseFeeHandler.GetBaseFeeNumeratorFromExcludedFeeAmount(currentPoint, activationPoint, tradeDirection, excludedFeeAmount, initSqrtPrice, currentSqrtPrice)
	if err != nil {
		return nil, err
	}
	return GetTotalFeeNumerator(poolFees, baseFeeNumerator, maxFeeNumerator), nil
}

func SplitFees(poolFees dammv2gen.PoolFeesStruct, feeAmount *big.Int, hasReferral bool) shared.SplitFees {
	protocolFee := MulDiv(feeAmount, big.NewInt(int64(poolFees.ProtocolFeePercent)), big.NewInt(100), shared.RoundingDown)
	tradingFee := new(big.Int).Sub(feeAmount, protocolFee)

	compoundingFee := big.NewInt(0)
	claimingFee := new(big.Int).Set(tradingFee)
	if poolFees.CompoundingFeeBps > 0 {
		compoundingFee = MulDiv(tradingFee, big.NewInt(int64(poolFees.CompoundingFeeBps)), big.NewInt(shared.BasisPointMax), shared.RoundingDown)
		claimingFee = new(big.Int).Sub(tradingFee, compoundingFee)
	}

	referralFee := big.NewInt(0)
	if hasReferral {
		referralFee = MulDiv(protocolFee, big.NewInt(int64(poolFees.ReferralFeePercent)), big.NewInt(100), shared.RoundingDown)
	}
	protocolFee = new(big.Int).Sub(protocolFee, referralFee)
	return shared.SplitFees{
		ClaimingFee:    claimingFee,
		CompoundingFee: compoundingFee,
		ProtocolFee:    protocolFee,
		ReferralFee:    referralFee,
	}
}

func GetFeeOnAmount(poolFees dammv2gen.PoolFeesStruct, amount, tradeFeeNumerator *big.Int, hasReferral bool) (shared.FeeOnAmountResult, error) {
	excludedFeeAmount, tradingFee := GetExcludedFeeAmount(tradeFeeNumerator, amount)
	split := SplitFees(poolFees, tradingFee, hasReferral)
	return shared.FeeOnAmountResult{
		Amount:         excludedFeeAmount,
		ClaimingFee:    split.ClaimingFee,
		CompoundingFee: split.CompoundingFee,
		ProtocolFee:    split.ProtocolFee,
		ReferralFee:    split.ReferralFee,
	}, nil
}

func GetExcludedFeeAmount(tradeFeeNumerator, includedFeeAmount *big.Int) (*big.Int, *big.Int) {
	tradingFee := MulDiv(includedFeeAmount, tradeFeeNumerator, big.NewInt(shared.FeeDenominator), shared.RoundingUp)
	excluded := new(big.Int).Sub(includedFeeAmount, tradingFee)
	return excluded, tradingFee
}

func GetIncludedFeeAmount(tradeFeeNumerator, excludedFeeAmount *big.Int) (*big.Int, *big.Int, error) {
	denominator := new(big.Int).Sub(big.NewInt(shared.FeeDenominator), tradeFeeNumerator)
	if denominator.Sign() <= 0 {
		return nil, nil, errors.New("invalid fee numerator")
	}
	included := MulDiv(excludedFeeAmount, big.NewInt(shared.FeeDenominator), denominator, shared.RoundingUp)
	feeAmount := new(big.Int).Sub(included, excludedFeeAmount)
	return included, feeAmount, nil
}

func GetMaxFeeNumerator(feeVersion shared.PoolVersion) (*big.Int, error) {
	switch feeVersion {
	case shared.PoolVersionV0:
		return big.NewInt(shared.MaxFeeNumeratorV0), nil
	case shared.PoolVersionV1:
		return big.NewInt(shared.MaxFeeNumeratorV1), nil
	default:
		return nil, errors.New("invalid pool version")
	}
}

func GetMaxFeeBps(feeVersion shared.PoolVersion) (uint16, error) {
	switch feeVersion {
	case shared.PoolVersionV0:
		return shared.MaxFeeBpsV0, nil
	case shared.PoolVersionV1:
		return shared.MaxFeeBpsV1, nil
	default:
		return 0, errors.New("invalid pool version")
	}
}
