package dynamic_bonding_curve

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/u128"

	"github.com/shopspring/decimal"
)

// BaseFee
type BaseFee struct {
	CliffFeeNumerator *big.Int
	FirstFactor       int64    // feeScheduler: numberOfPeriod, rateLimiter: feeIncrementBps
	SecondFactor      *big.Int // feeScheduler: periodFrequency, rateLimiter: maxLimiterDuration
	ThirdFactor       *big.Int // feeScheduler: reductionFactor, rateLimiter: referenceAmount
	BaseFeeMode       BaseFeeMode
}

// FeeSchedulerParams
type FeeSchedulerParams struct {
	StartingFeeBps int64
	EndingFeeBps   int64
	NumberOfPeriod uint16
	TotalDuration  uint16
}

// RateLimiterParams
type RateLimiterParams struct {
	BaseFeeBps         int64
	FeeIncrementBps    int64
	ReferenceAmount    int
	MaxLimiterDuration int
}

// LockedVestingParams
type LockedVestingParams struct {
	TotalLockedVestingAmount       int64
	NumberOfVestingPeriod          int64
	CliffUnlockAmount              int64
	TotalVestingDuration           int64
	CliffDurationFromMigrationTime int64
}

// BaseFeeParams: union type
type BaseFeeParams struct {
	BaseFeeMode       BaseFeeMode
	FeeSchedulerParam *FeeSchedulerParams
	RateLimiterParam  *RateLimiterParams
}

// BuildCurveBaseParam
type BuildCurveBaseParam struct {
	TotalTokenSupply            float64
	MigrationOption             MigrationOption
	TokenBaseDecimal            TokenDecimal
	TokenQuoteDecimal           TokenDecimal
	LockedVestingParam          LockedVestingParams
	BaseFeeParams               BaseFeeParams
	DynamicFeeEnabled           bool
	ActivationType              ActivationType
	CollectFeeMode              CollectFeeMode
	MigrationFeeOption          MigrationFeeOption
	TokenType                   TokenType
	PartnerLpPercentage         uint8
	CreatorLpPercentage         uint8
	PartnerLockedLpPercentage   uint8
	CreatorLockedLpPercentage   uint8
	CreatorTradingFeePercentage uint8
	Leftover                    int
	TokenUpdateAuthority        TokenUpdateAuthorityOption
	MigrationFee                MigrationFee
	MigratedPoolFee             *MigratedPoolFee
}

// BuildCurveParam
type BuildCurveParam struct {
	BuildCurveBaseParam
	PercentageSupplyOnMigration float64
	MigrationQuoteThreshold     float64
}

// BuildCurveWithMarketCapParam
type BuildCurveWithMarketCapParam struct {
	BuildCurveBaseParam
	InitialMarketCap   float64
	MigrationMarketCap float64
}

// BuildCurveWithTwoSegmentsParam
type BuildCurveWithTwoSegmentsParam struct {
	BuildCurveBaseParam
	InitialMarketCap            float64
	MigrationMarketCap          float64
	PercentageSupplyOnMigration int64
}

// BuildCurveWithLiquidityWeightsParam
type BuildCurveWithLiquidityWeightsParam struct {
	BuildCurveBaseParam
	InitialMarketCap   float64
	MigrationMarketCap float64
	LiquidityWeights   []float64
}

