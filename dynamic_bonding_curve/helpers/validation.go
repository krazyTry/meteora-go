package helpers

import (
	"context"
	"errors"
	"math"
	"math/big"


	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

func ValidateFeeScheduler(numberOfPeriod uint16, periodFrequency, reductionFactor, cliffFeeNumerator *big.Int, baseFeeMode BaseFeeMode) bool {
	if periodFrequency.Sign() != 0 || numberOfPeriod != 0 || reductionFactor.Sign() != 0 {
		if numberOfPeriod == 0 || periodFrequency.Sign() == 0 || reductionFactor.Sign() == 0 {
			return false
		}
	}
	minFeeNumerator, err := GetFeeSchedulerMinBaseFeeNumerator(cliffFeeNumerator, numberOfPeriod, reductionFactor, baseFeeMode)
	if err != nil {
		return false
	}
	maxFeeNumerator := GetFeeSchedulerMaxBaseFeeNumerator(cliffFeeNumerator)
	if minFeeNumerator.Cmp(big.NewInt(MinFeeNumerator)) < 0 || maxFeeNumerator.Cmp(big.NewInt(MaxFeeNumerator)) > 0 {
		return false
	}
	return true
}

func ValidateFeeRateLimiter(cliffFeeNumerator, feeIncrementBps, maxLimiterDuration, referenceAmount *big.Int, collectFeeMode CollectFeeMode, activationType ActivationType) bool {
	if collectFeeMode != CollectFeeModeQuoteToken {
		return false
	}
	isZero := referenceAmount.Sign() == 0 && maxLimiterDuration.Sign() == 0 && feeIncrementBps.Sign() == 0
	if isZero {
		return true
	}
	isNonZero := referenceAmount.Sign() > 0 && maxLimiterDuration.Sign() > 0 && feeIncrementBps.Sign() > 0
	if !isNonZero {
		return false
	}
	maxLimiterDurationLimit := big.NewInt(MaxRateLimiterDurationInSeconds)
	if activationType == ActivationTypeSlot {
		maxLimiterDurationLimit = big.NewInt(MaxRateLimiterDurationInSlots)
	}
	if maxLimiterDuration.Cmp(maxLimiterDurationLimit) > 0 {
		return false
	}
	feeIncrementNumerator, err := ToNumerator(feeIncrementBps, big.NewInt(FeeDenominator))
	if err != nil {
		return false
	}
	if feeIncrementNumerator.Cmp(big.NewInt(FeeDenominator)) >= 0 {
		return false
	}
	if cliffFeeNumerator.Cmp(big.NewInt(MinFeeNumerator)) < 0 || cliffFeeNumerator.Cmp(big.NewInt(MaxFeeNumerator)) > 0 {
		return false
	}
	minFeeNumerator, err := GetFeeNumeratorFromIncludedAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, big.NewInt(0))
	if err != nil {
		return false
	}
	maxFeeNumerator, err := GetFeeNumeratorFromIncludedAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, big.NewInt(math.MaxInt64))
	if err != nil {
		return false
	}
	return minFeeNumerator.Cmp(big.NewInt(MinFeeNumerator)) >= 0 && maxFeeNumerator.Cmp(big.NewInt(MaxFeeNumerator)) <= 0
}

