package helpers

import (
	"encoding/binary"
	"errors"
	"math/big"

	bin "github.com/gagliardetto/binary"
	dbcmath "github.com/krazyTry/meteora-go/dynamic_bonding_curve/math"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
	"github.com/shopspring/decimal"
)

// LiveCurvePoints stops at the first zero point, matching a stored PoolConfig.curve.
func LiveCurvePoints(curve []shared.LiquidityDistributionParameters) []shared.LiquidityDistributionParameters {
	for i, point := range curve {
		if point.SqrtPrice.BigInt().Sign() == 0 || point.Liquidity.BigInt().Sign() == 0 {
			return curve[:i]
		}
	}
	return curve
}

func GetBaseTokenForSwap(sqrtStartPrice, sqrtMigrationPrice *big.Int, curve []shared.LiquidityDistributionParameters) (*big.Int, error) {
	total := big.NewInt(0)
	lower := new(big.Int).Set(sqrtStartPrice)
	for _, point := range LiveCurvePoints(curve) {
		pointSqrt := point.SqrtPrice.BigInt()
		upper := pointSqrt
		if upper.Cmp(sqrtMigrationPrice) > 0 {
			upper = sqrtMigrationPrice
		}
		delta, err := dbcmath.GetDeltaAmountBaseUnsigned256(lower, upper, point.Liquidity.BigInt(), shared.RoundingUp)
		if err != nil {
			return nil, err
		}
		total.Add(total, delta)
		if pointSqrt.Cmp(sqrtMigrationPrice) > 0 {
			break
		}
		lower = new(big.Int).Set(pointSqrt)
	}
	return total, nil
}

// MigrationQuoteAmountFromThreshold is ceil(threshold * (100 - feePercentage) / 100).
func MigrationQuoteAmountFromThreshold(migrationQuoteThreshold *big.Int, migrationFeePercent uint8) (*big.Int, error) {
	if migrationFeePercent > 100 {
		return nil, errors.New(shared.ErrInvalidInput)
	}
	return MulDiv(migrationQuoteThreshold, big.NewInt(int64(100-migrationFeePercent)), big.NewInt(100), shared.RoundingUp)
}

func GetMigrationQuoteAmountFromMigrationQuoteThreshold(migrationQuoteThreshold decimal.Decimal, migrationFeePercent uint8) decimal.Decimal {
	if migrationQuoteThreshold.Equal(migrationQuoteThreshold.Truncate(0)) {
		amount, err := MigrationQuoteAmountFromThreshold(migrationQuoteThreshold.BigInt(), migrationFeePercent)
		if err == nil {
			return decimal.NewFromBigInt(amount, 0)
		}
	}
	numerator := migrationQuoteThreshold.Mul(decimal.NewFromInt(int64(100 - migrationFeePercent)))
	return numerator.Div(decimal.NewFromInt(100)).Ceil()
}

func GetMigrationQuoteThresholdFromMigrationQuoteAmount(migrationQuoteAmount decimal.Decimal, migrationFeePercent decimal.Decimal) decimal.Decimal {
	return migrationQuoteAmount.Mul(decimal.NewFromInt(100)).Div(decimal.NewFromInt(100).Sub(migrationFeePercent))
}

// ConstantProductBaseFromQuote is the base a constant-product migration deposits for a quote amount, rounded up.
func ConstantProductBaseFromQuote(migrationQuoteAmount, sqrtMigrationPrice *big.Int) (*big.Int, error) {
	if sqrtMigrationPrice.Sign() == 0 {
		return nil, errors.New(shared.ErrMathOverflow)
	}
	price := new(big.Int).Mul(sqrtMigrationPrice, sqrtMigrationPrice)
	quote := new(big.Int).Lsh(migrationQuoteAmount, 128)
	div, mod := new(big.Int).QuoRem(quote, price, new(big.Int))
	if mod.Sign() != 0 {
		div.Add(div, big.NewInt(1))
	}
	if div.Cmp(shared.U64Max) > 0 {
		return nil, errors.New(shared.ErrMathOverflow)
	}
	return div, nil
}

func GetMigrationBaseToken(migrationQuoteAmount, sqrtMigrationPrice *big.Int, migrationOption shared.MigrationOption) (*big.Int, error) {
	return GetMigrationBaseTokenForCollectFee(migrationQuoteAmount, sqrtMigrationPrice, migrationOption, shared.MigratedCollectFeeModeQuoteToken)
}