func BuildCurve(param BuildCurveParam) (*ConfigParameters, error) {
	baseFee, err := getBaseFeeParams(param.BaseFeeParams, param.TokenQuoteDecimal, param.ActivationType)
	if err != nil {
		return nil, err
	}

	lockedVesting, err := getLockedVestingParams(
		param.LockedVestingParam.TotalLockedVestingAmount,
		param.LockedVestingParam.NumberOfVestingPeriod,
		param.LockedVestingParam.CliffUnlockAmount,
		param.LockedVestingParam.TotalVestingDuration,
		param.LockedVestingParam.CliffDurationFromMigrationTime,
		param.TokenBaseDecimal,
	)
	if err != nil {
		return nil, err
	}

	migratedPoolFeeParams := getMigratedPoolFeeParams(
		param.MigrationOption,
		param.MigrationFeeOption,
		param.MigratedPoolFee,
	)

	migrationBaseSupply := decimal.NewFromFloat(param.TotalTokenSupply).Mul(decimal.NewFromFloat(param.PercentageSupplyOnMigration)).Div(decimal.NewFromInt(100))

	totalSupply := convertToLamports(param.TotalTokenSupply, param.TokenBaseDecimal)

	migrationQuoteAmount := getMigrationQuoteAmountFromMigrationQuoteThreshold(
		decimal.NewFromFloat(param.MigrationQuoteThreshold),
		param.MigrationFee.FeePercentage,
	)

	migrationPrice := migrationQuoteAmount.DivRound(migrationBaseSupply, 25)

	migrationQuoteThresholdInLamport := convertToLamports(param.MigrationQuoteThreshold, param.TokenQuoteDecimal)
	totalLeftover := convertToLamports(param.Leftover, param.TokenBaseDecimal)

	migrateSqrtPrice := getSqrtPriceFromPrice(migrationPrice, param.TokenBaseDecimal, param.TokenQuoteDecimal)

	migrationQuoteAmountInLamport := convertDecimalToBN(migrationQuoteAmount.Mul(decimal.New(1, int32(param.TokenQuoteDecimal))))

	migrationBaseAmount, err := getMigrationBaseToken(migrationQuoteAmountInLamport, migrateSqrtPrice, param.MigrationOption)
	if err != nil {
		return nil, err
	}

	totalVestingAmount := getTotalVestingAmount(&lockedVesting)

	swapAmount := totalSupply.Sub(migrationBaseAmount).Sub(totalVestingAmount).Sub(totalLeftover)

	firstCurve, err := getFirstCurve(migrateSqrtPrice, migrationBaseAmount, swapAmount, migrationQuoteThresholdInLamport, param.MigrationFee.FeePercentage)
	if err != nil {
		return nil, err
	}

	sqrtStartPrice := firstCurve.SqrtStartPrice
	curve := firstCurve.Curve

	totalDynamicSupply, err := getTotalSupplyFromCurve(
		migrationQuoteThresholdInLamport,
		sqrtStartPrice,
		curve,
		&lockedVesting,
		param.MigrationOption,
		totalLeftover,
		param.MigrationFee.FeePercentage,
	)
	if err != nil {
		return nil, err
	}

	remainingAmount := totalSupply.Sub(totalDynamicSupply)

	lastLiquidity := getInitialLiquidityFromDeltaBase(remainingAmount, decimal.NewFromBigInt(MAX_SQRT_PRICE, 0), migrateSqrtPrice)

	if !lastLiquidity.IsZero() {
		curve = append(curve, LiquidityDistributionParameters{
			SqrtPrice: u128.GenUint128FromString(MAX_SQRT_PRICE.String()),
			Liquidity: u128.GenUint128FromString(lastLiquidity.String()),
		})
	}

	return &ConfigParameters{
		PoolFees: PoolFeeParameters{
			BaseFee: baseFee,
			DynamicFee: func() *DynamicFeeParameters {
				if !param.DynamicFeeEnabled {
					return nil
				}
				feeBps := param.BaseFeeParams.FeeSchedulerParam.EndingFeeBps
				if param.BaseFeeParams.BaseFeeMode == BaseFeeModeRateLimiter {
					feeBps = param.BaseFeeParams.RateLimiterParam.BaseFeeBps
				}
				return getDynamicFeeParams(feeBps, MAX_PRICE_CHANGE_BPS_DEFAULT)
			}(),
		},
		ActivationType:            param.ActivationType,
		CollectFeeMode:            param.CollectFeeMode,
		MigrationOption:           param.MigrationOption,
		TokenType:                 param.TokenType,
		TokenDecimal:              param.TokenBaseDecimal,
		MigrationQuoteThreshold:   migrationQuoteThresholdInLamport.BigInt().Uint64(),
		PartnerLpPercentage:       param.PartnerLpPercentage,
		CreatorLpPercentage:       param.CreatorLpPercentage,
		PartnerLockedLpPercentage: param.PartnerLockedLpPercentage,
		CreatorLockedLpPercentage: param.CreatorLockedLpPercentage,
		SqrtStartPrice:            u128.GenUint128FromString(sqrtStartPrice.String()),
		LockedVesting:             lockedVesting,
		MigrationFeeOption:        param.MigrationFeeOption,
		TokenSupply: &TokenSupplyParams{
			PreMigrationTokenSupply:  totalSupply.BigInt().Uint64(),
			PostMigrationTokenSupply: totalSupply.BigInt().Uint64(),
		},
		CreatorTradingFeePercentage: param.CreatorTradingFeePercentage,
		TokenUpdateAuthority:        param.TokenUpdateAuthority,
		MigrationFee:                param.MigrationFee,
		MigratedPoolFee:             migratedPoolFeeParams,
		Padding:                     [7]uint64{},
		Curve:                       curve,
	}, nil
}

