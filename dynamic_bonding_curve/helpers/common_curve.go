package helpers

import (
	"encoding/binary"
	"errors"
	"math/big"

	bin "github.com/gagliardetto/binary"
	"github.com/shopspring/decimal"
)

func GetBaseTokenForSwap(sqrtStartPrice, sqrtMigrationPrice *big.Int, curve []LiquidityDistributionParameters) (*big.Int, error) {
	total := big.NewInt(0)
	for i := 0; i < len(curve); i++ {
		lower := sqrtStartPrice
		if i > 0 {
			lower = curve[i-1].SqrtPrice.BigInt()
		}
		if curve[i].SqrtPrice.BigInt().Cmp(sqrtMigrationPrice) > 0 {
			delta, err := GetDeltaAmountBaseUnsigned(lower, sqrtMigrationPrice, curve[i].Liquidity.BigInt(), RoundingUp)
			if err != nil {
				return nil, err
			}
			total.Add(total, delta)
			break
		}
		delta, err := GetDeltaAmountBaseUnsigned(lower, curve[i].SqrtPrice.BigInt(), curve[i].Liquidity.BigInt(), RoundingUp)
		if err != nil {
			return nil, err
		}
		total.Add(total, delta)
	}
	return total, nil
}

func GetMigrationQuoteAmountFromMigrationQuoteThreshold(migrationQuoteThreshold decimal.Decimal, migrationFeePercent uint8) decimal.Decimal {
	return migrationQuoteThreshold.Mul(decimal.NewFromInt(100).Sub(decimal.NewFromInt(int64(migrationFeePercent)))).Div(decimal.NewFromInt(100))
}

func GetMigrationQuoteThresholdFromMigrationQuoteAmount(migrationQuoteAmount decimal.Decimal, migrationFeePercent decimal.Decimal) decimal.Decimal {
	return migrationQuoteAmount.Mul(decimal.NewFromInt(100)).Div(decimal.NewFromInt(100).Sub(migrationFeePercent))
}

func GetMigrationBaseToken(migrationQuoteAmount, sqrtMigrationPrice *big.Int, migrationOption MigrationOption) (*big.Int, error) {
	if migrationOption == MigrationOptionMetDamm {
		price := new(big.Int).Mul(sqrtMigrationPrice, sqrtMigrationPrice)
		quote := new(big.Int).Lsh(migrationQuoteAmount, 128)
		div, mod := new(big.Int).QuoRem(quote, price, new(big.Int))
		if mod.Sign() != 0 {
			div.Add(div, big.NewInt(1))
		}
		return div, nil
	}
	if migrationOption == MigrationOptionMetDammV2 {
		liquidity, err := GetInitialLiquidityFromDeltaQuote(migrationQuoteAmount, MinSqrtPrice, sqrtMigrationPrice)
		if err != nil {
			return nil, err
		}
		baseAmount, err := GetDeltaAmountBaseUnsigned(sqrtMigrationPrice, MaxSqrtPrice, liquidity, RoundingUp)
		if err != nil {
			return nil, err
		}
		return baseAmount, nil
	}
	return nil, errors.New("Invalid migration option")
}

func GetMigrationThresholdPrice(migrationThreshold, sqrtStartPrice *big.Int, curve []LiquidityDistributionParameters) (*big.Int, error) {
	if len(curve) == 0 {
		return nil, errors.New("Curve is empty")
	}
	nextSqrtPrice := new(big.Int).Set(sqrtStartPrice)
	totalAmount, err := GetDeltaAmountQuoteUnsigned(nextSqrtPrice, curve[0].SqrtPrice.BigInt(), curve[0].Liquidity.BigInt(), RoundingUp)
	if err != nil {
		return nil, err
	}
	if totalAmount.Cmp(migrationThreshold) > 0 {
		return GetNextSqrtPriceFromInput(nextSqrtPrice, curve[0].Liquidity.BigInt(), migrationThreshold, false)
	}
	amountLeft := new(big.Int).Sub(migrationThreshold, totalAmount)
	nextSqrtPrice = curve[0].SqrtPrice.BigInt()
	for i := 1; i < len(curve); i++ {
		maxAmount, err := GetDeltaAmountQuoteUnsigned(nextSqrtPrice, curve[i].SqrtPrice.BigInt(), curve[i].Liquidity.BigInt(), RoundingUp)
		if err != nil {
			return nil, err
		}
		if maxAmount.Cmp(amountLeft) > 0 {
			nextSqrtPrice, err = GetNextSqrtPriceFromInput(nextSqrtPrice, curve[i].Liquidity.BigInt(), amountLeft, false)
			if err != nil {
				return nil, err
			}
			amountLeft = big.NewInt(0)
			break
		}
		amountLeft.Sub(amountLeft, maxAmount)
		nextSqrtPrice = curve[i].SqrtPrice.BigInt()
	}
	if amountLeft.Sign() != 0 {
		return nil, errors.New("Not enough liquidity")
	}
	return nextSqrtPrice, nil
}

func GetSwapAmountWithBuffer(swapBaseAmount, sqrtStartPrice *big.Int, curve []LiquidityDistributionParameters) (*big.Int, error) {
	swapAmountBuffer := new(big.Int).Add(swapBaseAmount, new(big.Int).Div(new(big.Int).Mul(swapBaseAmount, big.NewInt(SwapBufferPercentage)), big.NewInt(100)))
	maxBaseAmountOnCurve, err := GetBaseTokenForSwap(sqrtStartPrice, MaxSqrtPrice, curve)
	if err != nil {
		return nil, err
	}
	if swapAmountBuffer.Cmp(maxBaseAmountOnCurve) > 0 {
		return maxBaseAmountOnCurve, nil
	}
	return swapAmountBuffer, nil
}

func GetVestingLockedLiquidityBpsAtNSeconds(vestingInfo *LiquidityVestingInfoParameters, nSeconds uint64) uint64 {
	if vestingInfo == nil || vestingInfo.VestingPercentage == 0 {
		return 0
	}
	totalLiquidity := U128Max
	totalVestedLiquidity := new(big.Int).Div(new(big.Int).Mul(totalLiquidity, big.NewInt(int64(vestingInfo.VestingPercentage))), big.NewInt(100))
	bpsPerPeriod := vestingInfo.BpsPerPeriod
	numberOfPeriods := vestingInfo.NumberOfPeriods
	frequency := vestingInfo.Frequency
	cliffDuration := vestingInfo.CliffDurationFromMigrationTime

	totalBpsAfterCliff := bpsPerPeriod * numberOfPeriods
	totalVestingLiquidityAfterCliff := new(big.Int).Div(new(big.Int).Mul(totalVestedLiquidity, big.NewInt(int64(totalBpsAfterCliff))), big.NewInt(MaxBasisPoint))

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
	liquidityLockedBps := new(big.Int).Div(new(big.Int).Mul(locked, big.NewInt(MaxBasisPoint)), totalLiquidity)
	return liquidityLockedBps.Uint64()
}

func CalculateLockedLiquidityBpsAtTime(partnerPermanentLockedLiquidityPercentage, creatorPermanentLockedLiquidityPercentage uint8, partnerLiquidityVestingInfo, creatorLiquidityVestingInfo *LiquidityVestingInfoParameters, elapsedSeconds uint64) uint64 {
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
