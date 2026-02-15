package helpers

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"

	mathutil "github.com/krazyTry/meteora-go/dynamic_bonding_curve/math"
	"github.com/shopspring/decimal"
)

func BuildCurve(params BuildCurveParams) (ConfigParameters, error) {
	percentage := decimalFromFloat(params.PercentageSupplyOnMigration)
	migrationQuoteThreshold := decimalFromFloat(params.MigrationQuoteThreshold)
	return buildCurveInternal(params.BuildCurveBaseParams, percentage, migrationQuoteThreshold)
}

func BuildCurveWithMarketCap(params BuildCurveWithMarketCapParams) (ConfigParameters, error) {
	lockedVesting, err := GetLockedVestingParams(
		params.LockedVestingParams.TotalLockedVestingAmount,
		params.LockedVestingParams.NumberOfVestingPeriod,
		params.LockedVestingParams.CliffUnlockAmount,
		params.LockedVestingParams.TotalVestingDuration,
		params.LockedVestingParams.CliffDurationFromMigrationTime,
		params.TokenBaseDecimal,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	totalLeftover, err := lamportsFromUint64(params.Leftover, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	totalSupply, err := lamportsFromUint64(params.TotalTokenSupply, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	initialMarketCap := decimalFromFloat(params.InitialMarketCap)
	migrationMarketCap := decimalFromFloat(params.MigrationMarketCap)

	var percentageSupplyOnMigration decimal.Decimal
	if params.MigrationFee.FeePercentage > 0 {
		percentageSupplyOnMigration, err = CalculateAdjustedPercentageSupplyOnMigration(
			initialMarketCap,
			migrationMarketCap,
			params.MigrationFee,
			lockedVesting,
			totalLeftover,
			totalSupply,
		)
		if err != nil {
			return ConfigParameters{}, err
		}
	} else {
		percentageSupplyOnMigration, err = GetPercentageSupplyOnMigration(
			initialMarketCap,
			migrationMarketCap,
			lockedVesting,
			totalLeftover,
			totalSupply,
		)
		if err != nil {
			return ConfigParameters{}, err
		}
	}

	migrationQuoteAmount := GetMigrationQuoteAmount(
		migrationMarketCap,
		percentageSupplyOnMigration,
	)
	migrationQuoteThreshold := GetMigrationQuoteThresholdFromMigrationQuoteAmount(
		migrationQuoteAmount,
		decimalFromUint64(uint64(params.MigrationFee.FeePercentage)),
	)

	return buildCurveInternal(
		params.BuildCurveBaseParams,
		percentageSupplyOnMigration,
		migrationQuoteThreshold,
	)
}

func BuildCurveWithTwoSegments(params BuildCurveWithTwoSegmentsParams) (ConfigParameters, error) {
	baseFee, err := GetBaseFeeParams(params.BaseFeeParams, params.TokenQuoteDecimal, params.ActivationType)
	if err != nil {
		return ConfigParameters{}, err
	}

	lockedVesting, err := GetLockedVestingParams(
		params.LockedVestingParams.TotalLockedVestingAmount,
		params.LockedVestingParams.NumberOfVestingPeriod,
		params.LockedVestingParams.CliffUnlockAmount,
		params.LockedVestingParams.TotalVestingDuration,
		params.LockedVestingParams.CliffDurationFromMigrationTime,
		params.TokenBaseDecimal,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	partnerVestingParams := params.PartnerLiquidityVestingInfoParams
	if partnerVestingParams == nil {
		partnerVestingParams = &DefaultLiquidityVestingInfoParams
	}
	partnerLiquidityVestingInfo, err := GetLiquidityVestingInfoParams(
		partnerVestingParams.VestingPercentage,
		partnerVestingParams.BpsPerPeriod,
		partnerVestingParams.NumberOfPeriods,
		partnerVestingParams.CliffDurationFromMigrationTime,
		partnerVestingParams.TotalDuration,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	creatorVestingParams := params.CreatorLiquidityVestingInfoParams
	if creatorVestingParams == nil {
		creatorVestingParams = &DefaultLiquidityVestingInfoParams
	}
	creatorLiquidityVestingInfo, err := GetLiquidityVestingInfoParams(
		creatorVestingParams.VestingPercentage,
		creatorVestingParams.BpsPerPeriod,
		creatorVestingParams.NumberOfPeriods,
		creatorVestingParams.CliffDurationFromMigrationTime,
		creatorVestingParams.TotalDuration,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	poolCreationFeeInLamports, err := lamportsU64FromUint64(params.PoolCreationFee, TokenDecimalNine)
	if err != nil {
		return ConfigParameters{}, err
	}

	migratedPoolFeeParams := GetMigratedPoolFeeParams(
		params.MigrationOption,
		params.MigrationFeeOption,
		params.MigratedPoolFee,
	)

	percentageSupplyOnMigration := decimalFromFloat(params.PercentageSupplyOnMigration)
	migrationBaseSupply := decimalFromUint64(params.TotalTokenSupply).
		Mul(percentageSupplyOnMigration).
		Div(decimal.NewFromInt(100))

	totalSupply, err := lamportsFromUint64(params.TotalTokenSupply, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	migrationQuoteAmount := GetMigrationQuoteAmount(
		decimalFromFloat(params.MigrationMarketCap),
		percentageSupplyOnMigration,
	)
	migrationQuoteThreshold := GetMigrationQuoteThresholdFromMigrationQuoteAmount(
		migrationQuoteAmount,
		decimalFromUint64(uint64(params.MigrationFee.FeePercentage)),
	)

	migrationPrice := migrationQuoteAmount.Div(migrationBaseSupply)

	migrationQuoteThresholdInLamport, err := lamportsFromDecimal(migrationQuoteThreshold, params.TokenQuoteDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}
	migrationQuoteAmountInLamport, err := lamportsFromDecimal(migrationQuoteAmount, params.TokenQuoteDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	migrateSqrtPrice, err := GetSqrtPriceFromPrice(
		migrationPrice.String(),
		int(params.TokenBaseDecimal),
		int(params.TokenQuoteDecimal),
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	migrationBaseAmount, err := GetMigrationBaseToken(migrationQuoteAmountInLamport, migrateSqrtPrice, params.MigrationOption)
	if err != nil {
		return ConfigParameters{}, err
	}

	totalVestingAmount := GetTotalVestingAmount(lockedVesting)
	totalLeftover, err := lamportsFromUint64(params.Leftover, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	swapAmount := new(big.Int).Sub(totalSupply, migrationBaseAmount)
	swapAmount.Sub(swapAmount, totalVestingAmount)
	swapAmount.Sub(swapAmount, totalLeftover)

	initialSqrtPrice, err := GetSqrtPriceFromMarketCap(
		params.InitialMarketCap,
		params.TotalTokenSupply,
		params.TokenBaseDecimal,
		params.TokenQuoteDecimal,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	midSqrtPrice1, err := sqrtBigIntDecimalMul(migrateSqrtPrice, initialSqrtPrice)
	if err != nil {
		return ConfigParameters{}, err
	}

	midSqrtPrice2, err := fourthRootBigIntDecimalMul(
		decimal.NewFromBigInt(initialSqrtPrice, 0),
		powBigIntDecimal(migrateSqrtPrice, 3),
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	midSqrtPrice3, err := fourthRootBigIntDecimalMul(
		powBigIntDecimal(initialSqrtPrice, 3),
		decimal.NewFromBigInt(migrateSqrtPrice, 0),
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	midPrices := []*big.Int{midSqrtPrice3, midSqrtPrice2, midSqrtPrice1}
	var sqrtStartPrice *big.Int
	var curve []LiquidityDistributionParameters
	for _, mid := range midPrices {
		result, err := GetTwoCurve(migrateSqrtPrice, mid, initialSqrtPrice, swapAmount, migrationQuoteThresholdInLamport)
		if err != nil {
			return ConfigParameters{}, err
		}
		if result.IsOk {
			curve = result.Curve
			sqrtStartPrice = result.SqrtStartPrice
			break
		}
	}
	if sqrtStartPrice == nil {
		return ConfigParameters{}, errors.New("failed to derive valid two-segment curve")
	}

	totalDynamicSupply, err := GetTotalSupplyFromCurve(
		migrationQuoteThresholdInLamport,
		sqrtStartPrice,
		curve,
		lockedVesting,
		params.MigrationOption,
		totalLeftover,
		params.MigrationFee.FeePercentage,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	if totalDynamicSupply.Cmp(totalSupply) > 0 {
		leftOverDelta := new(big.Int).Sub(totalDynamicSupply, totalSupply)
		if leftOverDelta.Cmp(totalLeftover) >= 0 {
			return ConfigParameters{}, errors.New("leftOverDelta must be less than totalLeftover")
		}
	}

	migrationQuoteThresholdU64, err := BigIntToU64(migrationQuoteThresholdInLamport)
	if err != nil {
		return ConfigParameters{}, err
	}
	totalSupplyU64, err := BigIntToU64(totalSupply)
	if err != nil {
		return ConfigParameters{}, err
	}

	marketCapFeeScheduler, err := buildMigratedPoolMarketCapFeeSchedulerParams(
		params.MigratedPoolMarketCapFeeSchedulerParams,
		params.BaseFeeParams,
		params.MigratedPoolBaseFeeMode,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	cfg := ConfigParameters{
		PoolFees: PoolFeeParameters{
			BaseFee: baseFee,
		},
		ActivationType:             uint8(params.ActivationType),
		CollectFeeMode:             uint8(params.CollectFeeMode),
		MigrationOption:            uint8(params.MigrationOption),
		TokenType:                  uint8(params.TokenType),
		TokenDecimal:               uint8(params.TokenBaseDecimal),
		MigrationQuoteThreshold:    migrationQuoteThresholdU64,
		PartnerLiquidityPercentage: params.PartnerLiquidityPercentage,
		PartnerPermanentLockedLiquidityPercentage: params.PartnerPermanentLockedLiquidityPercentage,
		CreatorLiquidityPercentage:                params.CreatorLiquidityPercentage,
		CreatorPermanentLockedLiquidityPercentage: params.CreatorPermanentLockedLiquidityPercentage,
		SqrtStartPrice:     BigToU128(sqrtStartPrice),
		LockedVesting:      lockedVesting,
		MigrationFeeOption: uint8(params.MigrationFeeOption),
		TokenSupply: &TokenSupplyParams{
			PreMigrationTokenSupply:  totalSupplyU64,
			PostMigrationTokenSupply: totalSupplyU64,
		},
		CreatorTradingFeePercentage: params.CreatorTradingFeePercentage,
		MigratedPoolFee:             migratedPoolFeeParams,
		PoolCreationFee:             poolCreationFeeInLamports,
		PartnerLiquidityVestingInfo: partnerLiquidityVestingInfo,
		CreatorLiquidityVestingInfo: creatorLiquidityVestingInfo,
		MigratedPoolBaseFeeMode: uint8(derefDammV2BaseFeeMode(
			params.MigratedPoolBaseFeeMode,
			DammV2BaseFeeModeFeeTimeSchedulerLinear,
		)),
		MigratedPoolMarketCapFeeSchedulerParams: marketCapFeeScheduler,
		EnableFirstSwapWithMinFee:               params.EnableFirstSwapWithMinFee,
		Curve:                                   curve,
		TokenUpdateAuthority:                    params.TokenUpdateAuthority,
		MigrationFee: MigrationFee{
			FeePercentage:        params.MigrationFee.FeePercentage,
			CreatorFeePercentage: params.MigrationFee.CreatorFeePercentage,
		},
	}

	if params.DynamicFeeEnabled {
		dynamicFeeBps := baseFeeBpsForDynamicFee(params.BaseFeeParams)
		dynamicFee, err := GetDynamicFeeParams(dynamicFeeBps, uint16(MaxPriceChangePercentageDefault))
		if err != nil {
			return ConfigParameters{}, err
		}
		cfg.PoolFees.DynamicFee = dynamicFee
	}

	return cfg, nil
}

func BuildCurveWithMidPrice(params BuildCurveWithMidPriceParams) (ConfigParameters, error) {
	baseFee, err := GetBaseFeeParams(params.BaseFeeParams, params.TokenQuoteDecimal, params.ActivationType)
	if err != nil {
		return ConfigParameters{}, err
	}

	lockedVesting, err := GetLockedVestingParams(
		params.LockedVestingParams.TotalLockedVestingAmount,
		params.LockedVestingParams.NumberOfVestingPeriod,
		params.LockedVestingParams.CliffUnlockAmount,
		params.LockedVestingParams.TotalVestingDuration,
		params.LockedVestingParams.CliffDurationFromMigrationTime,
		params.TokenBaseDecimal,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	partnerVestingParams := params.PartnerLiquidityVestingInfoParams
	if partnerVestingParams == nil {
		partnerVestingParams = &DefaultLiquidityVestingInfoParams
	}
	partnerLiquidityVestingInfo, err := GetLiquidityVestingInfoParams(
		partnerVestingParams.VestingPercentage,
		partnerVestingParams.BpsPerPeriod,
		partnerVestingParams.NumberOfPeriods,
		partnerVestingParams.CliffDurationFromMigrationTime,
		partnerVestingParams.TotalDuration,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	creatorVestingParams := params.CreatorLiquidityVestingInfoParams
	if creatorVestingParams == nil {
		creatorVestingParams = &DefaultLiquidityVestingInfoParams
	}
	creatorLiquidityVestingInfo, err := GetLiquidityVestingInfoParams(
		creatorVestingParams.VestingPercentage,
		creatorVestingParams.BpsPerPeriod,
		creatorVestingParams.NumberOfPeriods,
		creatorVestingParams.CliffDurationFromMigrationTime,
		creatorVestingParams.TotalDuration,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	poolCreationFeeInLamports, err := lamportsU64FromUint64(params.PoolCreationFee, TokenDecimalNine)
	if err != nil {
		return ConfigParameters{}, err
	}

	migratedPoolFeeParams := GetMigratedPoolFeeParams(
		params.MigrationOption,
		params.MigrationFeeOption,
		params.MigratedPoolFee,
	)

	percentageSupplyOnMigration := decimalFromUint64(params.PercentageSupplyOnMigration)
	migrationBaseSupply := decimalFromUint64(params.TotalTokenSupply).
		Mul(percentageSupplyOnMigration).
		Div(decimal.NewFromInt(100))

	totalSupply, err := lamportsFromUint64(params.TotalTokenSupply, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	migrationQuoteAmount := GetMigrationQuoteAmount(
		decimalFromFloat(params.MigrationMarketCap),
		percentageSupplyOnMigration,
	)
	migrationQuoteThreshold := GetMigrationQuoteThresholdFromMigrationQuoteAmount(
		migrationQuoteAmount,
		decimalFromUint64(uint64(params.MigrationFee.FeePercentage)),
	)

	migrationPrice := migrationQuoteAmount.Div(migrationBaseSupply)

	migrationQuoteThresholdInLamport, err := lamportsFromDecimal(migrationQuoteThreshold, params.TokenQuoteDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}
	migrationQuoteAmountInLamport, err := lamportsFromDecimal(migrationQuoteAmount, params.TokenQuoteDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	migrateSqrtPrice, err := GetSqrtPriceFromPrice(
		migrationPrice.String(),
		int(params.TokenBaseDecimal),
		int(params.TokenQuoteDecimal),
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	migrationBaseAmount, err := GetMigrationBaseToken(migrationQuoteAmountInLamport, migrateSqrtPrice, params.MigrationOption)
	if err != nil {
		return ConfigParameters{}, err
	}

	totalVestingAmount := GetTotalVestingAmount(lockedVesting)
	totalLeftover, err := lamportsFromUint64(params.Leftover, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	swapAmount := new(big.Int).Sub(totalSupply, migrationBaseAmount)
	swapAmount.Sub(swapAmount, totalVestingAmount)
	swapAmount.Sub(swapAmount, totalLeftover)

	initialSqrtPrice, err := GetSqrtPriceFromMarketCap(
		params.InitialMarketCap,
		params.TotalTokenSupply,
		params.TokenBaseDecimal,
		params.TokenQuoteDecimal,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	midSqrtPrice, err := GetSqrtPriceFromPrice(
		decimalFromUint64(params.MidPrice).String(),
		int(params.TokenBaseDecimal),
		int(params.TokenQuoteDecimal),
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	result, err := GetTwoCurve(migrateSqrtPrice, midSqrtPrice, initialSqrtPrice, swapAmount, migrationQuoteThresholdInLamport)
	if err != nil {
		return ConfigParameters{}, err
	}
	if !result.IsOk {
		return ConfigParameters{}, errors.New("failed to derive mid-price curve")
	}

	totalDynamicSupply, err := GetTotalSupplyFromCurve(
		migrationQuoteThresholdInLamport,
		result.SqrtStartPrice,
		result.Curve,
		lockedVesting,
		params.MigrationOption,
		totalLeftover,
		params.MigrationFee.FeePercentage,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	if totalDynamicSupply.Cmp(totalSupply) > 0 {
		leftOverDelta := new(big.Int).Sub(totalDynamicSupply, totalSupply)
		if leftOverDelta.Cmp(totalLeftover) >= 0 {
			return ConfigParameters{}, errors.New("leftOverDelta must be less than totalLeftover")
		}
	}

	migrationQuoteThresholdU64, err := BigIntToU64(migrationQuoteThresholdInLamport)
	if err != nil {
		return ConfigParameters{}, err
	}
	totalSupplyU64, err := BigIntToU64(totalSupply)
	if err != nil {
		return ConfigParameters{}, err
	}

	marketCapFeeScheduler, err := buildMigratedPoolMarketCapFeeSchedulerParams(
		params.MigratedPoolMarketCapFeeSchedulerParams,
		params.BaseFeeParams,
		params.MigratedPoolBaseFeeMode,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	cfg := ConfigParameters{
		PoolFees: PoolFeeParameters{
			BaseFee: baseFee,
		},
		ActivationType:             uint8(params.ActivationType),
		CollectFeeMode:             uint8(params.CollectFeeMode),
		MigrationOption:            uint8(params.MigrationOption),
		TokenType:                  uint8(params.TokenType),
		TokenDecimal:               uint8(params.TokenBaseDecimal),
		MigrationQuoteThreshold:    migrationQuoteThresholdU64,
		PartnerLiquidityPercentage: params.PartnerLiquidityPercentage,
		PartnerPermanentLockedLiquidityPercentage: params.PartnerPermanentLockedLiquidityPercentage,
		CreatorLiquidityPercentage:                params.CreatorLiquidityPercentage,
		CreatorPermanentLockedLiquidityPercentage: params.CreatorPermanentLockedLiquidityPercentage,
		SqrtStartPrice:     BigToU128(result.SqrtStartPrice),
		LockedVesting:      lockedVesting,
		MigrationFeeOption: uint8(params.MigrationFeeOption),
		TokenSupply: &TokenSupplyParams{
			PreMigrationTokenSupply:  totalSupplyU64,
			PostMigrationTokenSupply: totalSupplyU64,
		},
		CreatorTradingFeePercentage: params.CreatorTradingFeePercentage,
		MigratedPoolFee:             migratedPoolFeeParams,
		PoolCreationFee:             poolCreationFeeInLamports,
		PartnerLiquidityVestingInfo: partnerLiquidityVestingInfo,
		CreatorLiquidityVestingInfo: creatorLiquidityVestingInfo,
		MigratedPoolBaseFeeMode: uint8(derefDammV2BaseFeeMode(
			params.MigratedPoolBaseFeeMode,
			DammV2BaseFeeModeFeeTimeSchedulerLinear,
		)),
		MigratedPoolMarketCapFeeSchedulerParams: marketCapFeeScheduler,
		EnableFirstSwapWithMinFee:               params.EnableFirstSwapWithMinFee,
		Curve:                                   result.Curve,
		TokenUpdateAuthority:                    params.TokenUpdateAuthority,
		MigrationFee: MigrationFee{
			FeePercentage:        params.MigrationFee.FeePercentage,
			CreatorFeePercentage: params.MigrationFee.CreatorFeePercentage,
		},
	}

	if params.DynamicFeeEnabled {
		dynamicFeeBps := baseFeeBpsForDynamicFee(params.BaseFeeParams)
		dynamicFee, err := GetDynamicFeeParams(dynamicFeeBps, uint16(MaxPriceChangePercentageDefault))
		if err != nil {
			return ConfigParameters{}, err
		}
		cfg.PoolFees.DynamicFee = dynamicFee
	}

	return cfg, nil
}

func BuildCurveWithLiquidityWeights(params BuildCurveWithLiquidityWeightsParams) (ConfigParameters, error) {
	baseFee, err := GetBaseFeeParams(params.BaseFeeParams, params.TokenQuoteDecimal, params.ActivationType)
	if err != nil {
		return ConfigParameters{}, err
	}

	lockedVesting, err := GetLockedVestingParams(
		params.LockedVestingParams.TotalLockedVestingAmount,
		params.LockedVestingParams.NumberOfVestingPeriod,
		params.LockedVestingParams.CliffUnlockAmount,
		params.LockedVestingParams.TotalVestingDuration,
		params.LockedVestingParams.CliffDurationFromMigrationTime,
		params.TokenBaseDecimal,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	partnerVestingParams := params.PartnerLiquidityVestingInfoParams
	if partnerVestingParams == nil {
		partnerVestingParams = &DefaultLiquidityVestingInfoParams
	}
	partnerLiquidityVestingInfo, err := GetLiquidityVestingInfoParams(
		partnerVestingParams.VestingPercentage,
		partnerVestingParams.BpsPerPeriod,
		partnerVestingParams.NumberOfPeriods,
		partnerVestingParams.CliffDurationFromMigrationTime,
		partnerVestingParams.TotalDuration,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	creatorVestingParams := params.CreatorLiquidityVestingInfoParams
	if creatorVestingParams == nil {
		creatorVestingParams = &DefaultLiquidityVestingInfoParams
	}
	creatorLiquidityVestingInfo, err := GetLiquidityVestingInfoParams(
		creatorVestingParams.VestingPercentage,
		creatorVestingParams.BpsPerPeriod,
		creatorVestingParams.NumberOfPeriods,
		creatorVestingParams.CliffDurationFromMigrationTime,
		creatorVestingParams.TotalDuration,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	poolCreationFeeInLamports, err := lamportsU64FromUint64(params.PoolCreationFee, TokenDecimalNine)
	if err != nil {
		return ConfigParameters{}, err
	}

	migratedPoolFeeParams := GetMigratedPoolFeeParams(
		params.MigrationOption,
		params.MigrationFeeOption,
		params.MigratedPoolFee,
	)

	pMin, err := GetSqrtPriceFromMarketCap(
		params.InitialMarketCap,
		params.TotalTokenSupply,
		params.TokenBaseDecimal,
		params.TokenQuoteDecimal,
	)
	if err != nil {
		return ConfigParameters{}, err
	}
	pMax, err := GetSqrtPriceFromMarketCap(
		params.MigrationMarketCap,
		params.TotalTokenSupply,
		params.TokenBaseDecimal,
		params.TokenQuoteDecimal,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	priceRatio := decimal.NewFromBigInt(pMax, 0).Div(decimal.NewFromBigInt(pMin, 0))
	qDecimal, err := decimalRootPow2(priceRatio, 4)
	if err != nil {
		return ConfigParameters{}, err
	}

	sqrtPrices := make([]*big.Int, 0, 17)
	currentPrice := new(big.Int).Set(pMin)
	for i := 0; i < 17; i++ {
		sqrtPrices = append(sqrtPrices, new(big.Int).Set(currentPrice))
		next := qDecimal.Mul(decimal.NewFromBigInt(currentPrice, 0))
		currentPrice = FromDecimalToBig(next)
	}

	totalSupply, err := lamportsFromUint64(params.TotalTokenSupply, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}
	totalLeftover, err := lamportsFromUint64(params.Leftover, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}
	totalVestingAmount := GetTotalVestingAmount(lockedVesting)
	totalSwapAndMigrationAmount := new(big.Int).Sub(totalSupply, totalVestingAmount)
	totalSwapAndMigrationAmount.Sub(totalSwapAndMigrationAmount, totalLeftover)

	sumFactor := decimal.Zero
	pmaxWeight := decimal.NewFromBigInt(pMax, 0)
	migrationFeeFactor := decimal.NewFromInt(100).
		Sub(decimal.NewFromInt(int64(params.MigrationFee.FeePercentage))).
		Div(decimal.NewFromInt(100))

	if len(params.LiquidityWeights) != 16 {
		return ConfigParameters{}, errors.New("liquidityWeights length must be 16")
	}

	for i := 1; i < 17; i++ {
		pi := decimal.NewFromBigInt(sqrtPrices[i], 0)
		piMinus := decimal.NewFromBigInt(sqrtPrices[i-1], 0)
		k := decimalFromFloat(params.LiquidityWeights[i-1])
		w1 := pi.Sub(piMinus).DivRound(pi.Mul(piMinus), 37)
		w2 := pi.Sub(piMinus).
			Mul(migrationFeeFactor).
			DivRound(pmaxWeight.Mul(pmaxWeight), 37)
		weight := k.Mul(w1.Add(w2))
		sumFactor = sumFactor.Add(weight)
	}

	if sumFactor.IsZero() {
		return ConfigParameters{}, errors.New("sumFactor must be greater than zero")
	}

	l1 := decimal.NewFromBigInt(totalSwapAndMigrationAmount, 0).Div(sumFactor)
	l1 = l1.Round(int32(36 - len(l1.Coefficient().String())))

	curve := make([]LiquidityDistributionParameters, 0, 16)
	for i := 0; i < 16; i++ {
		k := decimalFromFloat(params.LiquidityWeights[i])
		liquidity := FromDecimalToBig(l1.Mul(k))

		sqrtPrice := pMax
		if i < 15 {
			sqrtPrice = sqrtPrices[i+1]
		}
		// sqrtPrice := sqrtPrices[i+1]
		curve = append(curve, LiquidityDistributionParameters{
			SqrtPrice: BigToU128(sqrtPrice),
			Liquidity: BigToU128(liquidity),
		})
	}

	swapBaseAmount, err := GetBaseTokenForSwap(pMin, pMax, curve)
	if err != nil {
		return ConfigParameters{}, err
	}
	swapBaseAmountBuffer, err := GetSwapAmountWithBuffer(swapBaseAmount, pMin, curve)
	if err != nil {
		return ConfigParameters{}, err
	}
	migrationAmount := new(big.Int).Sub(totalSwapAndMigrationAmount, swapBaseAmountBuffer)

	migrationQuoteAmount := new(big.Int).Mul(migrationAmount, pMax)
	migrationQuoteAmount.Mul(migrationQuoteAmount, pMax)
	migrationQuoteAmount.Rsh(migrationQuoteAmount, 128)

	migrationQuoteThreshold := GetMigrationQuoteThresholdFromMigrationQuoteAmount(
		decimal.NewFromBigInt(migrationQuoteAmount, 0),
		decimalFromUint64(uint64(params.MigrationFee.FeePercentage)),
	)
	migrationQuoteThresholdInLamport := FromDecimalToBig(migrationQuoteThreshold)

	totalDynamicSupply, err := GetTotalSupplyFromCurve(
		migrationQuoteThresholdInLamport,
		pMin,
		curve,
		lockedVesting,
		params.MigrationOption,
		totalLeftover,
		params.MigrationFee.FeePercentage,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	if totalDynamicSupply.Cmp(totalSupply) > 0 {
		leftOverDelta := new(big.Int).Sub(totalDynamicSupply, totalSupply)
		if leftOverDelta.Cmp(totalLeftover) >= 0 {
			return ConfigParameters{}, errors.New("leftOverDelta must be less than totalLeftover")
		}
	}

	migrationQuoteThresholdU64, err := BigIntToU64(migrationQuoteThresholdInLamport)
	if err != nil {
		return ConfigParameters{}, err
	}
	totalSupplyU64, err := BigIntToU64(totalSupply)
	if err != nil {
		return ConfigParameters{}, err
	}

	marketCapFeeScheduler, err := buildMigratedPoolMarketCapFeeSchedulerParams(
		params.MigratedPoolMarketCapFeeSchedulerParams,
		params.BaseFeeParams,
		params.MigratedPoolBaseFeeMode,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	cfg := ConfigParameters{
		PoolFees: PoolFeeParameters{
			BaseFee: baseFee,
		},
		ActivationType:             uint8(params.ActivationType),
		CollectFeeMode:             uint8(params.CollectFeeMode),
		MigrationOption:            uint8(params.MigrationOption),
		TokenType:                  uint8(params.TokenType),
		TokenDecimal:               uint8(params.TokenBaseDecimal),
		MigrationQuoteThreshold:    migrationQuoteThresholdU64,
		PartnerLiquidityPercentage: params.PartnerLiquidityPercentage,
		PartnerPermanentLockedLiquidityPercentage: params.PartnerPermanentLockedLiquidityPercentage,
		CreatorLiquidityPercentage:                params.CreatorLiquidityPercentage,
		CreatorPermanentLockedLiquidityPercentage: params.CreatorPermanentLockedLiquidityPercentage,
		SqrtStartPrice:     BigToU128(pMin),
		LockedVesting:      lockedVesting,
		MigrationFeeOption: uint8(params.MigrationFeeOption),
		TokenSupply: &TokenSupplyParams{
			PreMigrationTokenSupply:  totalSupplyU64,
			PostMigrationTokenSupply: totalSupplyU64,
		},
		CreatorTradingFeePercentage: params.CreatorTradingFeePercentage,
		MigratedPoolFee:             migratedPoolFeeParams,
		PoolCreationFee:             poolCreationFeeInLamports,
		PartnerLiquidityVestingInfo: partnerLiquidityVestingInfo,
		CreatorLiquidityVestingInfo: creatorLiquidityVestingInfo,
		MigratedPoolBaseFeeMode: uint8(derefDammV2BaseFeeMode(
			params.MigratedPoolBaseFeeMode,
			DammV2BaseFeeModeFeeTimeSchedulerLinear,
		)),
		MigratedPoolMarketCapFeeSchedulerParams: marketCapFeeScheduler,
		EnableFirstSwapWithMinFee:               params.EnableFirstSwapWithMinFee,
		Curve:                                   curve,
		MigrationFee: MigrationFee{
			FeePercentage:        params.MigrationFee.FeePercentage,
			CreatorFeePercentage: params.MigrationFee.CreatorFeePercentage,
		},
		TokenUpdateAuthority: params.TokenUpdateAuthority,
	}

	if params.DynamicFeeEnabled {
		dynamicFeeBps := baseFeeBpsForDynamicFee(params.BaseFeeParams)
		dynamicFee, err := GetDynamicFeeParams(dynamicFeeBps, uint16(MaxPriceChangePercentageDefault))
		if err != nil {
			return ConfigParameters{}, err
		}
		cfg.PoolFees.DynamicFee = dynamicFee
	}

	return cfg, nil
}

func BuildCurveWithCustomSqrtPrices(params BuildCurveWithCustomSqrtPricesParams) (ConfigParameters, error) {
	if len(params.SqrtPrices) < 2 {
		return ConfigParameters{}, errors.New("sqrtPrices array must have at least 2 elements")
	}

	// sqrtPrices := make([]*big.Int, len(params.SqrtPrices))
	sqrtPrices := params.SqrtPrices
	for i := range sqrtPrices {
		// sqrtPrices[i] = params.SqrtPrices[i]
		if i > 0 && sqrtPrices[i].Cmp(sqrtPrices[i-1]) <= 0 {
			return ConfigParameters{}, errors.New("sqrtPrices must be in ascending order")
		}
	}

	liquidityWeights := params.LiquidityWeights
	if len(liquidityWeights) == 0 {
		numSegments := len(sqrtPrices) - 1
		liquidityWeights = make([]uint64, numSegments)
		for i := 0; i < numSegments; i++ {
			liquidityWeights[i] = 1
		}
	} else if len(liquidityWeights) != len(sqrtPrices)-1 {
		return ConfigParameters{}, errors.New("liquidityWeights length must equal sqrtPrices.length - 1")
	}

	baseFee, err := GetBaseFeeParams(params.BaseFeeParams, params.TokenQuoteDecimal, params.ActivationType)
	if err != nil {
		return ConfigParameters{}, err
	}

	lockedVesting, err := GetLockedVestingParams(
		params.LockedVestingParams.TotalLockedVestingAmount,
		params.LockedVestingParams.NumberOfVestingPeriod,
		params.LockedVestingParams.CliffUnlockAmount,
		params.LockedVestingParams.TotalVestingDuration,
		params.LockedVestingParams.CliffDurationFromMigrationTime,
		params.TokenBaseDecimal,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	partnerVestingParams := params.PartnerLiquidityVestingInfoParams
	if partnerVestingParams == nil {
		partnerVestingParams = &DefaultLiquidityVestingInfoParams
	}
	partnerLiquidityVestingInfo, err := GetLiquidityVestingInfoParams(
		partnerVestingParams.VestingPercentage,
		partnerVestingParams.BpsPerPeriod,
		partnerVestingParams.NumberOfPeriods,
		partnerVestingParams.CliffDurationFromMigrationTime,
		partnerVestingParams.TotalDuration,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	creatorVestingParams := params.CreatorLiquidityVestingInfoParams
	if creatorVestingParams == nil {
		creatorVestingParams = &DefaultLiquidityVestingInfoParams
	}
	creatorLiquidityVestingInfo, err := GetLiquidityVestingInfoParams(
		creatorVestingParams.VestingPercentage,
		creatorVestingParams.BpsPerPeriod,
		creatorVestingParams.NumberOfPeriods,
		creatorVestingParams.CliffDurationFromMigrationTime,
		creatorVestingParams.TotalDuration,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	poolCreationFeeInLamports, err := lamportsU64FromUint64(params.PoolCreationFee, TokenDecimalNine)
	if err != nil {
		return ConfigParameters{}, err
	}

	migratedPoolFeeParams := GetMigratedPoolFeeParams(
		params.MigrationOption,
		params.MigrationFeeOption,
		params.MigratedPoolFee,
	)

	pMin := sqrtPrices[0]
	pMax := sqrtPrices[len(sqrtPrices)-1]

	totalSupply, err := lamportsFromUint64(params.TotalTokenSupply, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}
	totalLeftover, err := lamportsFromUint64(params.Leftover, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}
	totalVestingAmount := GetTotalVestingAmount(lockedVesting)
	totalSwapAndMigrationAmount := new(big.Int).Sub(totalSupply, totalVestingAmount)
	totalSwapAndMigrationAmount.Sub(totalSwapAndMigrationAmount, totalLeftover)

	sumFactor := decimal.Zero
	pmaxWeight := decimal.NewFromBigInt(pMax, 0)
	migrationFeeFactor := decimal.NewFromInt(100).
		Sub(decimal.NewFromInt(int64(params.MigrationFee.FeePercentage))).
		Div(decimal.NewFromInt(100))

	numSegments := len(sqrtPrices) - 1

	for i := 0; i < numSegments; i++ {
		pi := decimal.NewFromBigInt(sqrtPrices[i+1], 0)
		piMinus := decimal.NewFromBigInt(sqrtPrices[i], 0)
		k := decimalFromUint64(liquidityWeights[i])

		w1 := pi.Sub(piMinus).DivRound(pi.Mul(piMinus), 37)
		w2 := pi.Sub(piMinus).
			Mul(migrationFeeFactor).
			DivRound(pmaxWeight.Mul(pmaxWeight), 37)
		weight := k.Mul(w1.Add(w2))
		sumFactor = sumFactor.Add(weight)
	}

	if sumFactor.IsZero() {
		return ConfigParameters{}, errors.New("sumFactor must be greater than zero")
	}

	l1 := decimal.NewFromBigInt(totalSwapAndMigrationAmount, 0).Div(sumFactor)
	l1 = l1.Round(int32(36 - len(l1.Coefficient().String())))

	curve := make([]LiquidityDistributionParameters, 0, numSegments)
	for i := 0; i < numSegments; i++ {
		k := decimalFromUint64(liquidityWeights[i])
		liquidity := FromDecimalToBig(l1.Mul(k))
		sqrtPrice := sqrtPrices[i+1]
		curve = append(curve, LiquidityDistributionParameters{
			SqrtPrice: BigToU128(sqrtPrice),
			Liquidity: BigToU128(liquidity),
		})
	}

	swapBaseAmount, err := GetBaseTokenForSwap(pMin, pMax, curve)
	if err != nil {
		return ConfigParameters{}, err
	}
	swapBaseAmountBuffer, err := GetSwapAmountWithBuffer(swapBaseAmount, pMin, curve)
	if err != nil {
		return ConfigParameters{}, err
	}
	migrationAmount := new(big.Int).Sub(totalSwapAndMigrationAmount, swapBaseAmountBuffer)

	migrationQuoteAmount := new(big.Int).Mul(migrationAmount, pMax)
	migrationQuoteAmount.Mul(migrationQuoteAmount, pMax)
	migrationQuoteAmount.Rsh(migrationQuoteAmount, 128)

	migrationQuoteThreshold := GetMigrationQuoteThresholdFromMigrationQuoteAmount(
		decimal.NewFromBigInt(migrationQuoteAmount, 0),
		decimalFromUint64(uint64(params.MigrationFee.FeePercentage)),
	)
	migrationQuoteThresholdInLamport := FromDecimalToBig(migrationQuoteThreshold)

	totalDynamicSupply, err := GetTotalSupplyFromCurve(
		migrationQuoteThresholdInLamport,
		pMin,
		curve,
		lockedVesting,
		params.MigrationOption,
		totalLeftover,
		params.MigrationFee.FeePercentage,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	if totalDynamicSupply.Cmp(totalSupply) > 0 {
		leftOverDelta := new(big.Int).Sub(totalDynamicSupply, totalSupply)
		if leftOverDelta.Cmp(totalLeftover) >= 0 {
			return ConfigParameters{}, errors.New("leftOverDelta must be less than totalLeftover")
		}
	}

	migrationQuoteThresholdU64, err := BigIntToU64(migrationQuoteThresholdInLamport)
	if err != nil {
		return ConfigParameters{}, err
	}
	totalSupplyU64, err := BigIntToU64(totalSupply)
	if err != nil {
		return ConfigParameters{}, err
	}

	marketCapFeeScheduler, err := buildMigratedPoolMarketCapFeeSchedulerParams(
		params.MigratedPoolMarketCapFeeSchedulerParams,
		params.BaseFeeParams,
		params.MigratedPoolBaseFeeMode,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	cfg := ConfigParameters{
		PoolFees: PoolFeeParameters{
			BaseFee: baseFee,
		},
		ActivationType:             uint8(params.ActivationType),
		CollectFeeMode:             uint8(params.CollectFeeMode),
		MigrationOption:            uint8(params.MigrationOption),
		TokenType:                  uint8(params.TokenType),
		TokenDecimal:               uint8(params.TokenBaseDecimal),
		MigrationQuoteThreshold:    migrationQuoteThresholdU64,
		PartnerLiquidityPercentage: params.PartnerLiquidityPercentage,
		PartnerPermanentLockedLiquidityPercentage: params.PartnerPermanentLockedLiquidityPercentage,
		CreatorLiquidityPercentage:                params.CreatorLiquidityPercentage,
		CreatorPermanentLockedLiquidityPercentage: params.CreatorPermanentLockedLiquidityPercentage,
		SqrtStartPrice:     BigToU128(pMin),
		LockedVesting:      lockedVesting,
		MigrationFeeOption: uint8(params.MigrationFeeOption),
		TokenSupply: &TokenSupplyParams{
			PreMigrationTokenSupply:  totalSupplyU64,
			PostMigrationTokenSupply: totalSupplyU64,
		},
		CreatorTradingFeePercentage: params.CreatorTradingFeePercentage,
		MigratedPoolFee:             migratedPoolFeeParams,
		PoolCreationFee:             poolCreationFeeInLamports,
		PartnerLiquidityVestingInfo: partnerLiquidityVestingInfo,
		CreatorLiquidityVestingInfo: creatorLiquidityVestingInfo,
		MigratedPoolBaseFeeMode: uint8(derefDammV2BaseFeeMode(
			params.MigratedPoolBaseFeeMode,
			DammV2BaseFeeModeFeeTimeSchedulerLinear,
		)),
		MigratedPoolMarketCapFeeSchedulerParams: marketCapFeeScheduler,
		EnableFirstSwapWithMinFee:               params.EnableFirstSwapWithMinFee,
		Curve:                                   curve,
		MigrationFee: MigrationFee{
			FeePercentage:        params.MigrationFee.FeePercentage,
			CreatorFeePercentage: params.MigrationFee.CreatorFeePercentage,
		},
		TokenUpdateAuthority: params.TokenUpdateAuthority,
	}

	if params.DynamicFeeEnabled {
		dynamicFeeBps := baseFeeBpsForDynamicFee(params.BaseFeeParams)
		dynamicFee, err := GetDynamicFeeParams(dynamicFeeBps, uint16(MaxPriceChangePercentageDefault))
		if err != nil {
			return ConfigParameters{}, err
		}
		cfg.PoolFees.DynamicFee = dynamicFee
	}

	return cfg, nil
}

type twoCurveResult struct {
	IsOk           bool
	SqrtStartPrice *big.Int
	Curve          []LiquidityDistributionParameters
}

func GetTwoCurve(migrationSqrtPrice, midSqrtPrice, initialSqrtPrice, swapAmount, migrationQuoteThreshold *big.Int) (twoCurveResult, error) {
	p0 := decimal.NewFromBigInt(initialSqrtPrice, 0)
	p1 := decimal.NewFromBigInt(midSqrtPrice, 0)
	p2 := decimal.NewFromBigInt(migrationSqrtPrice, 0)

	a1 := decimal.NewFromInt(1).DivRound(p0, 38).Sub(decimal.NewFromInt(1).DivRound(p1, 38))
	b1 := decimal.NewFromInt(1).DivRound(p1, 38).Sub(decimal.NewFromInt(1).DivRound(p2, 38))
	c1 := decimal.NewFromBigInt(swapAmount, 0)

	a2 := p1.Sub(p0)
	b2 := p2.Sub(p1)
	c2 := decimal.NewFromBigInt(migrationQuoteThreshold, 0).Mul(decimal.NewFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128), 0))
	// c2 := decimal.NewFromBigInt(migrationQuoteThreshold, 0).Mul(decimal.NewFromInt(2).Pow(decimal.NewFromInt(128)))
	c2 = c2.Round(int32(20 - len(c2.Coefficient().String())))

	denom0 := a1.Mul(b2).Sub(a2.Mul(b1))
	denom1 := b1.Mul(a2).Sub(b2.Mul(a1))
	if denom0.IsZero() || denom1.IsZero() {
		return twoCurveResult{IsOk: false}, nil
	}

	l0 := c1.Mul(b2).Sub(c2.Mul(b1)).Div(denom0)
	l1 := c1.Mul(a2).Sub(c2.Mul(a1)).Div(denom1)

	if l0.Sign() < 0 || l1.Sign() < 0 {
		return twoCurveResult{IsOk: false}, nil
	}

	curve := []LiquidityDistributionParameters{
		{
			SqrtPrice: BigToU128(midSqrtPrice),
			Liquidity: BigToU128(FromDecimalToBig(l0)),
		},
		{
			SqrtPrice: BigToU128(migrationSqrtPrice),
			Liquidity: BigToU128(FromDecimalToBig(l1)),
		},
	}

	return twoCurveResult{
		IsOk:           true,
		SqrtStartPrice: new(big.Int).Set(initialSqrtPrice),
		Curve:          curve,
	}, nil
}

func GetFirstCurve(migrationSqrtPrice, migrationBaseAmount, swapAmount, migrationQuoteThreshold *big.Int, migrationFeePercent uint8) (*big.Int, []LiquidityDistributionParameters, error) {
	migrationSqrtPriceDecimal := decimal.NewFromBigInt(migrationSqrtPrice, 0)
	migrationBaseAmountDecimal := decimal.NewFromBigInt(migrationBaseAmount, 0)
	swapAmountDecimal := decimal.NewFromBigInt(swapAmount, 0)
	migrationFeePercentDecimal := decimal.NewFromInt(int64(migrationFeePercent))

	denominator := swapAmountDecimal.
		Mul(decimal.NewFromInt(100).Sub(migrationFeePercentDecimal)).
		Div(decimal.NewFromInt(100))
	if denominator.IsZero() {
		return nil, nil, errors.New("swap amount denominator must be non-zero")
	}

	sqrtStartPriceDecimal := migrationSqrtPriceDecimal.
		Mul(migrationBaseAmountDecimal).
		Div(denominator)

	sqrtStartPrice := FromDecimalToBig(sqrtStartPriceDecimal)
	liquidity, err := GetLiquidity(swapAmount, migrationQuoteThreshold, sqrtStartPrice, migrationSqrtPrice)
	if err != nil {
		return nil, nil, err
	}

	curve := []LiquidityDistributionParameters{
		{
			SqrtPrice: BigToU128(migrationSqrtPrice),
			Liquidity: BigToU128(liquidity),
		},
	}
	return sqrtStartPrice, curve, nil
}

func GetTotalSupplyFromCurve(
	migrationQuoteThreshold *big.Int,
	sqrtStartPrice *big.Int,
	curve []LiquidityDistributionParameters,
	lockedVesting LockedVestingParameters,
	migrationOption MigrationOption,
	leftover *big.Int,
	migrationFeePercent uint8,
) (*big.Int, error) {
	sqrtMigrationPrice, err := GetMigrationThresholdPrice(migrationQuoteThreshold, sqrtStartPrice, curve)
	if err != nil {
		return nil, err
	}
	swapBaseAmount, err := GetBaseTokenForSwap(sqrtStartPrice, sqrtMigrationPrice, curve)
	if err != nil {
		return nil, err
	}
	swapBaseAmountBuffer, err := GetSwapAmountWithBuffer(swapBaseAmount, sqrtStartPrice, curve)
	if err != nil {
		return nil, err
	}

	migrationQuoteAmount := GetMigrationQuoteAmountFromMigrationQuoteThreshold(
		decimal.NewFromBigInt(migrationQuoteThreshold, 0),
		migrationFeePercent,
	)
	migrationBaseAmount, err := GetMigrationBaseToken(
		FromDecimalToBig(migrationQuoteAmount),
		sqrtMigrationPrice,
		migrationOption,
	)
	if err != nil {
		return nil, err
	}

	totalVestingAmount := GetTotalVestingAmount(lockedVesting)
	minimumBaseSupplyWithBuffer := new(big.Int).
		Add(swapBaseAmountBuffer, migrationBaseAmount)
	minimumBaseSupplyWithBuffer.Add(minimumBaseSupplyWithBuffer, totalVestingAmount)
	minimumBaseSupplyWithBuffer.Add(minimumBaseSupplyWithBuffer, leftover)
	return minimumBaseSupplyWithBuffer, nil
}

func GetSqrtPriceFromPrice(price string, tokenADecimal, tokenBDecimal int) (*big.Int, error) {
	decimalPrice, err := decimal.NewFromString(price)
	if err != nil {
		return nil, err
	}
	adjusted := decimalPrice.DivRound(decimal.New(1, int32(tokenADecimal)-int32(tokenBDecimal)), 25)

	sqrtValue, err := decimalSqrt(adjusted)
	if err != nil {
		return nil, err
	}
	sqrtValueQ64 := sqrtValue.Mul(decimal.NewFromBigInt(OneQ64, 0))
	return FromDecimalToBig(sqrtValueQ64), nil
}

func GetSqrtPriceFromMarketCap(marketCap float64, totalSupply uint64, tokenBaseDecimal, tokenQuoteDecimal TokenDecimal) (*big.Int, error) {
	if totalSupply == 0 {
		return nil, errors.New("totalSupply must be greater than zero")
	}
	price := decimalFromFloat(marketCap).Div(decimalFromUint64(totalSupply))
	return GetSqrtPriceFromPrice(price.String(), int(tokenBaseDecimal), int(tokenQuoteDecimal))
}

func GetMigrationQuoteAmount(migrationMarketCap, percentageSupplyOnMigration decimal.Decimal) decimal.Decimal {
	return migrationMarketCap.Mul(percentageSupplyOnMigration).Div(decimal.NewFromInt(100))
}

func GetPercentageSupplyOnMigration(
	initialMarketCap decimal.Decimal,
	migrationMarketCap decimal.Decimal,
	lockedVesting LockedVestingParameters,
	totalLeftover *big.Int,
	totalTokenSupply *big.Int,
) (decimal.Decimal, error) {
	marketCapRatio := initialMarketCap.Div(migrationMarketCap)
	sqrtRatio, err := decimalSqrt(marketCapRatio)
	if err != nil {
		return decimal.Zero, err
	}

	totalVestingAmount := GetTotalVestingAmount(lockedVesting)
	vestingPercentage := decimal.NewFromBigInt(totalVestingAmount, 0).
		Mul(decimal.NewFromInt(100)).
		Div(decimal.NewFromBigInt(totalTokenSupply, 0))
	leftoverPercentage := decimal.NewFromBigInt(totalLeftover, 0).
		Mul(decimal.NewFromInt(100)).
		Div(decimal.NewFromBigInt(totalTokenSupply, 0))

	numerator := decimal.NewFromInt(100).
		Mul(sqrtRatio).
		Sub(vestingPercentage.Add(leftoverPercentage).Mul(sqrtRatio))
	denominator := decimal.NewFromInt(1).Add(sqrtRatio)
	return numerator.Div(denominator), nil
}

func CalculateAdjustedPercentageSupplyOnMigration(
	initialMarketCap decimal.Decimal,
	migrationMarketCap decimal.Decimal,
	migrationFee struct {
		FeePercentage        uint8
		CreatorFeePercentage uint8
	},
	lockedVesting LockedVestingParameters,
	totalLeftover *big.Int,
	totalTokenSupply *big.Int,
) (decimal.Decimal, error) {
	f := decimal.NewFromInt(int64(migrationFee.FeePercentage)).Div(decimal.NewFromInt(100))

	totalVestingAmount := GetTotalVestingAmount(lockedVesting)
	v := decimal.NewFromBigInt(totalVestingAmount, 0).
		Mul(decimal.NewFromInt(100)).
		Div(decimal.NewFromBigInt(totalTokenSupply, 0))
	l := decimal.NewFromBigInt(totalLeftover, 0).
		Mul(decimal.NewFromInt(100)).
		Div(decimal.NewFromBigInt(totalTokenSupply, 0))

	requiredRatio, err := decimalSqrt(initialMarketCap.Div(migrationMarketCap))
	if err != nil {
		return decimal.Zero, err
	}

	oneMinusF := decimal.NewFromInt(1).Sub(f)
	availablePercentage := decimal.NewFromInt(100).Sub(v).Sub(l)
	numerator := requiredRatio.Mul(oneMinusF).Mul(availablePercentage)
	denominator := decimal.NewFromInt(1).Add(requiredRatio.Mul(oneMinusF))
	return numerator.Div(denominator), nil
}

func GetDynamicFeeParams(baseFeeBps uint16, maxPriceChangePercentage uint16) (*DynamicFeeParameters, error) {
	maxAllowed := uint16(MaxPriceChangePercentageDefault)
	if maxPriceChangePercentage > maxAllowed {
		return nil, fmt.Errorf("maxPriceChangePercentage (%d) must be <= %d", maxPriceChangePercentage, maxAllowed)
	}

	priceRatio := decimal.NewFromInt(int64(maxPriceChangePercentage)).
		Div(decimal.NewFromInt(int64(MaxBasisPoint))).
		Add(decimal.NewFromInt(1))
	sqrtPriceRatio, err := decimalSqrt(priceRatio)
	if err != nil {
		return nil, err
	}
	sqrtPriceRatioQ64 := FromDecimalToBig(sqrtPriceRatio.Mul(decimal.NewFromBigInt(OneQ64, 0)))

	deltaBinId := new(big.Int).Sub(sqrtPriceRatioQ64, OneQ64)
	deltaBinId.Div(deltaBinId, BinStepBpsU128Default)
	deltaBinId.Mul(deltaBinId, big.NewInt(2))

	maxVolatilityAccumulator := new(big.Int).Mul(deltaBinId, big.NewInt(MaxBasisPoint))

	squareVfaBin := new(big.Int).Mul(maxVolatilityAccumulator, big.NewInt(BinStepBpsDefault))
	squareVfaBin.Mul(squareVfaBin, squareVfaBin)
	if squareVfaBin.Sign() == 0 {
		return nil, errors.New("squareVfaBin must be greater than zero")
	}

	baseFeeNumerator := BpsToFeeNumerator(uint64(baseFeeBps))
	maxDynamicFeeNumerator := new(big.Int).Mul(baseFeeNumerator, big.NewInt(int64(maxPriceChangePercentage)))
	maxDynamicFeeNumerator.Div(maxDynamicFeeNumerator, big.NewInt(100))

	vFee := new(big.Int).Mul(maxDynamicFeeNumerator, DynamicFeeScalingFactor)
	vFee.Sub(vFee, DynamicFeeRoundingOffset)

	variableFeeControl := new(big.Int).Div(vFee, squareVfaBin)

	maxVolatilityAccumulatorU32, err := bigIntToUint32(maxVolatilityAccumulator)
	if err != nil {
		return nil, err
	}
	variableFeeControlU32, err := bigIntToUint32(variableFeeControl)
	if err != nil {
		return nil, err
	}

	return &DynamicFeeParameters{
		BinStep:                  uint16(BinStepBpsDefault),
		BinStepU128:              BigToU128(BinStepBpsU128Default),
		FilterPeriod:             uint16(DynamicFeeFilterPeriodDefault),
		DecayPeriod:              uint16(DynamicFeeDecayPeriodDefault),
		ReductionFactor:          uint16(DynamicFeeReductionFactorDefault),
		MaxVolatilityAccumulator: maxVolatilityAccumulatorU32,
		VariableFeeControl:       variableFeeControlU32,
	}, nil
}

func GetStartingBaseFeeBpsFromBaseFeeParams(baseFeeParams BaseFeeParams) uint16 {
	if baseFeeParams.BaseFeeMode == BaseFeeModeRateLimiter {
		if baseFeeParams.RateLimiterParam == nil {
			return 0
		}
		return baseFeeParams.RateLimiterParam.BaseFeeBps
	}
	if baseFeeParams.FeeSchedulerParam == nil {
		return 0
	}
	return baseFeeParams.FeeSchedulerParam.EndingFeeBps
}

func GetMigratedPoolMarketCapFeeSchedulerParams(
	startingBaseFeeBps uint16,
	endingBaseFeeBps uint16,
	dammV2BaseFeeMode DammV2BaseFeeMode,
	numberOfPeriod uint16,
	sqrtPriceStepBps uint16,
	schedulerExpirationDuration uint32,
) (MigratedPoolMarketCapFeeSchedulerParameters, error) {
	if dammV2BaseFeeMode == DammV2BaseFeeModeFeeTimeSchedulerLinear ||
		dammV2BaseFeeMode == DammV2BaseFeeModeFeeTimeSchedulerExponential {
		return DefaultMigratedPoolMarketCapFeeSchedulerParams, nil
	}

	if dammV2BaseFeeMode == DammV2BaseFeeModeRateLimiter {
		return MigratedPoolMarketCapFeeSchedulerParameters{}, errors.New("RateLimiter is not supported for DAMM v2 migration")
	}

	if numberOfPeriod == 0 {
		return MigratedPoolMarketCapFeeSchedulerParameters{}, errors.New("numberOfPeriod must be greater than zero")
	}

	if startingBaseFeeBps <= endingBaseFeeBps {
		return MigratedPoolMarketCapFeeSchedulerParameters{}, fmt.Errorf("startingBaseFeeBps (%d) must be greater than endingBaseFeeBps (%d)", startingBaseFeeBps, endingBaseFeeBps)
	}
	if startingBaseFeeBps > MaxFeeBps {
		return MigratedPoolMarketCapFeeSchedulerParameters{}, fmt.Errorf("startingBaseFeeBps (%d) exceeds maximum allowed", startingBaseFeeBps)
	}
	if numberOfPeriod == 0 || sqrtPriceStepBps == 0 || schedulerExpirationDuration == 0 {
		return MigratedPoolMarketCapFeeSchedulerParameters{}, errors.New("numberOfPeriod, sqrtPriceStepBps, and schedulerExpirationDuration must be greater than zero")
	}

	maxBaseFeeNumerator := BpsToFeeNumerator(uint64(startingBaseFeeBps))
	minBaseFeeNumerator := BpsToFeeNumerator(uint64(endingBaseFeeBps))

	var reductionFactor *big.Int
	if dammV2BaseFeeMode == DammV2BaseFeeModeFeeMarketCapSchedulerLinear {
		totalReduction := new(big.Int).Sub(maxBaseFeeNumerator, minBaseFeeNumerator)
		reductionFactor = new(big.Int).Div(totalReduction, big.NewInt(int64(numberOfPeriod)))
	} else {
		ratio := decimal.NewFromBigInt(minBaseFeeNumerator, 0).Div(decimal.NewFromBigInt(maxBaseFeeNumerator, 0))
		decayBase := ratio.Pow(decimal.NewFromInt(1).Div(decimal.NewFromInt(int64(numberOfPeriod))))
		reductionFactor = FromDecimalToBig(
			decimal.NewFromInt(MaxBasisPoint).Mul(decimal.NewFromInt(1).Sub(decayBase)),
		)
	}

	reductionFactorU64, err := BigIntToU64(reductionFactor)
	if err != nil {
		return MigratedPoolMarketCapFeeSchedulerParameters{}, err
	}

	return MigratedPoolMarketCapFeeSchedulerParameters{
		NumberOfPeriod:              numberOfPeriod,
		SqrtPriceStepBps:            sqrtPriceStepBps,
		SchedulerExpirationDuration: schedulerExpirationDuration,
		ReductionFactor:             reductionFactorU64,
	}, nil
}

func GetLockedVestingParams(
	totalLockedVestingAmount uint64,
	numberOfVestingPeriod uint64,
	cliffUnlockAmount uint64,
	totalVestingDuration uint64,
	cliffDurationFromMigrationTime uint64,
	tokenBaseDecimal TokenDecimal,
) (LockedVestingParameters, error) {
	if totalLockedVestingAmount == 0 {
		return LockedVestingParameters{}, nil
	}

	if totalLockedVestingAmount == cliffUnlockAmount {
		amountPerPeriod, err := lamportsU64FromUint64(1, tokenBaseDecimal)
		if err != nil {
			return LockedVestingParameters{}, err
		}
		cliffUnlockLamports, err := lamportsU64FromUint64(totalLockedVestingAmount-1, tokenBaseDecimal)
		if err != nil {
			return LockedVestingParameters{}, err
		}
		return LockedVestingParameters{
			AmountPerPeriod:                amountPerPeriod,
			CliffDurationFromMigrationTime: cliffDurationFromMigrationTime,
			Frequency:                      1,
			NumberOfPeriod:                 1,
			CliffUnlockAmount:              cliffUnlockLamports,
		}, nil
	}

	if numberOfVestingPeriod == 0 {
		return LockedVestingParameters{}, errors.New("numberOfVestingPeriod must be greater than zero")
	}
	if totalVestingDuration == 0 {
		return LockedVestingParameters{}, errors.New("totalVestingDuration must be greater than zero")
	}
	if cliffUnlockAmount > totalLockedVestingAmount {
		return LockedVestingParameters{}, errors.New("cliff unlock amount cannot be greater than total locked vesting amount")
	}

	amountPerPeriod := (totalLockedVestingAmount - cliffUnlockAmount) / numberOfVestingPeriod
	roundedAmountPerPeriod := amountPerPeriod
	totalPeriodicAmount := roundedAmountPerPeriod * numberOfVestingPeriod
	remainder := totalLockedVestingAmount - (cliffUnlockAmount + totalPeriodicAmount)
	adjustedCliffUnlockAmount := cliffUnlockAmount + remainder

	periodFrequency := totalVestingDuration / numberOfVestingPeriod

	amountPerPeriodLamports, err := lamportsU64FromUint64(roundedAmountPerPeriod, tokenBaseDecimal)
	if err != nil {
		return LockedVestingParameters{}, err
	}
	cliffUnlockLamports, err := lamportsU64FromUint64(adjustedCliffUnlockAmount, tokenBaseDecimal)
	if err != nil {
		return LockedVestingParameters{}, err
	}

	return LockedVestingParameters{
		AmountPerPeriod:                amountPerPeriodLamports,
		CliffDurationFromMigrationTime: cliffDurationFromMigrationTime,
		Frequency:                      periodFrequency,
		NumberOfPeriod:                 numberOfVestingPeriod,
		CliffUnlockAmount:              cliffUnlockLamports,
	}, nil
}

func GetLiquidityVestingInfoParams(
	vestingPercentage uint8,
	bpsPerPeriod uint16,
	numberOfPeriods uint16,
	cliffDurationFromMigrationTime uint32,
	totalDuration uint64,
) (LiquidityVestingInfoParameters, error) {
	if vestingPercentage > 100 {
		return LiquidityVestingInfoParameters{}, errors.New("vestingPercentage must be between 0 and 100")
	}

	if vestingPercentage == 0 {
		if bpsPerPeriod != 0 || numberOfPeriods != 0 || cliffDurationFromMigrationTime != 0 || totalDuration != 0 {
			return LiquidityVestingInfoParameters{}, errors.New("if vestingPercentage is 0, all other parameters must be 0")
		}
		return LiquidityVestingInfoParameters{}, nil
	}

	if numberOfPeriods == 0 {
		return LiquidityVestingInfoParameters{}, errors.New("numberOfPeriods must be greater than zero when vestingPercentage > 0")
	}
	if totalDuration == 0 {
		return LiquidityVestingInfoParameters{}, errors.New("totalDuration must be greater than zero")
	}
	if int64(bpsPerPeriod) < 0 || bpsPerPeriod > MaxBasisPoint {
		return LiquidityVestingInfoParameters{}, fmt.Errorf("bpsPerPeriod must be between 0 and %d", MaxBasisPoint)
	}

	frequency := totalDuration / uint64(numberOfPeriods)
	if frequency == 0 {
		return LiquidityVestingInfoParameters{}, errors.New("frequency must be greater than zero")
	}

	totalBps := uint32(bpsPerPeriod) * uint32(numberOfPeriods)
	if totalBps > MaxBasisPoint {
		return LiquidityVestingInfoParameters{}, fmt.Errorf("total BPS must not exceed %d", MaxBasisPoint)
	}

	totalVestingDuration := uint64(cliffDurationFromMigrationTime) + uint64(numberOfPeriods)*frequency
	if totalVestingDuration > MaxLockDurationInSeconds {
		return LiquidityVestingInfoParameters{}, fmt.Errorf("total vesting duration must not exceed %d", MaxLockDurationInSeconds)
	}

	if cliffDurationFromMigrationTime == 0 && numberOfPeriods == 0 {
		return LiquidityVestingInfoParameters{}, errors.New("if cliffDurationFromMigrationTime is 0, numberOfPeriods must be > 0")
	}

	if frequency > uint64(^uint32(0)) {
		return LiquidityVestingInfoParameters{}, errors.New("frequency overflows uint32")
	}

	return LiquidityVestingInfoParameters{
		VestingPercentage:              vestingPercentage,
		BpsPerPeriod:                   bpsPerPeriod,
		NumberOfPeriods:                numberOfPeriods,
		CliffDurationFromMigrationTime: cliffDurationFromMigrationTime,
		Frequency:                      uint32(frequency),
	}, nil
}

func GetBaseFeeParams(baseFeeParams BaseFeeParams, tokenQuoteDecimal TokenDecimal, activationType ActivationType) (BaseFeeParameters, error) {
	if baseFeeParams.BaseFeeMode == BaseFeeModeRateLimiter {
		if baseFeeParams.RateLimiterParam == nil {
			return BaseFeeParameters{}, errors.New("rate limiter parameters are required for RateLimiter mode")
		}
		return getRateLimiterParams(
			baseFeeParams.RateLimiterParam.BaseFeeBps,
			baseFeeParams.RateLimiterParam.FeeIncrementBps,
			baseFeeParams.RateLimiterParam.ReferenceAmount,
			baseFeeParams.RateLimiterParam.MaxLimiterDuration,
			tokenQuoteDecimal,
			activationType,
		)
	}

	if baseFeeParams.FeeSchedulerParam == nil {
		return BaseFeeParameters{}, errors.New("fee scheduler parameters are required for FeeScheduler mode")
	}
	return getFeeSchedulerParams(
		baseFeeParams.FeeSchedulerParam.StartingFeeBps,
		baseFeeParams.FeeSchedulerParam.EndingFeeBps,
		baseFeeParams.BaseFeeMode,
		baseFeeParams.FeeSchedulerParam.NumberOfPeriod,
		baseFeeParams.FeeSchedulerParam.TotalDuration,
	)
}

func GetMigratedPoolFeeParams(
	migrationOption MigrationOption,
	migrationFeeOption MigrationFeeOption,
	migratedPoolFee *struct {
		CollectFeeMode CollectFeeMode
		DynamicFee     DammV2DynamicFeeMode
		PoolFeeBps     uint16
	},
) MigratedPoolFee {
	defaultFeeParams := MigratedPoolFee{
		CollectFeeMode: 0,
		DynamicFee:     0,
		PoolFeeBps:     0,
	}

	if migrationOption == MigrationOptionMetDamm {
		return defaultFeeParams
	}

	if migrationOption == MigrationOptionMetDammV2 {
		if migrationFeeOption == MigrationFeeOptionCustomizable && migratedPoolFee != nil {
			return MigratedPoolFee{
				CollectFeeMode: uint8(migratedPoolFee.CollectFeeMode),
				DynamicFee:     uint8(migratedPoolFee.DynamicFee),
				PoolFeeBps:     migratedPoolFee.PoolFeeBps,
			}
		}
		return defaultFeeParams
	}

	return defaultFeeParams
}

func buildCurveInternal(
	params BuildCurveBaseParams,
	percentageSupplyOnMigration decimal.Decimal,
	migrationQuoteThreshold decimal.Decimal,
) (ConfigParameters, error) {
	baseFee, err := GetBaseFeeParams(params.BaseFeeParams, params.TokenQuoteDecimal, params.ActivationType)
	if err != nil {
		return ConfigParameters{}, err
	}

	lockedVesting, err := GetLockedVestingParams(
		params.LockedVestingParams.TotalLockedVestingAmount,
		params.LockedVestingParams.NumberOfVestingPeriod,
		params.LockedVestingParams.CliffUnlockAmount,
		params.LockedVestingParams.TotalVestingDuration,
		params.LockedVestingParams.CliffDurationFromMigrationTime,
		params.TokenBaseDecimal,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	partnerVestingParams := params.PartnerLiquidityVestingInfoParams
	if partnerVestingParams == nil {
		partnerVestingParams = &DefaultLiquidityVestingInfoParams
	}
	partnerLiquidityVestingInfo, err := GetLiquidityVestingInfoParams(
		partnerVestingParams.VestingPercentage,
		partnerVestingParams.BpsPerPeriod,
		partnerVestingParams.NumberOfPeriods,
		partnerVestingParams.CliffDurationFromMigrationTime,
		partnerVestingParams.TotalDuration,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	creatorVestingParams := params.CreatorLiquidityVestingInfoParams
	if creatorVestingParams == nil {
		creatorVestingParams = &DefaultLiquidityVestingInfoParams
	}
	creatorLiquidityVestingInfo, err := GetLiquidityVestingInfoParams(
		creatorVestingParams.VestingPercentage,
		creatorVestingParams.BpsPerPeriod,
		creatorVestingParams.NumberOfPeriods,
		creatorVestingParams.CliffDurationFromMigrationTime,
		creatorVestingParams.TotalDuration,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	poolCreationFeeInLamports, err := lamportsU64FromUint64(params.PoolCreationFee, TokenDecimalNine)
	if err != nil {
		return ConfigParameters{}, err
	}

	migratedPoolFeeParams := GetMigratedPoolFeeParams(
		params.MigrationOption,
		params.MigrationFeeOption,
		params.MigratedPoolFee,
	)

	migrationBaseSupply := decimalFromUint64(params.TotalTokenSupply).
		Mul(percentageSupplyOnMigration).
		Div(decimal.NewFromInt(100))

	totalSupply, err := lamportsFromUint64(params.TotalTokenSupply, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	migrationQuoteAmount := GetMigrationQuoteAmountFromMigrationQuoteThreshold(
		migrationQuoteThreshold,
		params.MigrationFee.FeePercentage,
	)

	migrationPrice := migrationQuoteAmount.DivRound(migrationBaseSupply, 25)

	migrationQuoteThresholdInLamport, err := lamportsFromDecimal(migrationQuoteThreshold, params.TokenQuoteDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	totalLeftover, err := lamportsFromUint64(params.Leftover, params.TokenBaseDecimal)
	if err != nil {
		return ConfigParameters{}, err
	}

	migrateSqrtPrice, err := GetSqrtPriceFromPrice(
		migrationPrice.String(),
		int(params.TokenBaseDecimal),
		int(params.TokenQuoteDecimal),
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	migrationQuoteAmountInLamport := FromDecimalToBig(
		migrationQuoteAmount.Mul(decimal.New(1, int32(params.TokenQuoteDecimal))),
	)
	migrationBaseAmount, err := GetMigrationBaseToken(migrationQuoteAmountInLamport, migrateSqrtPrice, params.MigrationOption)
	if err != nil {
		return ConfigParameters{}, err
	}

	totalVestingAmount := GetTotalVestingAmount(lockedVesting)
	swapAmount := new(big.Int).Sub(totalSupply, migrationBaseAmount)
	swapAmount.Sub(swapAmount, totalVestingAmount)
	swapAmount.Sub(swapAmount, totalLeftover)

	sqrtStartPrice, curve, err := GetFirstCurve(
		migrateSqrtPrice,
		migrationBaseAmount,
		swapAmount,
		migrationQuoteThresholdInLamport,
		params.MigrationFee.FeePercentage,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	totalDynamicSupply, err := GetTotalSupplyFromCurve(
		migrationQuoteThresholdInLamport,
		sqrtStartPrice,
		curve,
		lockedVesting,
		params.MigrationOption,
		totalLeftover,
		params.MigrationFee.FeePercentage,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	remainingAmount := new(big.Int).Sub(totalSupply, totalDynamicSupply)
	lastLiquidity, err := mathutil.GetInitialLiquidityFromDeltaBase(remainingAmount, MaxSqrtPrice, migrateSqrtPrice)
	if err != nil {
		return ConfigParameters{}, err
	}
	if lastLiquidity.Sign() != 0 {
		curve = append(curve, LiquidityDistributionParameters{
			SqrtPrice: BigToU128(MaxSqrtPrice),
			Liquidity: BigToU128(lastLiquidity),
		})
	}

	migrationQuoteThresholdU64, err := BigIntToU64(migrationQuoteThresholdInLamport)
	if err != nil {
		return ConfigParameters{}, err
	}
	totalSupplyU64, err := BigIntToU64(totalSupply)
	if err != nil {
		return ConfigParameters{}, err
	}

	marketCapFeeScheduler, err := buildMigratedPoolMarketCapFeeSchedulerParams(
		params.MigratedPoolMarketCapFeeSchedulerParams,
		params.BaseFeeParams,
		params.MigratedPoolBaseFeeMode,
	)
	if err != nil {
		return ConfigParameters{}, err
	}

	cfg := ConfigParameters{
		PoolFees: PoolFeeParameters{
			BaseFee: baseFee,
		},
		CollectFeeMode:             uint8(params.CollectFeeMode),
		MigrationOption:            uint8(params.MigrationOption),
		ActivationType:             uint8(params.ActivationType),
		TokenType:                  uint8(params.TokenType),
		TokenDecimal:               uint8(params.TokenBaseDecimal),
		PartnerLiquidityPercentage: params.PartnerLiquidityPercentage,
		PartnerPermanentLockedLiquidityPercentage: params.PartnerPermanentLockedLiquidityPercentage,
		CreatorLiquidityPercentage:                params.CreatorLiquidityPercentage,
		CreatorPermanentLockedLiquidityPercentage: params.CreatorPermanentLockedLiquidityPercentage,
		MigrationQuoteThreshold:                   migrationQuoteThresholdU64,
		SqrtStartPrice:                            BigToU128(sqrtStartPrice),
		LockedVesting:                             lockedVesting,
		MigrationFeeOption:                        uint8(params.MigrationFeeOption),
		TokenSupply: &TokenSupplyParams{
			PreMigrationTokenSupply:  totalSupplyU64,
			PostMigrationTokenSupply: totalSupplyU64,
		},
		CreatorTradingFeePercentage: params.CreatorTradingFeePercentage,
		TokenUpdateAuthority:        params.TokenUpdateAuthority,
		MigrationFee: MigrationFee{
			FeePercentage:        params.MigrationFee.FeePercentage,
			CreatorFeePercentage: params.MigrationFee.CreatorFeePercentage,
		},
		MigratedPoolFee:             migratedPoolFeeParams,
		PoolCreationFee:             poolCreationFeeInLamports,
		PartnerLiquidityVestingInfo: partnerLiquidityVestingInfo,
		CreatorLiquidityVestingInfo: creatorLiquidityVestingInfo,
		MigratedPoolBaseFeeMode: uint8(derefDammV2BaseFeeMode(
			params.MigratedPoolBaseFeeMode,
			DammV2BaseFeeModeFeeTimeSchedulerLinear,
		)),
		MigratedPoolMarketCapFeeSchedulerParams: marketCapFeeScheduler,
		EnableFirstSwapWithMinFee:               params.EnableFirstSwapWithMinFee,
		Curve:                                   curve,
	}

	if params.DynamicFeeEnabled {
		dynamicFeeBps := baseFeeBpsForDynamicFee(params.BaseFeeParams)
		dynamicFee, err := GetDynamicFeeParams(dynamicFeeBps, uint16(MaxPriceChangePercentageDefault))
		if err != nil {
			return ConfigParameters{}, err
		}
		cfg.PoolFees.DynamicFee = dynamicFee
	}

	return cfg, nil
}

func GetTotalVestingAmount(lockedVesting LockedVestingParameters) *big.Int {
	total := new(big.Int).Mul(
		new(big.Int).SetUint64(lockedVesting.AmountPerPeriod),
		new(big.Int).SetUint64(lockedVesting.NumberOfPeriod),
	)
	total.Add(total, new(big.Int).SetUint64(lockedVesting.CliffUnlockAmount))
	return total
}

func GetLiquidity(baseAmount, quoteAmount, minSqrtPrice, maxSqrtPrice *big.Int) (*big.Int, error) {
	liquidityFromBase, err := mathutil.GetInitialLiquidityFromDeltaBase(baseAmount, maxSqrtPrice, minSqrtPrice)
	if err != nil {
		return nil, err
	}
	liquidityFromQuote, err := mathutil.GetInitialLiquidityFromDeltaQuote(quoteAmount, minSqrtPrice, maxSqrtPrice)
	if err != nil {
		return nil, err
	}
	if liquidityFromBase.Cmp(liquidityFromQuote) < 0 {
		return liquidityFromBase, nil
	}
	return liquidityFromQuote, nil
}

func getFeeSchedulerParams(
	startingBaseFeeBps uint16,
	endingBaseFeeBps uint16,
	baseFeeMode BaseFeeMode,
	numberOfPeriod uint64,
	totalDuration uint64,
) (BaseFeeParameters, error) {
	if startingBaseFeeBps == endingBaseFeeBps {
		if numberOfPeriod != 0 || totalDuration != 0 {
			return BaseFeeParameters{}, errors.New("numberOfPeriod and totalDuration must both be zero")
		}
		cliffFeeNumerator := BpsToFeeNumerator(uint64(startingBaseFeeBps))
		return BaseFeeParameters{
			CliffFeeNumerator: mustU64(cliffFeeNumerator),
			BaseFeeMode:       uint8(BaseFeeModeFeeSchedulerLinear),
		}, nil
	}

	if numberOfPeriod == 0 {
		return BaseFeeParameters{}, errors.New("numberOfPeriod must be greater than zero")
	}
	if startingBaseFeeBps > MaxFeeBps {
		return BaseFeeParameters{}, fmt.Errorf("startingBaseFeeBps (%d) exceeds maximum", startingBaseFeeBps)
	}
	if endingBaseFeeBps < MinFeeBps {
		return BaseFeeParameters{}, fmt.Errorf("endingBaseFeeBps (%d) is less than minimum", endingBaseFeeBps)
	}
	if endingBaseFeeBps > startingBaseFeeBps {
		return BaseFeeParameters{}, errors.New("endingBaseFeeBps must be <= startingBaseFeeBps")
	}
	if totalDuration == 0 {
		return BaseFeeParameters{}, errors.New("totalDuration must be greater than zero")
	}
	if numberOfPeriod > uint64(^uint16(0)) {
		return BaseFeeParameters{}, errors.New("numberOfPeriod overflows uint16")
	}

	maxBaseFeeNumerator := BpsToFeeNumerator(uint64(startingBaseFeeBps))
	minBaseFeeNumerator := BpsToFeeNumerator(uint64(endingBaseFeeBps))

	periodFrequency := totalDuration / numberOfPeriod
	var reductionFactor *big.Int
	if baseFeeMode == BaseFeeModeFeeSchedulerLinear {
		totalReduction := new(big.Int).Sub(maxBaseFeeNumerator, minBaseFeeNumerator)
		reductionFactor = new(big.Int).Div(totalReduction, big.NewInt(int64(numberOfPeriod)))
	} else {
		ratio := decimal.NewFromBigInt(minBaseFeeNumerator, 0).Div(decimal.NewFromBigInt(maxBaseFeeNumerator, 0))
		decayBase := ratio.Pow(decimal.NewFromInt(1).Div(decimal.NewFromInt(int64(numberOfPeriod))))
		reductionFactor = FromDecimalToBig(
			decimal.NewFromInt(MaxBasisPoint).Mul(decimal.NewFromInt(1).Sub(decayBase)),
		)
	}

	reductionFactorU64, err := BigIntToU64(reductionFactor)
	if err != nil {
		return BaseFeeParameters{}, err
	}

	return BaseFeeParameters{
		CliffFeeNumerator: mustU64(maxBaseFeeNumerator),
		FirstFactor:       uint16(numberOfPeriod),
		SecondFactor:      periodFrequency,
		ThirdFactor:       reductionFactorU64,
		BaseFeeMode:       uint8(baseFeeMode),
	}, nil
}

func getRateLimiterParams(
	baseFeeBps uint16,
	feeIncrementBps uint16,
	referenceAmount uint64,
	maxLimiterDuration uint64,
	tokenQuoteDecimal TokenDecimal,
	activationType ActivationType,
) (BaseFeeParameters, error) {
	cliffFeeNumerator := BpsToFeeNumerator(uint64(baseFeeBps))
	feeIncrementNumerator := BpsToFeeNumerator(uint64(feeIncrementBps))

	if baseFeeBps == 0 || feeIncrementBps == 0 || referenceAmount == 0 || maxLimiterDuration == 0 {
		return BaseFeeParameters{}, errors.New("all rate limiter parameters must be greater than zero")
	}
	if baseFeeBps > MaxFeeBps {
		return BaseFeeParameters{}, fmt.Errorf("baseFeeBps (%d) exceeds maximum allowed", baseFeeBps)
	}
	if baseFeeBps < MinFeeBps {
		return BaseFeeParameters{}, fmt.Errorf("baseFeeBps (%d) is less than minimum allowed", baseFeeBps)
	}
	if feeIncrementBps > MaxFeeBps {
		return BaseFeeParameters{}, fmt.Errorf("feeIncrementBps (%d) exceeds maximum allowed", feeIncrementBps)
	}
	if feeIncrementNumerator.Cmp(big.NewInt(FeeDenominator)) >= 0 {
		return BaseFeeParameters{}, errors.New("fee increment numerator must be less than FEE_DENOMINATOR")
	}

	deltaNumerator := new(big.Int).Sub(big.NewInt(MaxFeeNumerator), cliffFeeNumerator)
	maxIndex := new(big.Int).Div(deltaNumerator, feeIncrementNumerator)
	if maxIndex.Cmp(big.NewInt(1)) < 0 {
		return BaseFeeParameters{}, errors.New("fee increment is too large for the given base fee")
	}

	if cliffFeeNumerator.Cmp(big.NewInt(MinFeeNumerator)) < 0 || cliffFeeNumerator.Cmp(big.NewInt(MaxFeeNumerator)) > 0 {
		return BaseFeeParameters{}, errors.New("base fee must be between minimum and maximum")
	}

	maxDuration := uint64(MaxRateLimiterDurationInSeconds)
	if activationType == ActivationTypeSlot {
		maxDuration = uint64(MaxRateLimiterDurationInSlots)
	}
	if maxLimiterDuration > maxDuration {
		return BaseFeeParameters{}, fmt.Errorf("max limiter duration exceeds maximum allowed value of %d", maxDuration)
	}

	referenceAmountLamports, err := lamportsU64FromUint64(referenceAmount, tokenQuoteDecimal)
	if err != nil {
		return BaseFeeParameters{}, err
	}

	return BaseFeeParameters{
		CliffFeeNumerator: mustU64(cliffFeeNumerator),
		FirstFactor:       feeIncrementBps,
		SecondFactor:      maxLimiterDuration,
		ThirdFactor:       referenceAmountLamports,
		BaseFeeMode:       uint8(BaseFeeModeRateLimiter),
	}, nil
}

func decimalSqrt(d decimal.Decimal) (decimal.Decimal, error) {
	if d.Sign() < 0 {
		return decimal.Zero, errors.New("cannot sqrt negative value")
	}
	if d.IsZero() {
		return decimal.Zero, nil
	}
	f := new(big.Float).SetPrec(256)
	if _, ok := f.SetString(d.String()); !ok {
		return decimal.Zero, errors.New("failed to parse decimal for sqrt")
	}
	sqrt := new(big.Float).SetPrec(256).Sqrt(f)
	str := sqrt.Text('f', -1)
	return decimal.NewFromString(str)
}

func decimalRootPow2(d decimal.Decimal, power int) (decimal.Decimal, error) {
	out := d
	for i := 0; i < power; i++ {
		var err error
		out, err = decimalSqrt(out)
		if err != nil {
			return decimal.Zero, err
		}
	}
	return out, nil
}

func powBigIntDecimal(v *big.Int, exp int) decimal.Decimal {
	out := decimal.NewFromInt(1)
	base := decimal.NewFromBigInt(v, 0)
	for i := 0; i < exp; i++ {
		out = out.Mul(base)
	}
	return out
}

func sqrtBigIntDecimalMul(a, b *big.Int) (*big.Int, error) {
	product := decimal.NewFromBigInt(a, 0).Mul(decimal.NewFromBigInt(b, 0))
	sqrtValue, err := decimalSqrt(product)
	if err != nil {
		return nil, err
	}
	return FromDecimalToBig(sqrtValue), nil
}

func fourthRootBigIntDecimalMul(a, b decimal.Decimal) (*big.Int, error) {
	product := a.Mul(b)
	root, err := decimalRootPow2(product, 2)
	if err != nil {
		return nil, err
	}
	return FromDecimalToBig(root), nil
}

func buildMigratedPoolMarketCapFeeSchedulerParams(
	params *MigratedPoolMarketCapFeeSchedulerParams,
	baseFeeParams BaseFeeParams,
	baseFeeMode *DammV2BaseFeeMode,
) (MigratedPoolMarketCapFeeSchedulerParameters, error) {
	if params == nil {
		return DefaultMigratedPoolMarketCapFeeSchedulerParams, nil
	}

	mode := derefDammV2BaseFeeMode(baseFeeMode, DammV2BaseFeeModeFeeTimeSchedulerLinear)
	starting := GetStartingBaseFeeBpsFromBaseFeeParams(baseFeeParams)
	out, err := GetMigratedPoolMarketCapFeeSchedulerParams(
		starting,
		params.EndingBaseFeeBps,
		mode,
		params.NumberOfPeriod,
		params.SqrtPriceStepBps,
		params.SchedulerExpirationDuration,
	)
	if err != nil {
		return MigratedPoolMarketCapFeeSchedulerParameters{}, err
	}
	return out, nil
}

func baseFeeBpsForDynamicFee(baseFeeParams BaseFeeParams) uint16 {
	if baseFeeParams.BaseFeeMode == BaseFeeModeRateLimiter {
		if baseFeeParams.RateLimiterParam == nil {
			return 0
		}
		return baseFeeParams.RateLimiterParam.BaseFeeBps
	}
	if baseFeeParams.FeeSchedulerParam == nil {
		return 0
	}
	return baseFeeParams.FeeSchedulerParam.EndingFeeBps
}

func derefDammV2BaseFeeMode(v *DammV2BaseFeeMode, fallback DammV2BaseFeeMode) DammV2BaseFeeMode {
	if v == nil {
		return fallback
	}
	return *v
}

func decimalFromUint64(v uint64) decimal.Decimal {
	return decimal.NewFromUint64(v)
	// return decimal.NewFromBigInt(new(big.Int).SetUint64(v), 0)
}

func decimalFromFloat(v float64) decimal.Decimal {
	return decimal.NewFromFloat(v)
}

func lamportsFromUint64(amount uint64, tokenDecimal TokenDecimal) (*big.Int, error) {
	return ConvertToLamports(strconv.FormatUint(amount, 10), int32(tokenDecimal))
}

func lamportsU64FromUint64(amount uint64, tokenDecimal TokenDecimal) (uint64, error) {
	val, err := lamportsFromUint64(amount, tokenDecimal)
	if err != nil {
		return 0, err
	}
	return BigIntToU64(val)
}

func lamportsFromDecimal(amount decimal.Decimal, tokenDecimal TokenDecimal) (*big.Int, error) {
	val, err := ConvertToLamports(amount.String(), int32(tokenDecimal))
	if err != nil {
		return nil, err
	}
	return val, nil
}

func bigIntToUint32(v *big.Int) (uint32, error) {
	if v.Sign() < 0 {
		return 0, errors.New("value must be non-negative")
	}
	if v.BitLen() > 32 {
		return 0, errors.New("value overflows uint32")
	}
	return uint32(v.Uint64()), nil
}

func mustU64(v *big.Int) uint64 {
	out, err := BigIntToU64(v)
	if err != nil {
		panic(err)
	}
	return out
}
