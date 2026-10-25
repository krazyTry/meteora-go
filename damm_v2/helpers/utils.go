package helpers

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math/big"
	"reflect"

	binary "github.com/gagliardetto/binary"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
	"github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/rpc"
)

func GetUnClaimLpFee(poolState *dammv2gen.Pool, positionState *dammv2gen.Position, currentTime ...*big.Int) (feeTokenA, feeTokenB *big.Int, rewards []*big.Int, err error) {
	totalPositionLiquidity := positionLiquidity(positionState)
	feeAPerTokenStored := wrappingSubU256(leBytesToBigInt(poolState.FeeAPerLiquidity[:]), leBytesToBigInt(positionState.FeeAPerTokenCheckpoint[:]))
	feeBPerTokenStored := wrappingSubU256(leBytesToBigInt(poolState.FeeBPerLiquidity[:]), leBytesToBigInt(positionState.FeeBPerTokenCheckpoint[:]))
	feeA := calculatePositionFeeOrReward(totalPositionLiquidity, feeAPerTokenStored, shared.LiquidityScale)
	feeB := calculatePositionFeeOrReward(totalPositionLiquidity, feeBPerTokenStored, shared.LiquidityScale)
	feeTokenA = saturatingAddU64(new(big.Int).SetUint64(positionState.FeeAPending), feeA)
	feeTokenB = saturatingAddU64(new(big.Int).SetUint64(positionState.FeeBPending), feeB)

	var point *big.Int
	if len(currentTime) > 0 {
		point = currentTime[0]
	}
	rewards = make([]*big.Int, 0, len(positionState.RewardInfos))
	for rewardIndex := range positionState.RewardInfos {
		reward, err := quotePositionReward(poolState, positionState, rewardIndex, point)
		if err != nil {
			return nil, nil, nil, err
		}
		rewards = append(rewards, reward)
	}
	return feeTokenA, feeTokenB, rewards, nil
}

func GetRewardInfo(poolState *dammv2gen.Pool, rewardIndex int, periodTime, currentTime *big.Int) (rewardPerPeriod, rewardBalance, totalRewardDistributed *big.Int, err error) {
	if rewardIndex < 0 || rewardIndex >= len(poolState.RewardInfos) {
		return nil, nil, nil, errors.New("invalid reward index")
	}
	poolReward := poolState.RewardInfos[rewardIndex]
	poolLiquidity := poolState.Liquidity.BigInt()
	rewardPerTokenStore, err := getRewardPerTokenStore(poolReward, poolLiquidity, currentTime)
	if err != nil {
		return nil, nil, nil, err
	}
	totalRewardDistributed = new(big.Int).Mul(rewardPerTokenStore, poolLiquidity)
	totalRewardDistributed.Rsh(totalRewardDistributed, shared.TotalRewardScale)

	rewardDurationEnd := new(big.Int).SetUint64(poolReward.RewardDurationEnd)
	if rewardDurationEnd.Cmp(currentTime) <= 0 {
		return big.NewInt(0), big.NewInt(0), totalRewardDistributed, nil
	}
	rewardPerPeriod, err = getRewardPerPeriod(poolReward, currentTime, periodTime)
	if err != nil {
		return nil, nil, nil, err
	}
	remainTime := new(big.Int).Sub(rewardDurationEnd, currentTime)
	rewardBalance = new(big.Int).Mul(poolReward.RewardRate.BigInt(), remainTime)
	if rewardBalance.Cmp(shared.U128Max) > 0 {
		return nil, nil, nil, errors.New("math overflow")
	}
	return new(big.Int).Rsh(rewardPerPeriod, 64), new(big.Int).Rsh(rewardBalance, 64), totalRewardDistributed, nil
}

func GetUserRewardPending(poolState *dammv2gen.Pool, positionState *dammv2gen.Position, rewardIndex int, currentTime, periodTime *big.Int) (userRewardPerPeriod, userPendingReward *big.Int, err error) {
	if rewardIndex < 0 || rewardIndex >= len(poolState.RewardInfos) {
		return nil, nil, errors.New("invalid reward index")
	}
	poolReward := poolState.RewardInfos[rewardIndex]
	userRewardInfo := positionState.RewardInfos[rewardIndex]
	if poolReward.Initialized == 0 {
		return big.NewInt(0), new(big.Int).SetUint64(userRewardInfo.RewardPendings), nil
	}
	poolLiquidity := poolState.Liquidity.BigInt()
	rewardPerTokenStore, err := getRewardPerTokenStore(poolReward, poolLiquidity, currentTime)
	if err != nil {
		return nil, nil, err
	}
	userPendingReward, err = pendingPositionReward(positionState, rewardIndex, rewardPerTokenStore)
	if err != nil {
		return nil, nil, err
	}
	if poolLiquidity.Sign() == 0 || new(big.Int).SetUint64(poolReward.RewardDurationEnd).Cmp(currentTime) <= 0 {
		return big.NewInt(0), userPendingReward, nil
	}
	rewardPerPeriod, err := getRewardPerPeriod(poolReward, currentTime, periodTime)
	if err != nil {
		return nil, nil, err
	}
	rewardPerTokenStorePerPeriod := new(big.Int).Lsh(rewardPerPeriod, shared.LiquidityScale)
	rewardPerTokenStorePerPeriod.Div(rewardPerTokenStorePerPeriod, poolLiquidity)
	userRewardPerPeriod = calculatePositionFeeOrReward(positionLiquidity(positionState), rewardPerTokenStorePerPeriod, shared.TotalRewardScale)
	return userRewardPerPeriod, userPendingReward, nil
}

