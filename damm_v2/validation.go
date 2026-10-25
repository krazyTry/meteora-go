package dammv2

import (
	"errors"
	"math/big"

	solanago "github.com/solana-foundation/solana-go/v2"

	"github.com/krazyTry/meteora-go/damm_v2/helpers"
	"github.com/krazyTry/meteora-go/damm_v2/math/pool_fees"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

const u24Max uint32 = 16_777_215

func ValidateFeeTimeScheduler(numberOfPeriod uint16, periodFrequency, reductionFactor, cliffFeeNumerator *big.Int, baseFeeMode BaseFeeMode, poolVersion PoolVersion) bool {
	return pool_fees.ValidateFeeTimeScheduler(numberOfPeriod, periodFrequency, reductionFactor, cliffFeeNumerator, baseFeeMode, poolVersion)
}

func ValidateFeeTimeSchedulerBaseFeeIsStatic(currentPoint, activationPoint, numberOfPeriod, periodFrequency *big.Int) bool {
	return pool_fees.ValidateFeeTimeSchedulerBaseFeeIsStatic(currentPoint, activationPoint, numberOfPeriod, periodFrequency)
}

func ValidateFeeMarketCapScheduler(cliffFeeNumerator *big.Int, numberOfPeriod uint16, sqrtPriceStepBps *big.Int, reductionFactor *big.Int, schedulerExpirationDuration *big.Int, feeMarketCapSchedulerMode BaseFeeMode, poolVersion PoolVersion) bool {
	return pool_fees.ValidateFeeMarketCapScheduler(cliffFeeNumerator, numberOfPeriod, sqrtPriceStepBps, reductionFactor, schedulerExpirationDuration, feeMarketCapSchedulerMode, poolVersion)
}

func ValidateFeeMarketCapBaseFeeIsStatic(currentPoint, activationPoint, schedulerExpirationDuration *big.Int) bool {
	return pool_fees.ValidateFeeMarketCapBaseFeeIsStatic(currentPoint, activationPoint, schedulerExpirationDuration)
}

func ValidateFeeRateLimiter(cliffFeeNumerator *big.Int, feeIncrementBps uint16, maxFeeBps uint16, maxLimiterDuration uint32, referenceAmount *big.Int, collectFeeMode shared.CollectFeeMode, activationType ActivationType, poolVersion PoolVersion) bool {
	return pool_fees.ValidateFeeRateLimiter(cliffFeeNumerator, feeIncrementBps, maxFeeBps, maxLimiterDuration, referenceAmount, collectFeeMode, activationType, poolVersion)
}

func ValidateFeeRateLimiterBaseFeeIsStatic(currentPoint, activationPoint *big.Int, maxLimiterDuration uint32, referenceAmount *big.Int, maxFeeBps uint16, feeIncrementBps uint16) bool {
	return pool_fees.ValidateFeeRateLimiterBaseFeeIsStatic(currentPoint, activationPoint, maxLimiterDuration, referenceAmount, maxFeeBps, feeIncrementBps)
}

func ValidateFeeFraction(numerator, denominator *big.Int) error {
	return pool_fees.ValidateFeeFraction(numerator, denominator)
}

func ValidateCollectFeeMode(collectFeeMode CollectFeeMode) error {
	switch collectFeeMode {
	case CollectFeeModeBothToken, CollectFeeModeOnlyB, CollectFeeModeCompounding:
		return nil
	default:
		return errors.New("invalid collect fee mode")
	}
}

func ValidateCompoundingFee(collectFeeMode CollectFeeMode, compoundingFeeBps uint16) error {
	if collectFeeMode == CollectFeeModeCompounding {
		if compoundingFeeBps > shared.BasisPointMax {
			return errors.New("invalid compounding fee bps")
		}
		return nil
	}
	if compoundingFeeBps != 0 {
		return errors.New("invalid compounding fee bps")
	}
	return nil
}

func AssertBaseFeeModeAllowedForNewPool(baseFeeMode BaseFeeMode) error {
	if baseFeeMode == BaseFeeModeRateLimiter {
		return errors.New("BaseFeeMode.RateLimiter is deprecated. New configs and pools cannot use RateLimiter. Existing RateLimiter pools are still supported for swaps and quotes.")
	}
	return nil
}

func ValidateActivationType(activationType ActivationType) error {
	switch activationType {
	case ActivationTypeSlot, ActivationTypeTimestamp:
		return nil
	default:
		return errors.New("invalid activation type")
	}
}

func ValidatePriceRange(collectFeeMode CollectFeeMode, sqrtMinPrice, sqrtMaxPrice *big.Int) error {
	if collectFeeMode == CollectFeeModeCompounding {
		return nil
	}
	minPrice := nonNilBig(sqrtMinPrice)
	maxPrice := nonNilBig(sqrtMaxPrice)
	if minPrice.Cmp(shared.MinSqrtPrice) < 0 || maxPrice.Cmp(shared.MaxSqrtPrice) > 0 || minPrice.Cmp(maxPrice) >= 0 {
		return errors.New("invalid price range")
	}
	return nil
}

func ValidateInitialSqrtPrice(collectFeeMode CollectFeeMode, initSqrtPrice, sqrtMinPrice, sqrtMaxPrice *big.Int) error {
	price := nonNilBig(initSqrtPrice)
	if collectFeeMode == CollectFeeModeCompounding {
		if price.Cmp(shared.MinSqrtPrice) < 0 || price.Cmp(shared.MaxSqrtPrice) > 0 {
			return errors.New("invalid price range")
		}
		return nil
	}
	if price.Cmp(nonNilBig(sqrtMinPrice)) < 0 || price.Cmp(nonNilBig(sqrtMaxPrice)) > 0 {
		return errors.New("invalid price range")
	}
	return nil
}

func ValidateDynamicFee(dynamicFee *DynamicFee) error {
	if dynamicFee == nil {
		return errors.New("invalid dynamic fee parameters")
	}
	if dynamicFee.BinStep != shared.BinStepBpsDefault {
		return errors.New("invalid dynamic fee parameters")
	}
	if dynamicFee.BinStepU128.BigInt().Cmp(shared.BinStepBpsU128Default) != 0 {
		return errors.New("invalid dynamic fee parameters")
	}
	if dynamicFee.FilterPeriod >= dynamicFee.DecayPeriod {
		return errors.New("invalid dynamic fee parameters")
	}
	if uint32(dynamicFee.ReductionFactor) > shared.BasisPointMax {
		return errors.New("invalid dynamic fee parameters")
	}
	if dynamicFee.VariableFeeControl > u24Max || dynamicFee.MaxVolatilityAccumulator > u24Max {
		return errors.New("invalid dynamic fee parameters")
	}
	return nil
}

func ValidatePoolFees(poolFees PoolFeesParams, collectFeeMode CollectFeeMode, activationType ActivationType, feeVersion ...PoolVersion) error {
	version := PoolVersion(shared.CurrentPoolVersion)
	if len(feeVersion) > 0 {
		version = feeVersion[0]
	}
	if err := ValidateCompoundingFee(collectFeeMode, poolFees.CompoundingFeeBps); err != nil {
		return err
	}
	baseFeeMode, err := helpers.GetBaseFeeModeFromBorshData(poolFees.BaseFee.Data[:])
	if err != nil {
		return err
	}
	if err := AssertBaseFeeModeAllowedForNewPool(baseFeeMode); err != nil {
		return err
	}
	handler, err := pool_fees.GetBaseFeeHandlerFromBorsh(poolFees.BaseFee.Data[:])
	if err != nil {
		return err
	}
	if !handler.Validate(collectFeeMode, activationType, version) {
		return errors.New("invalid base fee")
	}
	if poolFees.DynamicFee != nil {
		if err := ValidateDynamicFee(poolFees.DynamicFee); err != nil {
			return err
		}
	}
	return nil
}

func ValidateCustomizablePoolParams(params InitializeCustomizeablePoolParams) error {
	if err := ValidateTokenMints(params.TokenAMint, params.TokenBMint); err != nil {
		return err
	}
	if err := ValidateActivationType(params.ActivationType); err != nil {
		return err
	}
	if err := ValidateCollectFeeMode(params.CollectFeeMode); err != nil {
		return err
	}
	if err := ValidatePriceRange(params.CollectFeeMode, params.SqrtMinPrice, params.SqrtMaxPrice); err != nil {
		return err
	}
	if err := ValidateInitialSqrtPrice(params.CollectFeeMode, params.InitSqrtPrice, params.SqrtMinPrice, params.SqrtMaxPrice); err != nil {
		return err
	}
	if nonNilBig(params.LiquidityDelta).Sign() <= 0 {
		return errors.New("invalid minimum liquidity")
	}
	if params.CollectFeeMode == CollectFeeModeCompounding && nonNilBig(params.LiquidityDelta).Cmp(shared.DeadLiquidity) <= 0 {
		return errors.New("Compounding pool liquidity must be greater than DEAD_LIQUIDITY")
	}
	if nonNilBig(params.TokenAAmount).Sign() <= 0 && nonNilBig(params.TokenBAmount).Sign() <= 0 {
		return errors.New("amount is zero")
	}
	return ValidatePoolFees(params.PoolFees, params.CollectFeeMode, params.ActivationType)
}

func ValidateTokenMints(tokenAMint, tokenBMint solanago.PublicKey) error {
	if tokenAMint.Equals(tokenBMint) {
		return errors.New("token mints must be different")
	}
	if err := helpers.ValidateNotNativeMintToken2022(tokenAMint); err != nil {
		return err
	}
	return helpers.ValidateNotNativeMintToken2022(tokenBMint)
}

func ValidateCreatePoolParams(params CreatePoolParams) error {
	if err := ValidateTokenMints(params.TokenAMint, params.TokenBMint); err != nil {
		return err
	}
	if nonNilBig(params.LiquidityDelta).Sign() <= 0 {
		return errors.New("invalid minimum liquidity")
	}
	if nonNilBig(params.TokenAAmount).Sign() <= 0 && nonNilBig(params.TokenBAmount).Sign() <= 0 {
		return errors.New("amount is zero")
	}
	return nil
}

func ValidateAddLiquidityParams(liquidityDelta *big.Int) error {
	if nonNilBig(liquidityDelta).Sign() <= 0 {
		return errors.New("liquidityDelta must be greater than 0")
	}
	return nil
}

func ValidateRemoveLiquidityParams(liquidityDelta *big.Int) error {
	if nonNilBig(liquidityDelta).Sign() <= 0 {
		return errors.New("liquidityDelta must be greater than 0")
	}
	return nil
}

func ValidateSplitPositionParams(permanentLockedLiquidityPercentage, unlockedLiquidityPercentage, feeAPercentage, feeBPercentage, reward0Percentage, reward1Percentage, innerVestingLiquidityPercentage uint8) error {
	percentages := []uint8{
		permanentLockedLiquidityPercentage,
		unlockedLiquidityPercentage,
		feeAPercentage,
		feeBPercentage,
		reward0Percentage,
		reward1Percentage,
		innerVestingLiquidityPercentage,
	}
	allZero := true
	for _, pct := range percentages {
		if pct > 100 {
			return errors.New("Each percentage must be <= 100")
		}
		if pct != 0 {
			allZero = false
		}
	}
	if allZero {
		return errors.New("At least one percentage must be greater than 0")
	}
	return nil
}

func ValidateSplitPosition2Params(numerator uint32) error {
	if numerator == 0 || numerator > shared.SplitPositionDenominator {
		return errors.New("numerator must be in (0, 1000000000]")
	}
	return nil
}

func ValidateLockPositionParams(numberOfPeriod uint16, periodFrequency, cliffUnlockLiquidity, liquidityPerPeriod *big.Int) error {
	if numberOfPeriod > 0 {
		if nonNilBig(periodFrequency).Sign() <= 0 || nonNilBig(liquidityPerPeriod).Sign() <= 0 {
			return errors.New("periodFrequency and liquidityPerPeriod must be greater than 0 when numberOfPeriod > 0")
		}
	}
	total := new(big.Int).Set(nonNilBig(cliffUnlockLiquidity))
	total.Add(total, new(big.Int).Mul(nonNilBig(liquidityPerPeriod), big.NewInt(int64(numberOfPeriod))))
	if total.Sign() <= 0 {
		return errors.New("Total lock amount must be greater than 0")
	}
	return nil
}

func ValidateRewardIndex(rewardIndex uint8) error {
	if int(rewardIndex) >= shared.NumRewards {
		return errors.New("rewardIndex must be in [0, 2)")
	}
	return nil
}

func ValidateRewardDuration(rewardDuration *big.Int) error {
	duration := nonNilBig(rewardDuration)
	if duration.Cmp(big.NewInt(shared.MinRewardDuration)) < 0 || duration.Cmp(big.NewInt(shared.MaxRewardDuration)) > 0 {
		return errors.New("rewardDuration must be between 86400 and 31536000 seconds")
	}
	return nil
}

func nonNilBig(v *big.Int) *big.Int {
	if v == nil {
		return big.NewInt(0)
	}
	return v
}
