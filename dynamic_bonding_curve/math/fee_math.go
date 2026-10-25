package math

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/math/pool_fees"
	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func ToNumerator(bps *big.Int, feeDenominator *big.Int) (*big.Int, error) {
	return MulDiv(bps, feeDenominator, big.NewInt(dbc.MaxBasisPoint), dbc.RoundingDown)
}

func GetFeeMode(collectFeeMode dbc.CollectFeeMode, tradeDirection dbc.TradeDirection, hasReferral bool) (dbc.FeeMode, error) {
	switch collectFeeMode {
	case dbc.CollectFeeModeQuoteToken:
		switch tradeDirection {
		case dbc.TradeDirectionBaseToQuote:
			return dbc.FeeMode{FeesOnInput: false, FeesOnBaseToken: false, HasReferral: hasReferral}, nil
		case dbc.TradeDirectionQuoteToBase:
			return dbc.FeeMode{FeesOnInput: true, FeesOnBaseToken: false, HasReferral: hasReferral}, nil
		}
	case dbc.CollectFeeModeOutputToken:
		switch tradeDirection {
		case dbc.TradeDirectionBaseToQuote:
			return dbc.FeeMode{FeesOnInput: false, FeesOnBaseToken: false, HasReferral: hasReferral}, nil
		case dbc.TradeDirectionQuoteToBase:
			return dbc.FeeMode{FeesOnInput: false, FeesOnBaseToken: true, HasReferral: hasReferral}, nil
		}
	}
	return dbc.FeeMode{}, errors.New(dbc.ErrInvalidCollectFeeMode)
}

func GetTotalFeeNumeratorFromIncludedFeeAmount(poolFees dbc.PoolFeesConfig, volatilityTracker dbc.VolatilityTracker, currentPoint, activationPoint, includedFeeAmount *big.Int, tradeDirection dbc.TradeDirection) (*big.Int, error) {
	baseFeeHandler, err := pool_fees.GetBaseFeeHandler(new(big.Int).SetUint64(poolFees.BaseFee.CliffFeeNumerator), poolFees.BaseFee.FirstFactor, new(big.Int).SetUint64(poolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(poolFees.BaseFee.ThirdFactor), dbc.BaseFeeMode(poolFees.BaseFee.BaseFeeMode))
	if err != nil {
		return nil, err
	}
	baseFeeNumerator := baseFeeHandler.GetBaseFeeNumeratorFromIncludedFeeAmount(currentPoint, activationPoint, tradeDirection, includedFeeAmount)
	return GetTotalFeeNumerator(baseFeeNumerator, poolFees.DynamicFee, volatilityTracker), nil
}

func GetTotalFeeNumeratorFromExcludedFeeAmount(poolFees dbc.PoolFeesConfig, volatilityTracker dbc.VolatilityTracker, currentPoint, activationPoint, excludedFeeAmount *big.Int, tradeDirection dbc.TradeDirection) (*big.Int, error) {
	baseFeeHandler, err := pool_fees.GetBaseFeeHandler(new(big.Int).SetUint64(poolFees.BaseFee.CliffFeeNumerator), poolFees.BaseFee.FirstFactor, new(big.Int).SetUint64(poolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(poolFees.BaseFee.ThirdFactor), dbc.BaseFeeMode(poolFees.BaseFee.BaseFeeMode))
	if err != nil {
		return nil, err
	}
	baseFeeNumerator := baseFeeHandler.GetBaseFeeNumeratorFromExcludedFeeAmount(currentPoint, activationPoint, tradeDirection, excludedFeeAmount)
	return GetTotalFeeNumerator(baseFeeNumerator, poolFees.DynamicFee, volatilityTracker), nil
}

func GetTotalFeeNumerator(baseFeeNumerator *big.Int, dynamicFee dbc.DynamicFeeConfig, volatilityTracker dbc.VolatilityTracker) *big.Int {
	variableFeeNumerator := pool_fees.GetVariableFeeNumerator(dynamicFee, volatilityTracker)
	total := new(big.Int).Add(variableFeeNumerator, baseFeeNumerator)
	maxFee := big.NewInt(dbc.MaxFeeNumerator)
	if total.Cmp(maxFee) > 0 {
		return maxFee
	}
	return total
}

func GetFeeOnAmount(tradeFeeNumerator, amount *big.Int, hasReferral bool) (dbc.FeeOnAmountResult, error) {
	amountAfterFee, feeAmount, err := GetExcludedFeeAmount(tradeFeeNumerator, amount)
	if err != nil {
		return dbc.FeeOnAmountResult{}, err
	}
	tradingFee, protocolFee, referralFee, err := SplitFees(feeAmount, hasReferral)
	if err != nil {
		return dbc.FeeOnAmountResult{}, err
	}
	return dbc.FeeOnAmountResult{
		Amount:      amountAfterFee,
		ProtocolFee: protocolFee,
		ReferralFee: referralFee,
		TradingFee:  tradingFee,
	}, nil
}

func GetExcludedFeeAmount(tradeFeeNumerator, includedFeeAmount *big.Int) (*big.Int, *big.Int, error) {
	tradingFee, err := MulDiv(includedFeeAmount, tradeFeeNumerator, big.NewInt(dbc.FeeDenominator), dbc.RoundingUp)
	if err != nil {
		return nil, nil, err
	}
	excluded, err := Sub(includedFeeAmount, tradingFee)
	if err != nil {
		return nil, nil, err
	}
	return excluded, tradingFee, nil
}

func GetIncludedFeeAmount(tradeFeeNumerator, excludedFeeAmount *big.Int) (*big.Int, *big.Int, error) {
	denom, err := Sub(big.NewInt(dbc.FeeDenominator), tradeFeeNumerator)
	if err != nil {
		return nil, nil, err
	}
	included, err := MulDiv(excludedFeeAmount, big.NewInt(dbc.FeeDenominator), denom, dbc.RoundingUp)
	if err != nil {
		return nil, nil, err
	}
	if included.Cmp(dbc.U64Max) > 0 {
		return nil, nil, errors.New(dbc.ErrTypeCastFailed)
	}
	feeAmount, err := Sub(included, excludedFeeAmount)
	if err != nil {
		return nil, nil, err
	}
	return included, feeAmount, nil
}

func SplitFees(feeAmount *big.Int, hasReferral bool) (*big.Int, *big.Int, *big.Int, error) {
	protocolFee, err := MulDiv(feeAmount, big.NewInt(dbc.ProtocolFeePercent), big.NewInt(100), dbc.RoundingDown)
	if err != nil {
		return nil, nil, nil, err
	}
	tradingFee, err := Sub(feeAmount, protocolFee)
	if err != nil {
		return nil, nil, nil, err
	}
	referralFee := big.NewInt(0)
	if hasReferral {
		referralFee, err = MulDiv(protocolFee, big.NewInt(dbc.HostFeePercent), big.NewInt(100), dbc.RoundingDown)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	protocolAfterReferral, err := Sub(protocolFee, referralFee)
	if err != nil {
		return nil, nil, nil, err
	}
	return tradingFee, protocolAfterReferral, referralFee, nil
}

// guard for zero or negative denominator in GetIncludedFeeAmount use.
func ensurePositive(v *big.Int) error {
	if v.Sign() <= 0 {
		return errors.New("invalid denominator")
	}
	return nil
}