// GetMigrationBaseTokenForCollectFee sizes the migration base. Compounding and DAMM v1 use constant product.
func GetMigrationBaseTokenForCollectFee(migrationQuoteAmount, sqrtMigrationPrice *big.Int, migrationOption shared.MigrationOption, migratedCollectFeeMode shared.MigratedCollectFeeMode) (*big.Int, error) {
	if migrationOption == shared.MigrationOptionMetDamm || migratedCollectFeeMode == shared.MigratedCollectFeeModeCompounding {
		return ConstantProductBaseFromQuote(migrationQuoteAmount, sqrtMigrationPrice)
	}
	if migrationOption == shared.MigrationOptionMetDammV2 {
		liquidity, err := GetInitialLiquidityFromDeltaQuote(migrationQuoteAmount, shared.MinSqrtPrice, sqrtMigrationPrice)
		if err != nil {
			return nil, err
		}
		return GetDeltaAmountBaseUnsigned(sqrtMigrationPrice, shared.MaxSqrtPrice, liquidity, shared.RoundingUp)
	}
	return nil, errors.New(shared.ErrInvalidMigrationOption)
}

// GetMigrationBaseWeight is the base amount per unit of migration quote, unrounded.
func GetMigrationBaseWeight(sqrtMigrationPrice *big.Int, migrationOption shared.MigrationOption, migratedCollectFeeMode shared.MigratedCollectFeeMode) (decimal.Decimal, error) {
	p := decimal.NewFromBigInt(sqrtMigrationPrice, 0)
	if migrationOption == shared.MigrationOptionMetDamm || migratedCollectFeeMode == shared.MigratedCollectFeeModeCompounding {
		return decimal.NewFromInt(1).Div(p.Mul(p)), nil
	}
	if migrationOption == shared.MigrationOptionMetDammV2 {
		maxSqrt := decimal.NewFromBigInt(shared.MaxSqrtPrice, 0)
		minSqrt := decimal.NewFromBigInt(shared.MinSqrtPrice, 0)
		return maxSqrt.Sub(p).Div(p.Sub(minSqrt).Mul(p).Mul(maxSqrt)), nil
	}
	return decimal.Zero, errors.New(shared.ErrInvalidMigrationOption)
}

// GetMigrationDepositAmounts keeps the deposit inside both budgets. Quote rounds down and base rounds up.
func GetMigrationDepositAmounts(baseBudget, quoteBudget, baseAmount, quoteAmount *big.Int) (*big.Int, *big.Int, error) {
	if baseAmount.Cmp(baseBudget) == 0 && quoteAmount.Cmp(quoteBudget) == 0 {
		return new(big.Int).Set(baseBudget), new(big.Int).Set(quoteBudget), nil
	}
	baseSideBinds := new(big.Int).Mul(baseAmount, quoteBudget).Cmp(new(big.Int).Mul(quoteAmount, baseBudget)) <= 0
	var depositBase, depositQuote *big.Int
	var err error
	if baseSideBinds {
		depositBase = new(big.Int).Set(baseAmount)
		depositQuote, err = dbcmath.MulDiv(quoteBudget, baseAmount, baseBudget, shared.RoundingDown)
	} else {
		depositBase, err = dbcmath.MulDiv(baseBudget, quoteAmount, quoteBudget, shared.RoundingUp)
		depositQuote = new(big.Int).Set(quoteAmount)
	}
	if err != nil {
		return nil, nil, err
	}
	if depositBase.Sign() == 0 || depositQuote.Sign() == 0 {
		return nil, nil, errors.New(shared.ErrAmountIsZero)
	}
	return depositBase, depositQuote, nil
}

func GetMigrationThresholdPrice(migrationThreshold, sqrtStartPrice *big.Int, curve []shared.LiquidityDistributionParameters) (*big.Int, error) {
	points := LiveCurvePoints(curve)
	if len(points) == 0 {
		return nil, errors.New("Curve is empty")
	}
	amountLeft := new(big.Int).Set(migrationThreshold)
	sqrtPrice := new(big.Int).Set(sqrtStartPrice)
	for _, point := range points {
		maxAmount, err := dbcmath.GetDeltaAmountQuoteUnsigned256(sqrtPrice, point.SqrtPrice.BigInt(), point.Liquidity.BigInt(), shared.RoundingUp)
		if err != nil {
			return nil, err
		}
		if maxAmount.Cmp(amountLeft) > 0 {
			return GetNextSqrtPriceFromInput(sqrtPrice, point.Liquidity.BigInt(), amountLeft, false)
		}
		nextLeft, err := Sub(amountLeft, maxAmount)
		if err != nil {
			return nil, err
		}
		amountLeft = nextLeft
		sqrtPrice = point.SqrtPrice.BigInt()
	}
	if amountLeft.Sign() != 0 {
		held := new(big.Int).Sub(migrationThreshold, amountLeft)
		return nil, errors.New(shared.ErrNotEnoughLiquidity + ": the curve holds " + held.String() + " quote, the threshold is " + migrationThreshold.String())
	}
	return sqrtPrice, nil
}

