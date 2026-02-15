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
	if result.Cmp(shared.MaxU128) > 0 {
		return big.NewInt(0)
	}
	fee := new(big.Int).Mul(result, cliffFeeNumerator)
	fee.Rsh(fee, shared.ScaleOffset)
	return fee
}

func GetFeeMode(collectFeeMode shared.CollectFeeMode, tradeDirection shared.TradeDirection, hasReferral bool) shared.FeeMode {
	feesOnInput := false
	feesOnTokenA := false
	if collectFeeMode == shared.CollectFeeModeBothToken {
		if tradeDirection == shared.TradeDirectionAtoB {
			feesOnInput = false
			feesOnTokenA = false
		} else {
			feesOnInput = false
			feesOnTokenA = true
		}
	} else {
		if tradeDirection == shared.TradeDirectionAtoB {
			feesOnInput = false
			feesOnTokenA = false
		} else {
			feesOnInput = true
			feesOnTokenA = false
		}
	}
	return shared.FeeMode{FeesOnInput: feesOnInput, FeesOnTokenA: feesOnTokenA, HasReferral: hasReferral}
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

func SplitFees(poolFees dammv2gen.PoolFeesStruct, feeAmount *big.Int, hasReferral bool, hasPartner bool) shared.SplitFees {
	protocolFee := new(big.Int).Mul(feeAmount, big.NewInt(int64(poolFees.ProtocolFeePercent)))
	protocolFee.Div(protocolFee, big.NewInt(100))
	tradingFee := new(big.Int).Sub(feeAmount, protocolFee)
	referralFee := big.NewInt(0)
	if hasReferral {
		referralFee = new(big.Int).Mul(protocolFee, big.NewInt(int64(poolFees.ReferralFeePercent)))
		referralFee.Div(referralFee, big.NewInt(100))
	}
	protocolFeeAfterReferral := new(big.Int).Sub(protocolFee, referralFee)
	partnerFee := big.NewInt(0)
	if hasPartner && poolFees.PartnerFeePercent > 0 {
		partnerFee = new(big.Int).Mul(protocolFeeAfterReferral, big.NewInt(int64(poolFees.PartnerFeePercent)))
		partnerFee.Div(partnerFee, big.NewInt(100))
	}
	finalProtocolFee := new(big.Int).Sub(protocolFeeAfterReferral, partnerFee)
	return shared.SplitFees{TradingFee: tradingFee, ProtocolFee: finalProtocolFee, ReferralFee: referralFee, PartnerFee: partnerFee}
}

func GetFeeOnAmount(poolFees dammv2gen.PoolFeesStruct, amount, tradeFeeNumerator *big.Int, hasReferral, hasPartner bool) (shared.FeeOnAmountResult, error) {
	excludedFeeAmount, tradingFee := GetExcludedFeeAmount(tradeFeeNumerator, amount)
	split := SplitFees(poolFees, tradingFee, hasReferral, hasPartner)
	return shared.FeeOnAmountResult{FeeNumerator: tradeFeeNumerator, FeeAmount: tradingFee, AmountAfterFee: excludedFeeAmount, TradingFee: split.TradingFee, ProtocolFee: split.ProtocolFee, ReferralFee: split.ReferralFee, PartnerFee: split.PartnerFee}, nil
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

func GetMaxFeeNumerator(poolVersion shared.PoolVersion) *big.Int {
	switch poolVersion {
	case shared.PoolVersionV0:
		return big.NewInt(shared.MaxFeeNumeratorV0)
	case shared.PoolVersionV1:
		return big.NewInt(shared.MaxFeeNumeratorV1)
	default:
		return big.NewInt(0)
	}
}

func GetMaxFeeBps(poolVersion shared.PoolVersion) uint16 {
	switch poolVersion {
	case shared.PoolVersionV0:
		return shared.MaxFeeBpsV0
	case shared.PoolVersionV1:
		return shared.MaxFeeBpsV1
	default:
		return 0
	}
}