// BuildCurveWithMarketCap
func BuildCurveWithMarketCap(param BuildCurveWithMarketCapParam) (*ConfigParameters, error) {
	lockedVesting, err := getLockedVestingParams(
		param.LockedVestingParam.TotalLockedVestingAmount,
		param.LockedVestingParam.NumberOfVestingPeriod,
		param.LockedVestingParam.CliffUnlockAmount,
		param.LockedVestingParam.TotalVestingDuration,
		param.LockedVestingParam.CliffDurationFromMigrationTime,
		param.TokenBaseDecimal,
	)
	if err != nil {
		return nil, err
	}

	totalLeftover := convertToLamports(param.Leftover, param.TokenBaseDecimal)

	totalSupply := convertToLamports(param.TotalTokenSupply, param.TokenBaseDecimal)

	percentageSupplyOnMigration := getPercentageSupplyOnMigration(
		decimal.NewFromFloat(param.InitialMarketCap),
		decimal.NewFromFloat(param.MigrationMarketCap),
		&lockedVesting,
		totalLeftover,
		totalSupply,
	)

	migrationQuoteAmount := getMigrationQuoteAmount(
		decimal.NewFromFloat(param.MigrationMarketCap),
		percentageSupplyOnMigration,
	)
	migrationQuoteThreshold := getMigrationQuoteThresholdFromMigrationQuoteAmount(
		migrationQuoteAmount,
		param.MigrationFee.FeePercentage,
	).InexactFloat64()
	return BuildCurve(BuildCurveParam{
		BuildCurveBaseParam:         param.BuildCurveBaseParam,
		PercentageSupplyOnMigration: percentageSupplyOnMigration.InexactFloat64(),
		MigrationQuoteThreshold:     migrationQuoteThreshold,
	})
}

