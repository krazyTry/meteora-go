package dynamic_bonding_curve

import (
	"errors"
	"math/big"

	dmath "github.com/krazyTry/meteora-go/decimal_math"
	"github.com/krazyTry/meteora-go/u128"

	"github.com/shopspring/decimal"
)

// BaseFee represents the base fee configuration
type BaseFee struct {
	CliffFeeNumerator *big.Int    // Initial fee numerator value
	FirstFactor       int64       // Fee scheduler: number of periods, rate limiter: fee increment bps
	SecondFactor      *big.Int    // Fee scheduler: period frequency, rate limiter: max limiter duration
	ThirdFactor       *big.Int    // Fee scheduler: reduction factor, rate limiter: reference amount
	BaseFeeMode       BaseFeeMode // Base fee mode configuration
}

// FeeSchedulerParams represents fee scheduler configuration parameters
type FeeSchedulerParams struct {
	StartingFeeBps int64  // Starting fee in basis points
	EndingFeeBps   int64  // Ending fee in basis points
	NumberOfPeriod uint16 // Number of fee periods
	TotalDuration  uint16 // Total duration of fee schedule
}

// RateLimiterParams represents rate limiter configuration parameters
type RateLimiterParams struct {
	BaseFeeBps         int64 // Base fee in basis points
	FeeIncrementBps    int64 // Fee increment in basis points
	ReferenceAmount    int   // Reference amount for rate limiting
	MaxLimiterDuration int   // Maximum duration for rate limiter
}

// LockedVestingParams represents locked vesting configuration parameters
type LockedVestingParams struct {
	TotalLockedVestingAmount       int64 // Total amount locked for vesting
	NumberOfVestingPeriod          int64 // Number of vesting periods
	CliffUnlockAmount              int64 // Amount unlocked at cliff
	TotalVestingDuration           int64 // Total duration of vesting
	CliffDurationFromMigrationTime int64 // Cliff duration from migration time
}

// BaseFeeParams represents base fee parameters as a union type
type BaseFeeParams struct {
	BaseFeeMode       BaseFeeMode         // Base fee mode selection
	FeeSchedulerParam *FeeSchedulerParams // Fee scheduler parameters (optional)
	RateLimiterParam  *RateLimiterParams  // Rate limiter parameters (optional)
}

// BuildCurveBaseParam represents base parameters for building a curve
type BuildCurveBaseParam struct {
	TotalTokenSupply            float64                    // Total token supply
	MigrationOption             MigrationOption            // Migration option configuration
	TokenBaseDecimal            TokenDecimal               // Base token decimal places
	TokenQuoteDecimal           TokenDecimal               // Quote token decimal places
	LockedVestingParam          LockedVestingParams        // Locked vesting parameters
	BaseFeeParams               BaseFeeParams              // Base fee parameters
	DynamicFeeEnabled           bool                       // Whether dynamic fee is enabled
	ActivationType              ActivationType             // Pool activation type
	CollectFeeMode              CollectFeeMode             // Fee collection mode
	MigrationFeeOption          MigrationFeeOption         // Migration fee option
	TokenType                   TokenType                  // Token type (SPL or Token2022)
	PartnerLpPercentage         uint8                      // Partner LP percentage
	CreatorLpPercentage         uint8                      // Creator LP percentage
	PartnerLockedLpPercentage   uint8                      // Partner locked LP percentage
	CreatorLockedLpPercentage   uint8                      // Creator locked LP percentage
	CreatorTradingFeePercentage uint8                      // Creator trading fee percentage
	Leftover                    int64                      // Leftover amount
	TokenUpdateAuthority        TokenUpdateAuthorityOption // Token update authority option
	MigrationFee                MigrationFee               // Migration fee configuration
	MigratedPoolFee             *MigratedPoolFee           // Migrated pool fee (optional)
}

// BuildCurveParam represents parameters for building a standard curve
type BuildCurveParam struct {
	BuildCurveBaseParam
	PercentageSupplyOnMigration float64 // Percentage of supply available on migration
	MigrationQuoteThreshold     float64 // Quote threshold for migration
}

// BuildCurveWithMarketCapParam represents parameters for building a curve with market cap
type BuildCurveWithMarketCapParam struct {
	BuildCurveBaseParam
	InitialMarketCap   float64 // Initial market capitalization
	MigrationMarketCap float64 // Market cap at migration
}