func GetDeadLiquidityReward(poolState *dammv2gen.Pool, rewardIndex int, currentTime *big.Int) (*big.Int, error) {
	if rewardIndex < 0 || rewardIndex >= len(poolState.RewardInfos) {
		return nil, errors.New("invalid reward index")
	}
	if shared.CollectFeeMode(poolState.CollectFeeMode) != shared.CollectFeeModeCompounding {
		return big.NewInt(0), nil
	}
	poolReward := poolState.RewardInfos[rewardIndex]
	if poolReward.Initialized == 0 {
		return big.NewInt(0), nil
	}
	rewardPerTokenStored, err := getRewardPerTokenStore(poolReward, poolState.Liquidity.BigInt(), currentTime)
	if err != nil {
		return nil, err
	}
	checkpoint := mulShr256WrappingU64(shared.DeadLiquidity, rewardPerTokenStored, shared.TotalRewardScale)
	return wrappingSubU64(checkpoint, new(big.Int).SetUint64(poolReward.DeadLiquidityRewardCheckpoint)), nil
}

func positionLiquidity(positionState *dammv2gen.Position) *big.Int {
	total := new(big.Int).Add(positionState.UnlockedLiquidity.BigInt(), positionState.VestedLiquidity.BigInt())
	return total.Add(total, positionState.PermanentLockedLiquidity.BigInt())
}

func pendingPositionReward(positionState *dammv2gen.Position, rewardIndex int, rewardPerTokenStored *big.Int) (*big.Int, error) {
	userRewardInfo := positionState.RewardInfos[rewardIndex]
	accrued := calculatePositionFeeOrReward(
		positionLiquidity(positionState),
		wrappingSubU256(rewardPerTokenStored, leBytesToBigInt(userRewardInfo.RewardPerTokenCheckpoint[:])),
		shared.TotalRewardScale,
	)
	return saturatingAddU64(new(big.Int).SetUint64(userRewardInfo.RewardPendings), accrued), nil
}

func quotePositionReward(poolState *dammv2gen.Pool, positionState *dammv2gen.Position, rewardIndex int, currentTime *big.Int) (*big.Int, error) {
	userRewardInfo := positionState.RewardInfos[rewardIndex]
	poolReward := poolState.RewardInfos[rewardIndex]
	if poolReward.Initialized == 0 {
		return new(big.Int).SetUint64(userRewardInfo.RewardPendings), nil
	}
	var rewardPerTokenStored *big.Int
	var err error
	if currentTime != nil {
		rewardPerTokenStored, err = getRewardPerTokenStore(poolReward, poolState.Liquidity.BigInt(), currentTime)
		if err != nil {
			return nil, err
		}
	} else {
		rewardPerTokenStored = leBytesToBigInt(poolReward.RewardPerTokenStored[:])
	}
	return pendingPositionReward(positionState, rewardIndex, rewardPerTokenStored)
}

func elapsedRewardSeconds(poolReward dammv2gen.RewardInfo, currentTime *big.Int) (*big.Int, error) {
	lastTime := minBigInt(currentTime, new(big.Int).SetUint64(poolReward.RewardDurationEnd))
	timePeriod := new(big.Int).Sub(lastTime, new(big.Int).SetUint64(poolReward.LastUpdateTime))
	if timePeriod.Sign() < 0 {
		return nil, errors.New("math overflow")
	}
	return timePeriod, nil
}

func rewardPerTokenDelta(poolReward dammv2gen.RewardInfo, poolLiquidity, currentTime *big.Int) (*big.Int, error) {
	timePeriod, err := elapsedRewardSeconds(poolReward, currentTime)
	if err != nil {
		return nil, err
	}
	if poolLiquidity.Sign() == 0 {
		emptySeconds := new(big.Int).Add(new(big.Int).SetUint64(poolReward.CumulativeSecondsWithEmptyLiquidityReward), timePeriod)
		if emptySeconds.Cmp(shared.U64Max) > 0 {
			return nil, errors.New("math overflow")
		}
		return big.NewInt(0), nil
	}
	currentTotalReward := new(big.Int).Mul(timePeriod, poolReward.RewardRate.BigInt())
	if currentTotalReward.Cmp(shared.U128Max) > 0 {
		return nil, errors.New("math overflow")
	}
	delta := new(big.Int).Lsh(currentTotalReward, shared.LiquidityScale)
	return delta.Div(delta, poolLiquidity), nil
}