// BuildCurveWithTwoSegments
func BuildCurveWithTwoSegments(param BuildCurveWithTwoSegmentsParam) (*ConfigParameters, error) {

	baseFee, err := getBaseFeeParams(param.BaseFeeParams, param.TokenQuoteDecimal, param.ActivationType)
	if err != nil {
		return nil, err
	}

	lockedVesting, err := getLockedVestingParams(
		param.LockedVestingParam.TotalLockedVestingAmount,
		param.LockedVestingParam.NumberOfVestingPeriod,
		param.LockedVestingParam.CliffUnlockAmount,
		param.LockedVestingParam.TotalVestingDuration,
		param.LockedVestingParam.CliffDurationFromMigrationTime,
		param.TokenBaseDecimal,
	)
	if err != nil {
		return nil, err
	}

	migratedPoolFeeParams := getMigratedPoolFeeParams(
		param.MigrationOption,
		param.MigrationFeeOption,
		param.MigratedPoolFee,
	)

	migrationBaseSupply := decimal.NewFromFloat(param.TotalTokenSupply).Mul(decimal.NewFromInt(param.PercentageSupplyOnMigration)).Div(decimal.NewFromInt(100))

	totalSupply := convertToLamports(param.TotalTokenSupply, param.TokenBaseDecimal)

	migrationQuoteAmount := getMigrationQuoteAmount(
		decimal.NewFromFloat(param.MigrationMarketCap),
		decimal.NewFromInt(param.PercentageSupplyOnMigration),
	)

	migrationQuoteThreshold := getMigrationQuoteThresholdFromMigrationQuoteAmount(migrationQuoteAmount, param.MigrationFee.FeePercentage)

	migrationPrice := migrationQuoteAmount.Div(migrationBaseSupply)

	migrationQuoteThresholdInLamport := convertDecimalToBN(
		migrationQuoteThreshold.Mul(decimal.NewFromInt(1).Shift(int32(param.TokenQuoteDecimal))),
	)

	migrationQuoteAmountInLamport := convertDecimalToBN(
		migrationQuoteAmount.Mul(decimal.NewFromInt(1).Shift(int32(param.TokenQuoteDecimal))),
	)

	migrateSqrtPrice := getSqrtPriceFromPrice(
		migrationPrice,
		param.TokenBaseDecimal,
		param.TokenQuoteDecimal,
	)
	migrationBaseAmount, err := getMigrationBaseToken(migrationQuoteAmountInLamport, migrateSqrtPrice, param.MigrationOption)
	if err != nil {
		return nil, err
	}
	totalVestingAmount := getTotalVestingAmount(&lockedVesting)
	totalLeftover := convertToLamports(param.Leftover, param.TokenBaseDecimal)
	swapAmount := totalSupply.Sub(migrationBaseAmount).Sub(totalVestingAmount).Sub(totalLeftover)
	initialSqrtPrice := getSqrtPriceFromMarketCap(
		param.InitialMarketCap,
		param.TotalTokenSupply,
		param.TokenBaseDecimal,
		param.TokenQuoteDecimal,
	)

	midSqrtPriceDecimal1 := decimalSqrt(migrateSqrtPrice.Mul(initialSqrtPrice))

	midSqrtPrice1 := midSqrtPriceDecimal1.Floor()

	// mid_price2 = (p1 * p2^3)^(1/4)
	numerator1 := initialSqrtPrice

	numerator2 := migrateSqrtPrice.Pow(decimal.NewFromFloat(3))

	product1 := numerator1.Mul(numerator2)

	midSqrtPriceDecimal2, err := nth(product1, 0.25, 2)
	if err != nil {
		return nil, err
	}

	midSqrtPrice2 := midSqrtPriceDecimal2.Floor()

	// mid_price3 = (p1^3 * p2)^(1/4)
	numerator3 := initialSqrtPrice.Pow(decimal.NewFromInt(3))
	numerator4 := migrateSqrtPrice
	product2 := numerator3.Mul(numerator4)

	midSqrtPriceDecimal3, err := nth(product2, 0.25, 2) // product2.Pow(decimal.NewFromFloat(0.25))
	if err != nil {
		return nil, err
	}

	midSqrtPrice3 := midSqrtPriceDecimal3.Floor()

	midPrices := []decimal.Decimal{midSqrtPrice1, midSqrtPrice2, midSqrtPrice3}
	sqrtStartPrice := decimal.NewFromInt(0)
	var curve []LiquidityDistributionParameters

	for _, mid := range midPrices {
		result := getTwoCurve(
			migrateSqrtPrice,
			mid,
			initialSqrtPrice,
			swapAmount,
			migrationQuoteThresholdInLamport,
		)

		if result.IsOk {
			curve = result.Curve
			sqrtStartPrice = result.SqrtStartPrice
			break
		}
	}

	totalDynamicSupply, err := getTotalSupplyFromCurve(
		migrationQuoteThresholdInLamport,
		sqrtStartPrice,
		curve,
		&lockedVesting,
		param.MigrationOption,
		totalLeftover,
		param.MigrationFee.FeePercentage,
	)
	if err != nil {
		return nil, err
	}

	if totalDynamicSupply.Cmp(totalSupply) > 0 {
		leftOverDelta := totalDynamicSupply.Sub(totalSupply)
		if leftOverDelta.Cmp(totalLeftover) >= 0 {
			return nil, errors.New("leftOverDelta must be less than totalLeftover")
		}
	}

	return &ConfigParameters{
		PoolFees: PoolFeeParameters{
			BaseFee: baseFee,
			DynamicFee: func() *DynamicFeeParameters {
				if !param.DynamicFeeEnabled {
					return nil
				}

				var baseBps int64
				if param.BaseFeeParams.BaseFeeMode == BaseFeeModeRateLimiter {
					baseBps = param.BaseFeeParams.RateLimiterParam.BaseFeeBps
				} else {
					baseBps = param.BaseFeeParams.FeeSchedulerParam.EndingFeeBps
				}
				return getDynamicFeeParams(baseBps, MAX_PRICE_CHANGE_BPS_DEFAULT)
			}(),
		},
		ActivationType:            param.ActivationType,
		CollectFeeMode:            param.CollectFeeMode,
		MigrationOption:           param.MigrationOption,
		TokenType:                 param.TokenType,
		TokenDecimal:              param.TokenBaseDecimal,
		MigrationQuoteThreshold:   migrationQuoteThresholdInLamport.BigInt().Uint64(),
		PartnerLpPercentage:       param.PartnerLpPercentage,
		CreatorLpPercentage:       param.CreatorLpPercentage,
		PartnerLockedLpPercentage: param.PartnerLockedLpPercentage,
		CreatorLockedLpPercentage: param.CreatorLockedLpPercentage,
		SqrtStartPrice:            u128.GenUint128FromString(sqrtStartPrice.String()),
		LockedVesting:             lockedVesting,
		MigrationFeeOption:        param.MigrationFeeOption,
		TokenSupply: &TokenSupplyParams{
			PreMigrationTokenSupply:  totalSupply.BigInt().Uint64(),
			PostMigrationTokenSupply: totalSupply.BigInt().Uint64(),
		},
		CreatorTradingFeePercentage: param.CreatorTradingFeePercentage,
		TokenUpdateAuthority:        param.TokenUpdateAuthority,
		MigrationFee:                param.MigrationFee,
		MigratedPoolFee:             migratedPoolFeeParams,
		Padding:                     [7]uint64{},
		Curve:                       curve,
	}, nil
}