func ValidatePoolFees(poolFees PoolFeeParameters, collectFeeMode CollectFeeMode, activationType ActivationType) bool {
	if poolFees.BaseFee.CliffFeeNumerator == 0 {
		return false
	}
	if poolFees.BaseFee.CliffFeeNumerator < uint64(MinFeeNumerator) {
		return false
	}
	if poolFees.BaseFee.BaseFeeMode == uint8(BaseFeeModeFeeSchedulerLinear) || poolFees.BaseFee.BaseFeeMode == uint8(BaseFeeModeFeeSchedulerExponential) {
		if !ValidateFeeScheduler(poolFees.BaseFee.FirstFactor, new(big.Int).SetUint64(poolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(poolFees.BaseFee.ThirdFactor), new(big.Int).SetUint64(poolFees.BaseFee.CliffFeeNumerator), BaseFeeMode(poolFees.BaseFee.BaseFeeMode)) {
			return false
		}
	}
	if poolFees.BaseFee.BaseFeeMode == uint8(BaseFeeModeRateLimiter) {
		if !ValidateFeeRateLimiter(new(big.Int).SetUint64(poolFees.BaseFee.CliffFeeNumerator), new(big.Int).SetUint64(uint64(poolFees.BaseFee.FirstFactor)), new(big.Int).SetUint64(poolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(poolFees.BaseFee.ThirdFactor), collectFeeMode, activationType) {
			return false
		}
	}
	if poolFees.DynamicFee != nil {
		if !ValidateDynamicFee(poolFees.DynamicFee) {
			return false
		}
	}
	return true
}

func ValidateDynamicFee(dynamicFee *DynamicFeeParameters) bool {
	if dynamicFee == nil {
		return true
	}
	if dynamicFee.BinStep != uint16(BinStepBpsDefault) {
		return false
	}
	if dynamicFee.BinStepU128.BigInt().Cmp(BinStepBpsU128Default) != 0 {
		return false
	}
	if dynamicFee.FilterPeriod >= dynamicFee.DecayPeriod {
		return false
	}
	if dynamicFee.ReductionFactor > uint16(MaxBasisPoint) {
		return false
	}
	if dynamicFee.VariableFeeControl > uint32(U24Max) {
		return false
	}
	if dynamicFee.MaxVolatilityAccumulator > uint32(U24Max) {
		return false
	}
	return true
}

func ValidateCollectFeeMode(collectFeeMode CollectFeeMode) bool {
	return collectFeeMode == CollectFeeModeQuoteToken || collectFeeMode == CollectFeeModeOutputToken
}

func ValidateMigrationAndTokenType(migrationOption MigrationOption, tokenType TokenType) bool {
	if migrationOption == MigrationOptionMetDamm {
		return tokenType == TokenTypeSPL
	}
	return true
}

func ValidateActivationType(activationType ActivationType) bool {
	return activationType == ActivationTypeSlot || activationType == ActivationTypeTimestamp
}

func ValidateMigrationFeeOption(migrationFeeOption MigrationFeeOption, migrationOption *MigrationOption) bool {
	if migrationFeeOption == MigrationFeeOptionCustomizable {
		if migrationOption == nil {
			return false
		}
		return *migrationOption == MigrationOptionMetDammV2
	}
	switch migrationFeeOption {
	case MigrationFeeOptionFixedBps25, MigrationFeeOptionFixedBps30, MigrationFeeOptionFixedBps100, MigrationFeeOptionFixedBps200, MigrationFeeOptionFixedBps400, MigrationFeeOptionFixedBps600:
		return true
	default:
		return false
	}
}

func ValidateTokenDecimals(tokenDecimal TokenDecimal) bool {
	return tokenDecimal >= TokenDecimalSix && tokenDecimal <= TokenDecimalNine
}

func ValidateLPPercentages(partnerLiquidityPercentage, partnerPermanentLockedLiquidityPercentage, creatorLiquidityPercentage, creatorPermanentLockedLiquidityPercentage, partnerVestingPercentage, creatorVestingPercentage uint8) bool {
	total := uint16(partnerLiquidityPercentage) + uint16(partnerPermanentLockedLiquidityPercentage) + uint16(creatorLiquidityPercentage) + uint16(creatorPermanentLockedLiquidityPercentage) + uint16(partnerVestingPercentage) + uint16(creatorVestingPercentage)
	return total == 100
}

func ValidateCurve(curve []LiquidityDistributionParameters, sqrtStartPrice *big.Int) bool {
	if len(curve) == 0 || len(curve) > MaxCurvePoint {
		return false
	}
	first := curve[0]
	if first.SqrtPrice.BigInt().Cmp(sqrtStartPrice) <= 0 || first.Liquidity.BigInt().Sign() <= 0 || first.SqrtPrice.BigInt().Cmp(MaxSqrtPrice) > 0 {
		return false
	}
	for i := 1; i < len(curve); i++ {
		cur := curve[i]
		prev := curve[i-1]
		if cur.SqrtPrice.BigInt().Cmp(prev.SqrtPrice.BigInt()) <= 0 || cur.Liquidity.BigInt().Sign() <= 0 {
			return false
		}
	}
	return curve[len(curve)-1].SqrtPrice.BigInt().Cmp(MaxSqrtPrice) <= 0
}

func ValidateTokenSupply(tokenSupply *TokenSupplyParams, leftoverReceiver solanago.PublicKey, swapBaseAmount, migrationBaseAmount *big.Int, lockedVesting LockedVestingParameters, swapBaseAmountBuffer *big.Int) bool {
	if tokenSupply == nil {
		return true
	}
	if leftoverReceiver.IsZero() {
		return false
	}
	minWithBuffer, err := GetTotalTokenSupply(swapBaseAmountBuffer, migrationBaseAmount, struct {
		AmountPerPeriod   *big.Int
		NumberOfPeriod    *big.Int
		CliffUnlockAmount *big.Int
	}{
		AmountPerPeriod:   new(big.Int).SetUint64(lockedVesting.AmountPerPeriod),
		NumberOfPeriod:    new(big.Int).SetUint64(uint64(lockedVesting.NumberOfPeriod)),
		CliffUnlockAmount: new(big.Int).SetUint64(lockedVesting.CliffUnlockAmount),
	})
	if err != nil {
		return false
	}
	minWithoutBuffer, err := GetTotalTokenSupply(swapBaseAmount, migrationBaseAmount, struct {
		AmountPerPeriod   *big.Int
		NumberOfPeriod    *big.Int
		CliffUnlockAmount *big.Int
	}{
		AmountPerPeriod:   new(big.Int).SetUint64(lockedVesting.AmountPerPeriod),
		NumberOfPeriod:    new(big.Int).SetUint64(uint64(lockedVesting.NumberOfPeriod)),
		CliffUnlockAmount: new(big.Int).SetUint64(lockedVesting.CliffUnlockAmount),
	})
	if err != nil {
		return false
	}
	pre := new(big.Int).SetUint64(tokenSupply.PreMigrationTokenSupply)
	post := new(big.Int).SetUint64(tokenSupply.PostMigrationTokenSupply)
	if minWithoutBuffer.Cmp(post) > 0 || post.Cmp(pre) > 0 || minWithBuffer.Cmp(pre) > 0 {
		return false
	}
	return true
}

func ValidateTokenUpdateAuthorityOptions(option TokenUpdateAuthorityOption) bool {
	switch option {
	case TokenUpdateAuthorityCreatorUpdateAuthority, TokenUpdateAuthorityImmutable, TokenUpdateAuthorityPartnerUpdateAuthority, TokenUpdateAuthorityCreatorUpdateAndMintAuthority, TokenUpdateAuthorityPartnerUpdateAndMintAuthority:
		return true
	default:
		return false
	}
}

func ValidatePoolCreationFee(poolCreationFee uint64) bool {
	if poolCreationFee == 0 {
		return true
	}
	return poolCreationFee >= MinPoolCreationFee && poolCreationFee <= MaxPoolCreationFee
}

func ValidateLiquidityVestingInfo(vestingInfo LiquidityVestingInfoParameters) bool {
	isZero := vestingInfo.VestingPercentage == 0 &&
		vestingInfo.BpsPerPeriod == 0 &&
		vestingInfo.NumberOfPeriods == 0 &&
		vestingInfo.CliffDurationFromMigrationTime == 0 &&
		vestingInfo.Frequency == 0
	if isZero {
		return true
	}
	if vestingInfo.VestingPercentage > 100 {
		return false
	}
	if vestingInfo.VestingPercentage > 0 && vestingInfo.Frequency == 0 {
		return false
	}
	return true
}

func ValidateMinimumLockedLiquidity(partnerPermanentLockedLiquidityPercentage, creatorPermanentLockedLiquidityPercentage uint8, partnerLiquidityVestingInfo, creatorLiquidityVestingInfo *LiquidityVestingInfoParameters) bool {
	lockedBpsAtDay1 := CalculateLockedLiquidityBpsAtTime(partnerPermanentLockedLiquidityPercentage, creatorPermanentLockedLiquidityPercentage, partnerLiquidityVestingInfo, creatorLiquidityVestingInfo, SecondsPerDay)
	return lockedBpsAtDay1 >= MinLockedLiquidityBps
}

func ValidateMigratedPoolFee(migratedPoolFee MigratedPoolFee, migrationOption *MigrationOption, migrationFeeOption *MigrationFeeOption) bool {
	isEmpty := func() bool {
		return migratedPoolFee.CollectFeeMode == 0 && migratedPoolFee.DynamicFee == 0 && migratedPoolFee.PoolFeeBps == 0
	}
	if migrationOption != nil && migrationFeeOption != nil {
		if *migrationOption == MigrationOptionMetDamm {
			return isEmpty()
		}
		if *migrationOption == MigrationOptionMetDammV2 && *migrationFeeOption != MigrationFeeOptionCustomizable {
			return isEmpty()
		}
	}
	if isEmpty() {
		return true
	}
	if migratedPoolFee.PoolFeeBps < uint16(MinMigratedPoolFeeBps) || migratedPoolFee.PoolFeeBps > uint16(MaxMigratedPoolFeeBps) {
		return false
	}
	if !ValidateCollectFeeMode(CollectFeeMode(migratedPoolFee.CollectFeeMode)) {
		return false
	}
	if migratedPoolFee.DynamicFee != uint8(DammV2DynamicFeeModeDisabled) && migratedPoolFee.DynamicFee != uint8(DammV2DynamicFeeModeEnabled) {
		return false
	}
	return true
}

func ValidateMigratedPoolBaseFeeMode(migratedPoolBaseFeeMode DammV2BaseFeeMode, migratedPoolMarketCapFeeSchedulerParams MigratedPoolMarketCapFeeSchedulerParameters, migrationOption *MigrationOption) error {
	if migrationOption != nil && *migrationOption != MigrationOptionMetDammV2 {
		return nil
	}
	if migratedPoolBaseFeeMode == DammV2BaseFeeModeRateLimiter {
		return errors.New("RateLimiter (mode 2) is not supported for DAMM V2 migration")
	}
	isFixedFeeParams := migratedPoolMarketCapFeeSchedulerParams.NumberOfPeriod == 0 &&
		migratedPoolMarketCapFeeSchedulerParams.SqrtPriceStepBps == 0 &&
		migratedPoolMarketCapFeeSchedulerParams.SchedulerExpirationDuration == 0 &&
		migratedPoolMarketCapFeeSchedulerParams.ReductionFactor == 0
	if migratedPoolBaseFeeMode == DammV2BaseFeeModeFeeTimeSchedulerLinear || migratedPoolBaseFeeMode == DammV2BaseFeeModeFeeTimeSchedulerExponential {
		if !isFixedFeeParams {
			return errors.New("FeeTimeScheduler modes only work as fixed fee for migrated pools")
		}
		return nil
	}
	if migratedPoolBaseFeeMode == DammV2BaseFeeModeFeeMarketCapSchedulerLinear || migratedPoolBaseFeeMode == DammV2BaseFeeModeFeeMarketCapSchedulerExp {
		if isFixedFeeParams {
			return nil
		}
		if migratedPoolMarketCapFeeSchedulerParams.NumberOfPeriod <= 0 || migratedPoolMarketCapFeeSchedulerParams.SqrtPriceStepBps <= 0 || migratedPoolMarketCapFeeSchedulerParams.SchedulerExpirationDuration <= 0 {
			return errors.New("For market cap schedulers, numberOfPeriod, sqrtPriceStepBps, and schedulerExpirationDuration must be > 0")
		}
		return nil
	}
	return errors.New("Unknown migratedPoolBaseFeeMode")
}

func ValidateMigrationFee(migrationFee MigrationFee) error {
	if migrationFee.FeePercentage > MaxMigrationFeePercentage {
		return errors.New("Migration fee percentage out of range")
	}
	if migrationFee.CreatorFeePercentage > MaxCreatorMigrationFeePercentage {
		return errors.New("Migration creator fee percentage out of range")
	}
	return nil
}

func ValidateConfigParameters(configParam CreateConfigParams) error {
	if !ValidatePoolFees(configParam.PoolFees, CollectFeeMode(configParam.CollectFeeMode), ActivationType(configParam.ActivationType)) {
		return errors.New("Invalid pool fees")
	}
	if !ValidateCollectFeeMode(CollectFeeMode(configParam.CollectFeeMode)) {
		return errors.New("Invalid collect fee mode")
	}
	if !ValidateTokenUpdateAuthorityOptions(TokenUpdateAuthorityOption(configParam.TokenUpdateAuthority)) {
		return errors.New("Invalid option for token update authority")
	}
	if !ValidateMigrationAndTokenType(MigrationOption(configParam.MigrationOption), TokenType(configParam.TokenType)) {
		return errors.New("Token type must be SPL for MeteoraDamm migration")
	}
	if !ValidateActivationType(ActivationType(configParam.ActivationType)) {
		return errors.New("Invalid activation type")
	}
	migrationOption := MigrationOption(configParam.MigrationOption)
	migrationFeeOption := MigrationFeeOption(configParam.MigrationFeeOption)
	if !ValidateMigrationFeeOption(migrationFeeOption, &migrationOption) {
		return errors.New("Invalid migration fee option")
	}
	if err := ValidateMigrationFee(configParam.MigrationFee); err != nil {
		return err
	}
	if configParam.CreatorTradingFeePercentage > 100 {
		return errors.New("Creator trading fee percentage must be between 0 and 100")
	}
	if !ValidateTokenDecimals(TokenDecimal(configParam.TokenDecimal)) {
		return errors.New("Token decimal must be between 6 and 9")
	}
	partnerVestingPercentage := configParam.PartnerLiquidityVestingInfo.VestingPercentage
	creatorVestingPercentage := configParam.CreatorLiquidityVestingInfo.VestingPercentage
	if !ValidateLPPercentages(configParam.PartnerLiquidityPercentage, configParam.PartnerPermanentLockedLiquidityPercentage, configParam.CreatorLiquidityPercentage, configParam.CreatorPermanentLockedLiquidityPercentage, partnerVestingPercentage, creatorVestingPercentage) {
		return errors.New("Sum of LP percentages must equal 100")
	}
	if !ValidatePoolCreationFee(configParam.PoolCreationFee) {
		return errors.New("Invalid pool creation fee")
	}
	if migrationOption == MigrationOptionMetDamm {
		if !isZeroVesting(configParam.PartnerLiquidityVestingInfo) || !isZeroVesting(configParam.CreatorLiquidityVestingInfo) {
			return errors.New("Liquidity vesting is not supported for MeteoraDamm migration")
		}
	} else if migrationOption == MigrationOptionMetDammV2 {
		if !ValidateLiquidityVestingInfo(configParam.PartnerLiquidityVestingInfo) {
			return errors.New("Invalid partner liquidity vesting info")
		}
		if !ValidateLiquidityVestingInfo(configParam.CreatorLiquidityVestingInfo) {
			return errors.New("Invalid creator liquidity vesting info")
		}
	}
	sqrtMigrationPrice, err := GetMigrationThresholdPrice(new(big.Int).SetUint64(configParam.MigrationQuoteThreshold), configParam.SqrtStartPrice.BigInt(), configParam.Curve)
	if err != nil {
		return err
	}
	if sqrtMigrationPrice.Cmp(MaxSqrtPrice) >= 0 {
		return errors.New("Migration sqrt price exceeds maximum")
	}
	if !ValidateMinimumLockedLiquidity(configParam.PartnerPermanentLockedLiquidityPercentage, configParam.CreatorPermanentLockedLiquidityPercentage, &configParam.PartnerLiquidityVestingInfo, &configParam.CreatorLiquidityVestingInfo) {
		locked := CalculateLockedLiquidityBpsAtTime(configParam.PartnerPermanentLockedLiquidityPercentage, configParam.CreatorPermanentLockedLiquidityPercentage, &configParam.PartnerLiquidityVestingInfo, &configParam.CreatorLiquidityVestingInfo, SecondsPerDay)
		return errors.New("Invalid migration locked liquidity: " + decimal.NewFromInt(int64(locked)).String())
	}
	if configParam.MigrationQuoteThreshold == 0 {
		return errors.New("Migration quote threshold must be greater than 0")
	}
	if configParam.SqrtStartPrice.BigInt().Cmp(MinSqrtPrice) < 0 || configParam.SqrtStartPrice.BigInt().Cmp(MaxSqrtPrice) >= 0 {
		return errors.New("Invalid sqrt start price")
	}
	if !ValidateMigratedPoolFee(configParam.MigratedPoolFee, &migrationOption, &migrationFeeOption) {
		return errors.New("Invalid migrated pool fee parameters")
	}
	if migrationOption == MigrationOptionMetDammV2 {
		if err := ValidateMigratedPoolBaseFeeMode(DammV2BaseFeeMode(configParam.MigratedPoolBaseFeeMode), configParam.MigratedPoolMarketCapFeeSchedulerParams, &migrationOption); err != nil {
			return err
		}
	}
	if !ValidateCurve(configParam.Curve, configParam.SqrtStartPrice.BigInt()) {
		return errors.New("Invalid curve")
	}
	if !IsDefaultLockedVesting(struct {
		AmountPerPeriod                *big.Int
		CliffDurationFromMigrationTime *big.Int
		Frequency                      *big.Int
		NumberOfPeriod                 *big.Int
		CliffUnlockAmount              *big.Int
	}{
		AmountPerPeriod:                new(big.Int).SetUint64(configParam.LockedVesting.AmountPerPeriod),
		CliffDurationFromMigrationTime: new(big.Int).SetUint64(configParam.LockedVesting.CliffDurationFromMigrationTime),
		Frequency:                      new(big.Int).SetUint64(configParam.LockedVesting.Frequency),
		NumberOfPeriod:                 new(big.Int).SetUint64(uint64(configParam.LockedVesting.NumberOfPeriod)),
		CliffUnlockAmount:              new(big.Int).SetUint64(configParam.LockedVesting.CliffUnlockAmount),
	}) {
		total := new(big.Int).Add(new(big.Int).SetUint64(configParam.LockedVesting.CliffUnlockAmount), new(big.Int).Mul(new(big.Int).SetUint64(configParam.LockedVesting.AmountPerPeriod), big.NewInt(int64(configParam.LockedVesting.NumberOfPeriod))))
		if configParam.LockedVesting.Frequency == 0 || total.Sign() == 0 {
			return errors.New("Invalid vesting parameters")
		}
	}
	if configParam.TokenSupply != nil {
		sqrtMigrationPrice, err := GetMigrationThresholdPrice(new(big.Int).SetUint64(configParam.MigrationQuoteThreshold), configParam.SqrtStartPrice.BigInt(), configParam.Curve)
		if err != nil {
			return err
		}
		swapBaseAmount, err := GetBaseTokenForSwap(configParam.SqrtStartPrice.BigInt(), sqrtMigrationPrice, configParam.Curve)
		if err != nil {
			return err
		}
		migrationQuoteAmount := GetMigrationQuoteAmountFromMigrationQuoteThreshold(decimal.NewFromInt(int64(configParam.MigrationQuoteThreshold)), configParam.MigrationFee.FeePercentage)
		migrationBaseAmount, err := GetMigrationBaseToken(migrationQuoteAmount.BigInt(), sqrtMigrationPrice, migrationOption)
		if err != nil {
			return err
		}
		swapBaseAmountBuffer, err := GetSwapAmountWithBuffer(swapBaseAmount, configParam.SqrtStartPrice.BigInt(), configParam.Curve)
		if err != nil {
			return err
		}
		if !ValidateTokenSupply(configParam.TokenSupply, configParam.LeftoverReceiver, swapBaseAmount, migrationBaseAmount, configParam.LockedVesting, swapBaseAmountBuffer) {
			return errors.New("Invalid token supply")
		}
	}
	return nil
}

func isZeroVesting(v LiquidityVestingInfoParameters) bool {
	return v.VestingPercentage == 0 && v.BpsPerPeriod == 0 && v.NumberOfPeriods == 0 && v.CliffDurationFromMigrationTime == 0 && v.Frequency == 0
}

func ValidateBaseTokenType(baseTokenType TokenType, poolConfig PoolConfig) bool {
	return baseTokenType == TokenType(poolConfig.TokenType)
}

func ValidateBalance(ctx context.Context, client *rpc.Client, owner, inputMint, inputTokenAccount solanago.PublicKey, amountIn *big.Int) error {
	if IsNativeSol(inputMint) {
		bal, err := client.GetBalance(ctx, owner, rpc.CommitmentConfirmed)
		if err != nil {
			return err
		}
		required := new(big.Int).Add(amountIn, big.NewInt(10_000_000))
		if new(big.Int).SetUint64(bal.Value).Cmp(required) < 0 {
			return errors.New("Insufficient SOL balance")
		}
		return nil
	}
	res, err := client.GetTokenAccountBalance(ctx, inputTokenAccount, rpc.CommitmentConfirmed)
	if err != nil {
		return err
	}
	bal := new(big.Int)
	bal.SetString(res.Value.Amount, 10)
	if bal.Cmp(amountIn) < 0 {
		return errors.New("Insufficient token balance")
	}
	return nil
}

func ValidateSwapAmount(amountIn *big.Int) error {
	if amountIn.Sign() <= 0 {
		return errors.New("Swap amount must be greater than 0")
	}
	return nil
}