func getRewardPerTokenStore(poolReward dammv2gen.RewardInfo, poolLiquidity, currentTime *big.Int) (*big.Int, error) {
	stored := leBytesToBigInt(poolReward.RewardPerTokenStored[:])
	if poolReward.Initialized == 0 {
		return stored, nil
	}
	delta, err := rewardPerTokenDelta(poolReward, poolLiquidity, currentTime)
	if err != nil {
		return nil, err
	}
	return wrappingAddU256(stored, delta), nil
}

func getRewardPerPeriod(poolReward dammv2gen.RewardInfo, currentTime, periodTime *big.Int) (*big.Int, error) {
	timeRewardApplicable := new(big.Int).Add(currentTime, periodTime)
	rewardDurationEnd := new(big.Int).SetUint64(poolReward.RewardDurationEnd)
	period := new(big.Int)
	if timeRewardApplicable.Cmp(rewardDurationEnd) <= 0 {
		period.Set(periodTime)
	} else {
		period.Sub(rewardDurationEnd, currentTime)
	}
	rewardPerPeriod := new(big.Int).Mul(poolReward.RewardRate.BigInt(), period)
	if rewardPerPeriod.Cmp(shared.U128Max) > 0 {
		return nil, errors.New("math overflow")
	}
	return rewardPerPeriod, nil
}

func wrappingAddU256(value, delta *big.Int) *big.Int {
	return new(big.Int).Mod(new(big.Int).Add(value, delta), shared.U256Modulus)
}

func wrappingSubU256(value, checkpoint *big.Int) *big.Int {
	return new(big.Int).Mod(new(big.Int).Sub(value, checkpoint), shared.U256Modulus)
}

func wrappingSubU64(value, checkpoint *big.Int) *big.Int {
	return new(big.Int).Mod(new(big.Int).Sub(value, checkpoint), shared.U64Modulus)
}

func saturatingAddU64(pending, amount *big.Int) *big.Int {
	sum := new(big.Int).Add(pending, amount)
	if sum.Cmp(shared.U64Max) > 0 {
		return new(big.Int).Set(shared.U64Max)
	}
	return sum
}

func calculatePositionFeeOrReward(liquidity, tokenPerLiquidityDelta *big.Int, offset uint) *big.Int {
	shifted := new(big.Int).Mul(liquidity, tokenPerLiquidityDelta)
	shifted.Rsh(shifted, offset)
	if shifted.Cmp(shared.U64Max) > 0 {
		return new(big.Int).Set(shared.U64Max)
	}
	return shifted
}

func mulShr256WrappingU64(x, y *big.Int, offset uint) *big.Int {
	prod := new(big.Int).Mul(x, y)
	prod.Rsh(prod, offset)
	return prod.Mod(prod, shared.U64Modulus)
}

func leBytesToBigInt(b []uint8) *big.Int {
	out := new(big.Int)
	for i := len(b) - 1; i >= 0; i-- {
		out.Lsh(out, 8)
		out.Add(out, big.NewInt(int64(b[i])))
	}
	return out
}

func minBigInt(a, b *big.Int) *big.Int {
	if a.Cmp(b) <= 0 {
		return new(big.Int).Set(a)
	}
	return new(big.Int).Set(b)
}

// Filter represents a filter for querying accounts by owner and offset
type Filter struct {
	Owner  solana.PublicKey // Account owner to filter by
	Offset uint64           // Offset for pagination
}

func discriminator(name string) []byte {
	hash := sha256.Sum256([]byte("account:" + name))
	var out [8]byte
	copy(out[:], hash[:8])
	return out[:]
}

// ComputeStructOffset gets the offset position of an object in a struct
func ComputeStructOffset(x any, o string) uint64 {
	t := reflect.TypeOf(x).Elem()
	fields := make([]reflect.StructField, 0)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == o {
			break
		}
		fields = append(fields, f)
	}

	newType := reflect.StructOf(fields)
	newValue := reflect.New(newType).Elem()

	buf__ := new(bytes.Buffer)
	enc__ := binary.NewBorshEncoder(buf__)
	enc__.Encode(newValue.Interface())

	// instruction discriminators offset = 8
	return uint64(buf__.Len()) + 8
}

func CreateProgramAccountFilter(key string, filter *Filter) []rpc.RPCFilter {
	var filters []rpc.RPCFilter
	filters = append(filters, rpc.RPCFilter{
		Memcmp: &rpc.RPCFilterMemcmp{
			Offset: 0,
			Bytes:  discriminator(key),
		},
	})

	if filter != nil {
		filters = append(filters, rpc.RPCFilter{
			Memcmp: &rpc.RPCFilterMemcmp{
				Offset: filter.Offset,
				Bytes:  filter.Owner[:],
			},
		})
	}

	return filters
}