func BuildCurveWithLiquidityWeights(param BuildCurveWithLiquidityWeightsParam) (*ConfigParameters, error) {

	baseFee, err := getBaseFeeParams(param.BaseFeeParams, param.TokenQuoteDecimal, param.ActivationType)
	if err != nil {
		return nil, err
	}

	lockedVesting, err := getLockedVestingParams(
		param.LockedVestingParam.TotalLockedVestingAmount,
		param.LockedVestingParam.NumberOfVestingPeriod,
		param.LockedVestingParam.CliffUnlockAmount,
		param.LockedVestingParam.TotalVestingDuration,
		param.LockedVestingParam.CliffDurationFromMigrationTime,
		param.TokenBaseDecimal,
	)
	if err != nil {
		return nil, err
	}

	migratedPoolFee := getMigratedPoolFeeParams(param.MigrationOption, param.MigrationFeeOption, param.MigratedPoolFee)

	pMin := getSqrtPriceFromMarketCap(
		param.InitialMarketCap,
		param.TotalTokenSupply,
		param.TokenBaseDecimal,
		param.TokenQuoteDecimal,
	)

	pMax := getSqrtPriceFromMarketCap(
		param.MigrationMarketCap,
		param.TotalTokenSupply,
		param.TokenBaseDecimal,
		param.TokenQuoteDecimal,
	)

	// q = (pMax/pMin)^(1/16)

	decimalSqrt16 := func(x decimal.Decimal) decimal.Decimal {
		r := x
		for i := 0; i < 4; i++ {
			r = decimalSqrt(r)
		}
		return r
	}

	priceRatio := pMax.DivRound(pMin, 19)

	qDecimal := decimalSqrt16(priceRatio).Truncate(19)

	sqrtPrices := make([]decimal.Decimal, 0, 17)
	currentPrice := pMin

	for i := 0; i < 17; i++ {
		sqrtPrices = append(sqrtPrices, currentPrice)
		currentPrice = convertDecimalToBN(qDecimal.Mul(currentPrice))
	}

	totalSupply := convertToLamports(param.TotalTokenSupply, param.TokenBaseDecimal)
	totalLeftover := convertToLamports(param.Leftover, param.TokenBaseDecimal)
	totalVestingAmount := getTotalVestingAmount(&lockedVesting)

	totalSwapAndMigrationAmount := totalSupply.Sub(totalVestingAmount).Sub(totalLeftover)

	// sum_{i=1..16} k_i * [ (pi - p_{i-1})/(pi*p_{i-1}) + (pi - p_{i-1}) * (1 - fee)/pMax^2 ]
	sumFactor := decimal.Zero
	pMaxDec := decimal.RequireFromString(pMax.String())
	feeFactor := decimal.NewFromInt(int64(100 - param.MigrationFee.FeePercentage)).Div(decimal.NewFromInt(100))

	for i := 1; i < 17; i++ {
		pi := decimal.RequireFromString(sqrtPrices[i].String())

		pim := decimal.RequireFromString(sqrtPrices[i-1].String())

		k := decimal.NewFromFloat(param.LiquidityWeights[i-1])

		// w1 := pi.Sub(pim).DivRound(truncateSig(pi.Mul(pim), 20), 38)
		w1 := pi.Sub(pim).DivRound(pi.Mul(pim), 37)

		// w2 := pi.Sub(pim).Mul(feeFactor).DivRound(truncateSig(pMaxDec.Mul(pMaxDec), 20), 39)
		w2 := pi.Sub(pim).Mul(feeFactor).DivRound(pMaxDec.Mul(pMaxDec), 37)

		weight := k.Mul(w1.Add(w2))

		sumFactor = sumFactor.Add(weight).Truncate(38)
	}
	// l1 = (Swap_Amount + Base_Amount) / sumFactor
	// l1 := truncateSig(totalSwapAndMigrationAmount.Div(sumFactor.RoundUp(36)).Truncate(0), 20)
	l1 := totalSwapAndMigrationAmount.Div(sumFactor.RoundUp(36))

	curve := make([]LiquidityDistributionParameters, 0, 16)
	for i := 0; i < 16; i++ {
		k := decimal.NewFromFloat(param.LiquidityWeights[i])
		liq := truncateSig(convertDecimalToBN(l1.Mul(k)), 20)
		sPrice := pMax
		if i < 15 {
			sPrice = sqrtPrices[i+1]
		}

		curve = append(curve, LiquidityDistributionParameters{
			SqrtPrice: u128.GenUint128FromString(sPrice.String()),
			Liquidity: u128.GenUint128FromString(liq.String()),
		})
	}

	swapBaseAmount, err := getBaseTokenForSwap(pMin, pMax, curve)
	if err != nil {
		return nil, err
	}
	swapBaseAmountBuffer, err := getSwapAmountWithBuffer(swapBaseAmount, pMin, curve)
	if err != nil {
		return nil, err
	}

	migrationAmount := totalSwapAndMigrationAmount.Sub(swapBaseAmountBuffer)

	// quote = base * pMax^2 >> 128
	migrationQuoteAmount := migrationAmount.Mul(pMax).Mul(pMax)
	migrationQuoteAmount = decimal.NewFromBigInt(new(big.Int).Rsh(migrationQuoteAmount.BigInt(), 128), 0)

	mqThreshold := getMigrationQuoteThresholdFromMigrationQuoteAmount(
		decimal.RequireFromString(migrationQuoteAmount.String()),
		param.MigrationFee.FeePercentage,
	).Round(8)
	mqThresholdLamports := convertDecimalToBN(mqThreshold)

	totalDynamicSupply, err := getTotalSupplyFromCurve(
		mqThresholdLamports,
		pMin,
		curve,
		&lockedVesting,
		param.MigrationOption,
		totalLeftover,
		param.MigrationFee.FeePercentage,
	)
	if err != nil {
		return nil, err
	}

	if totalDynamicSupply.Cmp(totalSupply) > 0 {
		leftDelta := totalDynamicSupply.Sub(totalSupply)
		if leftDelta.Cmp(totalLeftover) >= 0 {
			panic("leftOverDelta must be less than totalLeftover")
		}
	}

	return &ConfigParameters{
		PoolFees: PoolFeeParameters{
			BaseFee: baseFee,
			DynamicFee: func() *DynamicFeeParameters {
				if !param.DynamicFeeEnabled {
					return nil
				}
				var baseBps int64
				if param.BaseFeeParams.BaseFeeMode == BaseFeeModeRateLimiter {
					baseBps = param.BaseFeeParams.RateLimiterParam.BaseFeeBps
				} else {
					baseBps = param.BaseFeeParams.FeeSchedulerParam.EndingFeeBps
				}
				return getDynamicFeeParams(baseBps, MAX_PRICE_CHANGE_BPS_DEFAULT)
			}(),
		},
		ActivationType:            param.ActivationType,
		CollectFeeMode:            param.CollectFeeMode,
		MigrationOption:           param.MigrationOption,
		TokenType:                 param.TokenType,
		TokenDecimal:              param.TokenBaseDecimal,
		MigrationQuoteThreshold:   mqThresholdLamports.BigInt().Uint64(),
		PartnerLpPercentage:       param.PartnerLpPercentage,
		CreatorLpPercentage:       param.CreatorLpPercentage,
		PartnerLockedLpPercentage: param.PartnerLockedLpPercentage,
		CreatorLockedLpPercentage: param.CreatorLockedLpPercentage,
		SqrtStartPrice:            u128.GenUint128FromString(pMin.String()),
		LockedVesting:             lockedVesting,
		MigrationFeeOption:        param.MigrationFeeOption,
		TokenSupply: &TokenSupplyParams{
			PreMigrationTokenSupply:  totalSupply.BigInt().Uint64(),
			PostMigrationTokenSupply: totalSupply.BigInt().Uint64(),
		},
		CreatorTradingFeePercentage: param.CreatorTradingFeePercentage,
		TokenUpdateAuthority:        param.TokenUpdateAuthority,
		MigrationFee:                param.MigrationFee,
		MigratedPoolFee:             migratedPoolFee,
		Padding:                     [7]uint64{},
		Curve:                       curve,
	}, nil
}
