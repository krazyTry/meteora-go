package helpers

import (
	"context"
	"errors"
	"math"
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
	"github.com/shopspring/decimal"
)

func ValidateFeeScheduler(numberOfPeriod uint16, periodFrequency, reductionFactor, cliffFeeNumerator *big.Int, baseFeeMode shared.BaseFeeMode) bool {
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
	if minFeeNumerator.Cmp(big.NewInt(shared.MinFeeNumerator)) < 0 || maxFeeNumerator.Cmp(big.NewInt(shared.MaxFeeNumerator)) > 0 {
		return false
	}
	return true
}

func ValidateFeeRateLimiter(cliffFeeNumerator, feeIncrementBps, maxLimiterDuration, referenceAmount *big.Int, collectFeeMode shared.CollectFeeMode, activationType shared.ActivationType) bool {
	if collectFeeMode != shared.CollectFeeModeQuoteToken {
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
	maxLimiterDurationLimit := big.NewInt(shared.MaxRateLimiterDurationInSeconds)
	if activationType == shared.ActivationTypeSlot {
		maxLimiterDurationLimit = big.NewInt(shared.MaxRateLimiterDurationInSlots)
	}
	if maxLimiterDuration.Cmp(maxLimiterDurationLimit) > 0 {
		return false
	}
	feeIncrementNumerator, err := ToNumerator(feeIncrementBps, big.NewInt(shared.FeeDenominator))
	if err != nil {
		return false
	}
	if feeIncrementNumerator.Cmp(big.NewInt(shared.FeeDenominator)) >= 0 {
		return false
	}
	if cliffFeeNumerator.Cmp(big.NewInt(shared.MinFeeNumerator)) < 0 || cliffFeeNumerator.Cmp(big.NewInt(shared.MaxFeeNumerator)) > 0 {
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
	return minFeeNumerator.Cmp(big.NewInt(shared.MinFeeNumerator)) >= 0 && maxFeeNumerator.Cmp(big.NewInt(shared.MaxFeeNumerator)) <= 0
}

func ValidatePoolFees(poolFees shared.PoolFeeParameters, collectFeeMode shared.CollectFeeMode, activationType shared.ActivationType) bool {
	if poolFees.BaseFee.CliffFeeNumerator == 0 {
		return false
	}
	if poolFees.BaseFee.CliffFeeNumerator < uint64(shared.MinFeeNumerator) {
		return false
	}
	if poolFees.BaseFee.BaseFeeMode == uint8(shared.BaseFeeModeFeeSchedulerLinear) || poolFees.BaseFee.BaseFeeMode == uint8(shared.BaseFeeModeFeeSchedulerExponential) {
		if !ValidateFeeScheduler(poolFees.BaseFee.FirstFactor, new(big.Int).SetUint64(poolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(poolFees.BaseFee.ThirdFactor), new(big.Int).SetUint64(poolFees.BaseFee.CliffFeeNumerator), shared.BaseFeeMode(poolFees.BaseFee.BaseFeeMode)) {
			return false
		}
	}
	if poolFees.BaseFee.BaseFeeMode == uint8(shared.BaseFeeModeRateLimiter) {
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

func ValidateDynamicFee(dynamicFee *shared.DynamicFeeParameters) bool {
	if dynamicFee == nil {
		return true
	}
	if dynamicFee.BinStep != uint16(shared.BinStepBpsDefault) {
		return false
	}
	if dynamicFee.BinStepU128.BigInt().Cmp(shared.BinStepBpsU128Default) != 0 {
		return false
	}
	if dynamicFee.FilterPeriod >= dynamicFee.DecayPeriod {
		return false
	}
	if dynamicFee.ReductionFactor > uint16(shared.MaxBasisPoint) {
		return false
	}
	if dynamicFee.VariableFeeControl > uint32(shared.U24Max) {
		return false
	}
	if dynamicFee.MaxVolatilityAccumulator > uint32(shared.U24Max) {
		return false
	}
	return true
}

func ValidateCollectFeeMode(collectFeeMode shared.CollectFeeMode) bool {
	return collectFeeMode == shared.CollectFeeModeQuoteToken || collectFeeMode == shared.CollectFeeModeOutputToken
}

func ValidateMigrationAndTokenType(migrationOption shared.MigrationOption, tokenType shared.TokenType) bool {
	if migrationOption == shared.MigrationOptionMetDamm {
		return tokenType == shared.TokenTypeSPL
	}
	return true
}

func ValidateActivationType(activationType shared.ActivationType) bool {
	return activationType == shared.ActivationTypeSlot || activationType == shared.ActivationTypeTimestamp
}

func ValidateMigrationFeeOption(migrationFeeOption shared.MigrationFeeOption, migrationOption *shared.MigrationOption) bool {
	if migrationFeeOption == shared.MigrationFeeOptionCustomizable {
		if migrationOption == nil {
			return false
		}
		return *migrationOption == shared.MigrationOptionMetDammV2
	}
	switch migrationFeeOption {
	case shared.MigrationFeeOptionFixedBps25, shared.MigrationFeeOptionFixedBps30,
		shared.MigrationFeeOptionFixedBps100, shared.MigrationFeeOptionFixedBps200,
		shared.MigrationFeeOptionFixedBps400, shared.MigrationFeeOptionFixedBps600:
		return true
	default:
		return false
	}
}

func ValidateTokenDecimals(tokenDecimal shared.TokenDecimal) bool {
	return tokenDecimal >= shared.TokenDecimalSix && tokenDecimal <= shared.TokenDecimalNine
}

func ValidateLPPercentages(partnerLiquidityPercentage, partnerPermanentLockedLiquidityPercentage, creatorLiquidityPercentage, creatorPermanentLockedLiquidityPercentage, partnerVestingPercentage, creatorVestingPercentage uint8) bool {
	total := uint16(partnerLiquidityPercentage) + uint16(partnerPermanentLockedLiquidityPercentage) + uint16(creatorLiquidityPercentage) + uint16(creatorPermanentLockedLiquidityPercentage) + uint16(partnerVestingPercentage) + uint16(creatorVestingPercentage)
	return total == 100
}

func ValidateCurve(curve []shared.LiquidityDistributionParameters, sqrtStartPrice *big.Int) bool {
	if len(curve) == 0 || len(curve) > shared.MaxCurvePoint {
		return false
	}
	first := curve[0]
	if first.SqrtPrice.BigInt().Cmp(sqrtStartPrice) <= 0 || first.Liquidity.BigInt().Sign() <= 0 || first.SqrtPrice.BigInt().Cmp(shared.MaxSqrtPrice) > 0 {
		return false
	}
	for i := 1; i < len(curve); i++ {
		cur := curve[i]
		prev := curve[i-1]
		if cur.SqrtPrice.BigInt().Cmp(prev.SqrtPrice.BigInt()) <= 0 || cur.Liquidity.BigInt().Sign() <= 0 {
			return false
		}
	}
	return curve[len(curve)-1].SqrtPrice.BigInt().Cmp(shared.MaxSqrtPrice) <= 0
}

func ValidateTokenSupply(tokenSupply *shared.TokenSupplyParams, leftoverReceiver solanago.PublicKey, swapBaseAmount, migrationBaseAmount *big.Int, lockedVesting shared.LockedVestingParameters, swapBaseAmountBuffer *big.Int) bool {
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

func ValidateTokenUpdateAuthorityOptions(option shared.TokenUpdateAuthorityOption) bool {
	switch option {
	case shared.TokenUpdateAuthorityCreatorUpdateAuthority, shared.TokenUpdateAuthorityImmutable,
		shared.TokenUpdateAuthorityPartnerUpdateAuthority, shared.TokenUpdateAuthorityCreatorUpdateAndMintAuthority,
		shared.TokenUpdateAuthorityPartnerUpdateAndMintAuthority:
		return true
	default:
		return false
	}
}

func ValidatePoolCreationFee(poolCreationFee uint64) bool {
	if poolCreationFee == 0 {
		return true
	}
	return poolCreationFee >= shared.MinPoolCreationFee && poolCreationFee <= shared.MaxPoolCreationFee
}

func ValidateLiquidityVestingInfo(vestingInfo shared.LiquidityVestingInfoParameters) bool {
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

func ValidateMinimumLockedLiquidity(partnerPermanentLockedLiquidityPercentage, creatorPermanentLockedLiquidityPercentage uint8, partnerLiquidityVestingInfo, creatorLiquidityVestingInfo *shared.LiquidityVestingInfoParameters) bool {
	lockedBpsAtDay1 := CalculateLockedLiquidityBpsAtTime(partnerPermanentLockedLiquidityPercentage, creatorPermanentLockedLiquidityPercentage, partnerLiquidityVestingInfo, creatorLiquidityVestingInfo, shared.SecondsPerDay)
	return lockedBpsAtDay1 >= shared.MinLockedLiquidityBps
}

func ValidateMigratedPoolFee(migratedPoolFee shared.MigratedPoolFee, migrationOption *shared.MigrationOption, migrationFeeOption *shared.MigrationFeeOption) bool {
	isEmpty := func() bool {
		return migratedPoolFee.CollectFeeMode == 0 && migratedPoolFee.DynamicFee == 0 && migratedPoolFee.PoolFeeBps == 0
	}
	if migrationOption != nil && migrationFeeOption != nil {
		if *migrationOption == shared.MigrationOptionMetDamm {
			return isEmpty()
		}
		if *migrationOption == shared.MigrationOptionMetDammV2 && *migrationFeeOption != shared.MigrationFeeOptionCustomizable {
			return isEmpty()
		}
	}
	if isEmpty() {
		return true
	}
	if migratedPoolFee.PoolFeeBps < uint16(shared.MinMigratedPoolFeeBps) || migratedPoolFee.PoolFeeBps > uint16(shared.MaxMigratedPoolFeeBps) {
		return false
	}
	if !ValidateCollectFeeMode(shared.CollectFeeMode(migratedPoolFee.CollectFeeMode)) {
		return false
	}
	if migratedPoolFee.DynamicFee != uint8(shared.DammV2DynamicFeeModeDisabled) && migratedPoolFee.DynamicFee != uint8(shared.DammV2DynamicFeeModeEnabled) {
		return false
	}
	return true
}

func ValidateMigratedPoolBaseFeeMode(migratedPoolBaseFeeMode shared.DammV2BaseFeeMode, migratedPoolMarketCapFeeSchedulerParams shared.MigratedPoolMarketCapFeeSchedulerParameters, migrationOption *shared.MigrationOption) error {
	if migrationOption != nil && *migrationOption != shared.MigrationOptionMetDammV2 {
		return nil
	}
	if migratedPoolBaseFeeMode == shared.DammV2BaseFeeModeRateLimiter {
		return errors.New("RateLimiter (mode 2) is not supported for DAMM V2 migration")
	}
	isFixedFeeParams := migratedPoolMarketCapFeeSchedulerParams.NumberOfPeriod == 0 &&
		migratedPoolMarketCapFeeSchedulerParams.SqrtPriceStepBps == 0 &&
		migratedPoolMarketCapFeeSchedulerParams.SchedulerExpirationDuration == 0 &&
		migratedPoolMarketCapFeeSchedulerParams.ReductionFactor == 0
	if migratedPoolBaseFeeMode == shared.DammV2BaseFeeModeFeeTimeSchedulerLinear || migratedPoolBaseFeeMode == shared.DammV2BaseFeeModeFeeTimeSchedulerExponential {
		if !isFixedFeeParams {
			return errors.New("FeeTimeScheduler modes only work as fixed fee for migrated pools")
		}
		return nil
	}
	if migratedPoolBaseFeeMode == shared.DammV2BaseFeeModeFeeMarketCapSchedulerLinear || migratedPoolBaseFeeMode == shared.DammV2BaseFeeModeFeeMarketCapSchedulerExp {
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

func ValidateMigrationFee(migrationFee shared.MigrationFee) error {
	if migrationFee.FeePercentage > shared.MaxMigrationFeePercentage {
		return errors.New("Migration fee percentage out of range")
	}
	if migrationFee.CreatorFeePercentage > shared.MaxCreatorMigrationFeePercentage {
		return errors.New("Migration creator fee percentage out of range")
	}
	return nil
}

func ValidateConfigParameters(configParam shared.CreateConfigParams) error {
	if !ValidatePoolFees(configParam.PoolFees, shared.CollectFeeMode(configParam.CollectFeeMode), shared.ActivationType(configParam.ActivationType)) {
		return errors.New("Invalid pool fees")
	}
	if !ValidateCollectFeeMode(shared.CollectFeeMode(configParam.CollectFeeMode)) {
		return errors.New("Invalid collect fee mode")
	}
	if !ValidateTokenUpdateAuthorityOptions(shared.TokenUpdateAuthorityOption(configParam.TokenUpdateAuthority)) {
		return errors.New("Invalid option for token update authority")
	}
	if !ValidateMigrationAndTokenType(shared.MigrationOption(configParam.MigrationOption), shared.TokenType(configParam.TokenType)) {
		return errors.New("Token type must be SPL for MeteoraDamm migration")
	}
	if !ValidateActivationType(shared.ActivationType(configParam.ActivationType)) {
		return errors.New("Invalid activation type")
	}
	migrationOption := shared.MigrationOption(configParam.MigrationOption)
	migrationFeeOption := shared.MigrationFeeOption(configParam.MigrationFeeOption)
	if !ValidateMigrationFeeOption(migrationFeeOption, &migrationOption) {
		return errors.New("Invalid migration fee option")
	}
	if err := ValidateMigrationFee(configParam.MigrationFee); err != nil {
		return err
	}
	if configParam.CreatorTradingFeePercentage > 100 {
		return errors.New("Creator trading fee percentage must be between 0 and 100")
	}
	if !ValidateTokenDecimals(shared.TokenDecimal(configParam.TokenDecimal)) {
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
	if migrationOption == shared.MigrationOptionMetDamm {
		if !isZeroVesting(configParam.PartnerLiquidityVestingInfo) || !isZeroVesting(configParam.CreatorLiquidityVestingInfo) {
			return errors.New("Liquidity vesting is not supported for MeteoraDamm migration")
		}
	} else if migrationOption == shared.MigrationOptionMetDammV2 {
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
	if sqrtMigrationPrice.Cmp(shared.MaxSqrtPrice) >= 0 {
		return errors.New("Migration sqrt price exceeds maximum")
	}
	if !ValidateMinimumLockedLiquidity(configParam.PartnerPermanentLockedLiquidityPercentage, configParam.CreatorPermanentLockedLiquidityPercentage, &configParam.PartnerLiquidityVestingInfo, &configParam.CreatorLiquidityVestingInfo) {
		locked := CalculateLockedLiquidityBpsAtTime(configParam.PartnerPermanentLockedLiquidityPercentage, configParam.CreatorPermanentLockedLiquidityPercentage, &configParam.PartnerLiquidityVestingInfo, &configParam.CreatorLiquidityVestingInfo, shared.SecondsPerDay)
		return errors.New("Invalid migration locked liquidity: " + decimal.NewFromInt(int64(locked)).String())
	}
	if configParam.MigrationQuoteThreshold == 0 {
		return errors.New("Migration quote threshold must be greater than 0")
	}
	if configParam.SqrtStartPrice.BigInt().Cmp(shared.MinSqrtPrice) < 0 || configParam.SqrtStartPrice.BigInt().Cmp(shared.MaxSqrtPrice) >= 0 {
		return errors.New("Invalid sqrt start price")
	}
	if !ValidateMigratedPoolFee(configParam.MigratedPoolFee, &migrationOption, &migrationFeeOption) {
		return errors.New("Invalid migrated pool fee parameters")
	}
	if migrationOption == shared.MigrationOptionMetDammV2 {
		if err := ValidateMigratedPoolBaseFeeMode(shared.DammV2BaseFeeMode(configParam.MigratedPoolBaseFeeMode), configParam.MigratedPoolMarketCapFeeSchedulerParams, &migrationOption); err != nil {
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

func isZeroVesting(v shared.LiquidityVestingInfoParameters) bool {
	return v.VestingPercentage == 0 && v.BpsPerPeriod == 0 && v.NumberOfPeriods == 0 && v.CliffDurationFromMigrationTime == 0 && v.Frequency == 0
}

func ValidateBaseTokenType(baseTokenType shared.TokenType, poolConfig shared.PoolConfig) bool {
	return baseTokenType == shared.TokenType(poolConfig.TokenType)
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
