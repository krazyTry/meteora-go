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

func GetFeeMode(collectFeeMode dbc.CollectFeeMode, tradeDirection dbc.TradeDirection, hasReferral bool) dbc.FeeMode {
	feesOnInput := false
	feesOnBaseToken := false

	if collectFeeMode == dbc.CollectFeeModeOutputToken {
		if tradeDirection == dbc.TradeDirectionQuoteToBase {
			feesOnInput = false
			feesOnBaseToken = true
		}
	} else {
		if tradeDirection == dbc.TradeDirectionQuoteToBase {
			feesOnInput = true
			feesOnBaseToken = false
		}
	}

	return dbc.FeeMode{FeesOnInput: feesOnInput, FeesOnBaseToken: feesOnBaseToken, HasReferral: hasReferral}
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

func GetFeeOnAmount(tradeFeeNumerator, amount *big.Int, poolFees dbc.PoolFeesConfig, hasReferral bool) (dbc.FeeOnAmountResult, error) {
	amountAfterFee, tradingFee, err := GetExcludedFeeAmount(tradeFeeNumerator, amount)
	if err != nil {
		return dbc.FeeOnAmountResult{}, err
	}
	protocolFee, err := MulDiv(tradingFee, big.NewInt(dbc.ProtocolFeePercent), big.NewInt(100), dbc.RoundingDown)
	if err != nil {
		return dbc.FeeOnAmountResult{}, err
	}
	updatedTradingFee, err := Sub(tradingFee, protocolFee)
	if err != nil {
		return dbc.FeeOnAmountResult{}, err
	}
	referralFee := big.NewInt(0)
	if hasReferral {
		referralFee, err = MulDiv(protocolFee, big.NewInt(dbc.HostFeePercent), big.NewInt(100), dbc.RoundingDown)
		if err != nil {
			return dbc.FeeOnAmountResult{}, err
		}
	}
	updatedProtocolFee, err := Sub(protocolFee, referralFee)
	if err != nil {
		return dbc.FeeOnAmountResult{}, err
	}
	return dbc.FeeOnAmountResult{
		Amount:      amountAfterFee,
		ProtocolFee: updatedProtocolFee,
		ReferralFee: referralFee,
		TradingFee:  updatedTradingFee,
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
	feeAmount, err := Sub(included, excludedFeeAmount)
	if err != nil {
		return nil, nil, err
	}
	return included, feeAmount, nil
}

func SplitFees(poolFees dbc.PoolFeesConfig, feeAmount *big.Int, hasReferral bool) (*big.Int, *big.Int, *big.Int, error) {
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
