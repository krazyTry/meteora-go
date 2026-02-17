package helpers

import (
	"math/big"

	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

func IsVestingComplete(vestingData *dammv2gen.Vesting, currentPoint *big.Int) bool {
	cliffPoint := big.NewInt(int64(vestingData.InnerVesting.CliffPoint))
	periodFrequency := big.NewInt(int64(vestingData.InnerVesting.PeriodFrequency))
	numberOfPeriods := vestingData.InnerVesting.NumberOfPeriod

	endPoint := new(big.Int).Add(cliffPoint, new(big.Int).Mul(periodFrequency, big.NewInt(int64(numberOfPeriods))))

	return currentPoint.Cmp(endPoint) >= 0
}

func GetTotalLockedLiquidity(vestingData *dammv2gen.Vesting) *big.Int {
	cliffUnlockLiquidity := vestingData.InnerVesting.CliffUnlockLiquidity.BigInt()
	liquidityPerPeriod := vestingData.InnerVesting.LiquidityPerPeriod.BigInt()
	return new(big.Int).Add(cliffUnlockLiquidity, new(big.Int).Mul(liquidityPerPeriod, big.NewInt(int64(vestingData.InnerVesting.NumberOfPeriod))))
}

func GetAvailableVestingLiquidity(vestingData *dammv2gen.Vesting, currentPoint *big.Int) *big.Int {
	cliffPoint := big.NewInt(int64(vestingData.InnerVesting.CliffPoint))
	periodFrequency := big.NewInt(int64(vestingData.InnerVesting.PeriodFrequency))
	cliffUnlockLiquidity := vestingData.InnerVesting.CliffUnlockLiquidity.BigInt()
	liquidityPerPeriod := vestingData.InnerVesting.LiquidityPerPeriod.BigInt()
	numberOfPeriod := vestingData.InnerVesting.NumberOfPeriod
	totalReleasedLiquidity := vestingData.InnerVesting.TotalReleasedLiquidity.BigInt()

	if currentPoint.Cmp(cliffPoint) < 0 {
		return big.NewInt(0)
	}
	if periodFrequency.Sign() == 0 {
		return new(big.Int).Set(cliffUnlockLiquidity)
	}

	passedPeriod := new(big.Int).Sub(currentPoint, cliffPoint)
	passedPeriod.Div(passedPeriod, periodFrequency)

	maxPeriods := big.NewInt(int64(numberOfPeriod))
	if passedPeriod.Cmp(maxPeriods) > 0 {
		passedPeriod = maxPeriods
	}

	unlockedLiquidity := new(big.Int).Add(cliffUnlockLiquidity, new(big.Int).Mul(passedPeriod, liquidityPerPeriod))
	available := new(big.Int).Sub(unlockedLiquidity, totalReleasedLiquidity)
	return available
}