// CurveBreakdown is the quote a migration threshold consumes on each live segment.
type CurveBreakdown struct {
	SegmentAmounts [](*big.Int)
	FinalSqrtPrice *big.Int
	TotalAmount    *big.Int
}

// GetCurveBreakdown consumes only the remaining threshold on each segment and stops at exact completion.
func GetCurveBreakdown(migrationQuoteThreshold, sqrtStartPrice *big.Int, curve []shared.LiquidityDistributionParameters) (CurveBreakdown, error) {
	points := LiveCurvePoints(curve)
	if len(points) == 0 {
		return CurveBreakdown{}, errors.New("Curve is empty")
	}
	segmentAmounts := make([]*big.Int, 0, len(points))
	remaining := new(big.Int).Set(migrationQuoteThreshold)
	lower := new(big.Int).Set(sqrtStartPrice)
	finalSqrtPrice := new(big.Int).Set(sqrtStartPrice)
	for _, point := range points {
		if remaining.Sign() == 0 {
			segmentAmounts = append(segmentAmounts, big.NewInt(0))
			continue
		}
		upper := point.SqrtPrice.BigInt()
		maxSegment, err := dbcmath.GetDeltaAmountQuoteUnsigned256(lower, upper, point.Liquidity.BigInt(), shared.RoundingUp)
		if err != nil {
			return CurveBreakdown{}, err
		}
		if maxSegment.Cmp(remaining) > 0 {
			segmentAmounts = append(segmentAmounts, new(big.Int).Set(remaining))
			next, err := GetNextSqrtPriceFromInput(lower, point.Liquidity.BigInt(), remaining, false)
			if err != nil {
				return CurveBreakdown{}, err
			}
			finalSqrtPrice = next
			remaining = big.NewInt(0)
			lower = upper
			continue
		}
		segmentAmounts = append(segmentAmounts, maxSegment)
		remaining = new(big.Int).Sub(remaining, maxSegment)
		finalSqrtPrice = upper
		lower = new(big.Int).Set(upper)
	}
	if remaining.Sign() != 0 {
		held := new(big.Int).Sub(migrationQuoteThreshold, remaining)
		return CurveBreakdown{}, errors.New(shared.ErrNotEnoughLiquidity + ": the curve holds " + held.String() + " quote, the threshold is " + migrationQuoteThreshold.String())
	}
	return CurveBreakdown{SegmentAmounts: segmentAmounts, FinalSqrtPrice: finalSqrtPrice, TotalAmount: new(big.Int).Set(migrationQuoteThreshold)}, nil
}

func GetSwapAmountWithBuffer(swapBaseAmount, sqrtStartPrice *big.Int, curve []shared.LiquidityDistributionParameters) (*big.Int, error) {
	swapAmountBuffer := new(big.Int).Add(swapBaseAmount, new(big.Int).Div(new(big.Int).Mul(swapBaseAmount, big.NewInt(shared.SwapBufferPercentage)), big.NewInt(100)))
	maxBaseAmountOnCurve, err := GetBaseTokenForSwap(sqrtStartPrice, shared.MaxSqrtPrice, curve)
	if err != nil {
		return nil, err
	}
	swapAmountWithBuffer := swapAmountBuffer
	if swapAmountBuffer.Cmp(maxBaseAmountOnCurve) > 0 {
		swapAmountWithBuffer = maxBaseAmountOnCurve
	}
	if swapAmountWithBuffer.Cmp(shared.U64Max) > 0 {
		return nil, errors.New(shared.ErrMathOverflow)
	}
	return swapAmountWithBuffer, nil
}