// BuildCurveWithTwoSegmentsParam represents parameters for building a curve with two segments
type BuildCurveWithTwoSegmentsParam struct {
	BuildCurveBaseParam
	InitialMarketCap            float64 // Initial market capitalization
	MigrationMarketCap          float64 // Market cap at migration
	PercentageSupplyOnMigration int64   // Percentage of supply available on migration
}

// BuildCurveWithLiquidityWeightsParam represents parameters for building a curve with liquidity weights
type BuildCurveWithLiquidityWeightsParam struct {
	BuildCurveBaseParam
	InitialMarketCap   float64   // Initial market capitalization
	MigrationMarketCap float64   // Market cap at migration
	LiquidityWeights   []float64 // Liquidity weight distribution
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

	migrationBaseSupply := decimal.NewFromFloat(param.TotalTokenSupply).Mul(decimal.NewFromFloat(param.PercentageSupplyOnMigration)).Div(N100)

	totalSupply := convertToLamports(decimal.NewFromFloat(param.TotalTokenSupply), param.TokenBaseDecimal)

	migrationQuoteAmount := getMigrationQuoteAmountFromMigrationQuoteThreshold(
		decimal.NewFromFloat(param.MigrationQuoteThreshold),
		param.MigrationFee.FeePercentage,
	)

	migrationPrice := migrationQuoteAmount.DivRound(migrationBaseSupply, 38)

	migrationQuoteThresholdInLamport := convertToLamports(decimal.NewFromFloat(param.MigrationQuoteThreshold), param.TokenQuoteDecimal)

	totalLeftover := convertToLamports(decimal.NewFromInt(param.Leftover), param.TokenBaseDecimal)

	migrateSqrtPrice := getSqrtPriceFromPrice(migrationPrice, param.TokenBaseDecimal, param.TokenQuoteDecimal)

	migrationQuoteAmountInLamport := convertDecimalToBN(migrationQuoteAmount.Mul(decimal.New(1, int32(param.TokenQuoteDecimal))))

	migrationBaseAmount, err := getMigrationBaseToken(migrationQuoteAmountInLamport, migrateSqrtPrice, param.MigrationOption)
	if err != nil {
		return nil, err
	}

	totalVestingAmount := getTotalVestingAmount(&lockedVesting)

	swapAmount := totalSupply.Sub(migrationBaseAmount).Sub(totalVestingAmount).Sub(totalLeftover)

	sqrtStartPrice, curve, err := getFirstCurve(
		migrateSqrtPrice,
		migrationBaseAmount,
		swapAmount,
		migrationQuoteThresholdInLamport,
		decimal.NewFromUint64(uint64(param.MigrationFee.FeePercentage)),
	)
	if err != nil {
		return nil, err
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

	remainingAmount := totalSupply.Sub(totalDynamicSupply)

	lastLiquidity := getInitialLiquidityFromDeltaBase(remainingAmount, MAX_SQRT_PRICE, migrateSqrtPrice)

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

	totalLeftover := convertToLamports(decimal.NewFromInt(param.Leftover), param.TokenBaseDecimal)

	totalSupply := convertToLamports(decimal.NewFromFloat(param.TotalTokenSupply), param.TokenBaseDecimal)

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

	migrationBaseSupply := decimal.NewFromFloat(param.TotalTokenSupply).Mul(decimal.NewFromInt(param.PercentageSupplyOnMigration)).Div(N100)

	totalSupply := convertToLamports(decimal.NewFromFloat(param.TotalTokenSupply), param.TokenBaseDecimal)

	migrationQuoteAmount := getMigrationQuoteAmount(
		decimal.NewFromFloat(param.MigrationMarketCap),
		decimal.NewFromInt(param.PercentageSupplyOnMigration),
	)

	migrationQuoteThreshold := getMigrationQuoteThresholdFromMigrationQuoteAmount(migrationQuoteAmount, param.MigrationFee.FeePercentage)

	migrationPrice := migrationQuoteAmount.Div(migrationBaseSupply)

	migrationQuoteThresholdInLamport := convertDecimalToBN(
		migrationQuoteThreshold.Mul(N1.Shift(int32(param.TokenQuoteDecimal))),
	)

	migrationQuoteAmountInLamport := convertDecimalToBN(
		migrationQuoteAmount.Mul(N1.Shift(int32(param.TokenQuoteDecimal))),
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

	totalLeftover := convertToLamports(decimal.NewFromInt(param.Leftover), param.TokenBaseDecimal)

	swapAmount := totalSupply.Sub(migrationBaseAmount).Sub(totalVestingAmount).Sub(totalLeftover)

	initialSqrtPrice := getSqrtPriceFromMarketCap(
		param.InitialMarketCap,
		param.TotalTokenSupply,
		param.TokenBaseDecimal,
		param.TokenQuoteDecimal,
	)
	midSqrtPriceDecimal1 := dmath.Sqrt(migrateSqrtPrice.Mul(initialSqrtPrice), 64)

	midSqrtPrice1 := midSqrtPriceDecimal1.Floor()

	// mid_price2 = (p1 * p2^3)^(1/4)
	numerator1 := initialSqrtPrice

	numerator2 := dmath.Pow(migrateSqrtPrice, N3, 0) // migrateSqrtPrice.Pow(N3)

	product1 := numerator1.Mul(numerator2)

	midSqrtPriceDecimal2 := dmath.Pow(product1, N025, 2)
	midSqrtPrice2 := midSqrtPriceDecimal2.Floor()

	// mid_price3 = (p1^3 * p2)^(1/4)
	numerator3 := initialSqrtPrice.Pow(N3)
	numerator4 := migrateSqrtPrice
	product2 := numerator3.Mul(numerator4)

	midSqrtPriceDecimal3 := dmath.Pow(product2, N025, 2)
	midSqrtPrice3 := midSqrtPriceDecimal3.Floor()

	midPrices := []decimal.Decimal{midSqrtPrice1, midSqrtPrice2, midSqrtPrice3}
	sqrtStartPrice := N0
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

	priceRatio := pMax.DivRound(pMin, 19)

	qDecimal := dmath.Pow(priceRatio, N1.Div(decimal.NewFromInt(16)), 20).Truncate(19) //decimalSqrt16(priceRatio).Truncate(19)

	sqrtPrices := make([]decimal.Decimal, 17)
	currentPrice := pMin

	for i := range sqrtPrices {
		sqrtPrices[i] = currentPrice
		currentPrice = convertDecimalToBN(qDecimal.Mul(currentPrice))
	}

	totalSupply := convertToLamports(decimal.NewFromFloat(param.TotalTokenSupply), param.TokenBaseDecimal)
	totalLeftover := convertToLamports(decimal.NewFromInt(param.Leftover), param.TokenBaseDecimal)
	totalVestingAmount := getTotalVestingAmount(&lockedVesting)

	totalSwapAndMigrationAmount := totalSupply.Sub(totalVestingAmount).Sub(totalLeftover)

	// sum_{i=1..16} k_i * [ (pi - p_{i-1})/(pi*p_{i-1}) + (pi - p_{i-1}) * (1 - fee)/pMax^2 ]
	sumFactor := N0
	migrationFeeFactor := N100.Sub(decimal.NewFromUint64(uint64(param.MigrationFee.FeePercentage))).Div(N100)

	for i := 1; i < 17; i++ {

		k := decimal.NewFromFloat(param.LiquidityWeights[i-1])
		pim := decimal.RequireFromString(sqrtPrices[i-1].String())

		pi := decimal.RequireFromString(sqrtPrices[i].String())

		// w1 := pi.Sub(pim).DivRound(truncateSig(pi.Mul(pim), 20), 38)
		w1 := pi.Sub(pim).DivRound(pi.Mul(pim), 37)

		// w2 := pi.Sub(pim).Mul(feeFactor).DivRound(truncateSig(pMaxDec.Mul(pMaxDec), 20), 39)
		w2 := pi.Sub(pim).Mul(migrationFeeFactor).DivRound(pMax.Mul(pMax), 37)

		weight := k.Mul(w1.Add(w2))

		sumFactor = sumFactor.Add(weight).RoundUp(37)
	}

	l1 := truncateSig(totalSwapAndMigrationAmount.Div(sumFactor.Truncate(36)).Floor(), 20)

	curve := make([]LiquidityDistributionParameters, 16)
	for i := range curve {
		k := decimal.NewFromFloat(param.LiquidityWeights[i])
		liq := truncateSig(convertDecimalToBN(l1.Mul(k)), 20)

		sPrice := pMax
		if i < 15 {
			sPrice = sqrtPrices[i+1]
		}

		curve[i] = LiquidityDistributionParameters{
			SqrtPrice: u128.GenUint128FromString(sPrice.String()),
			Liquidity: u128.GenUint128FromString(liq.String()),
		}
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
	migrationQuoteAmount = dmath.Rsh(migrationQuoteAmount, 128)

	mqThreshold := getMigrationQuoteThresholdFromMigrationQuoteAmount(
		migrationQuoteAmount,
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