func GetVestingLockedLiquidityBpsAtNSeconds(vestingInfo *shared.LiquidityVestingInfoParameters, nSeconds uint64) uint64 {
	if vestingInfo == nil || vestingInfo.VestingPercentage == 0 {
		return 0
	}
	totalLiquidity := shared.U128Max
	totalVestedLiquidity := new(big.Int).Div(new(big.Int).Mul(totalLiquidity, big.NewInt(int64(vestingInfo.VestingPercentage))), big.NewInt(100))
	bpsPerPeriod := vestingInfo.BpsPerPeriod
	numberOfPeriods := vestingInfo.NumberOfPeriods
	frequency := vestingInfo.Frequency
	cliffDuration := vestingInfo.CliffDurationFromMigrationTime

	totalBpsAfterCliff := bpsPerPeriod * numberOfPeriods
	totalVestingLiquidityAfterCliff := new(big.Int).Div(new(big.Int).Mul(totalVestedLiquidity, big.NewInt(int64(totalBpsAfterCliff))), big.NewInt(shared.MaxBasisPoint))

	liquidityPerPeriod := big.NewInt(0)
	adjustedFrequency := frequency
	adjustedNumberOfPeriods := numberOfPeriods
	adjustedCliffDuration := cliffDuration
	if numberOfPeriods > 0 {
		liquidityPerPeriod = new(big.Int).Div(totalVestingLiquidityAfterCliff, big.NewInt(int64(numberOfPeriods)))
	}
	if liquidityPerPeriod.Sign() == 0 {
		adjustedNumberOfPeriods = 0
		adjustedFrequency = 0
		if adjustedCliffDuration == 0 {
			adjustedCliffDuration = 1
		}
	}

	cliffUnlockLiquidity := new(big.Int).Sub(totalVestedLiquidity, new(big.Int).Mul(liquidityPerPeriod, big.NewInt(int64(adjustedNumberOfPeriods))))
	cliffPoint := big.NewInt(int64(adjustedCliffDuration))
	currentPoint := big.NewInt(int64(nSeconds))

	unlocked := big.NewInt(0)
	if currentPoint.Cmp(cliffPoint) >= 0 {
		unlocked = new(big.Int).Set(cliffUnlockLiquidity)
		if adjustedFrequency > 0 && adjustedNumberOfPeriods > 0 {
			timeAfterCliff := new(big.Int).Sub(currentPoint, cliffPoint)
			periodsElapsed := new(big.Int).Div(timeAfterCliff, big.NewInt(int64(adjustedFrequency))).Uint64()
			if periodsElapsed > uint64(adjustedNumberOfPeriods) {
				periodsElapsed = uint64(adjustedNumberOfPeriods)
			}
			unlocked.Add(unlocked, new(big.Int).Mul(liquidityPerPeriod, big.NewInt(int64(periodsElapsed))))
		}
	}
	locked := new(big.Int).Sub(totalVestedLiquidity, unlocked)
	liquidityLockedBps := new(big.Int).Div(new(big.Int).Mul(locked, big.NewInt(shared.MaxBasisPoint)), totalLiquidity)
	return liquidityLockedBps.Uint64()
}

func CalculateLockedLiquidityBpsAtTime(partnerPermanentLockedLiquidityPercentage, creatorPermanentLockedLiquidityPercentage uint8, partnerLiquidityVestingInfo, creatorLiquidityVestingInfo *shared.LiquidityVestingInfoParameters, elapsedSeconds uint64) uint64 {
	partnerVested := GetVestingLockedLiquidityBpsAtNSeconds(partnerLiquidityVestingInfo, elapsedSeconds)
	creatorVested := GetVestingLockedLiquidityBpsAtNSeconds(creatorLiquidityVestingInfo, elapsedSeconds)
	partnerPermanent := uint64(partnerPermanentLockedLiquidityPercentage) * 100
	creatorPermanent := uint64(creatorPermanentLockedLiquidityPercentage) * 100
	return partnerVested + partnerPermanent + creatorVested + creatorPermanent
}

func BigToU128(v *big.Int) bin.Uint128 {
	// u := bin.NewUint128LittleEndian()
	u := bin.Uint128{
		Endianness: binary.LittleEndian,
	}
	bytes := v.Bytes()
	if len(bytes) < 16 {
		pad := make([]byte, 16-len(bytes))
		bytes = append(pad, bytes...)
	}
	hi := new(big.Int).SetBytes(bytes[:8]).Uint64()
	lo := new(big.Int).SetBytes(bytes[8:]).Uint64()
	u.Hi = hi
	u.Lo = lo
	return u
}
