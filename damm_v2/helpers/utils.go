package helpers

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math/big"
	"reflect"

	binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

func GetUnClaimLpFee(poolState *dammv2gen.Pool, positionState *dammv2gen.Position) (feeTokenA, feeTokenB *big.Int, rewards []*big.Int, err error) {
	totalPositionLiquidity := new(big.Int).Add(positionState.UnlockedLiquidity.BigInt(), positionState.VestedLiquidity.BigInt())
	totalPositionLiquidity.Add(totalPositionLiquidity, positionState.PermanentLockedLiquidity.BigInt())

	feeAPerTokenStored := new(big.Int).Sub(leBytesToBigInt(poolState.FeeAPerLiquidity[:]), leBytesToBigInt(positionState.FeeAPerTokenCheckpoint[:]))
	feeBPerTokenStored := new(big.Int).Sub(leBytesToBigInt(poolState.FeeBPerLiquidity[:]), leBytesToBigInt(positionState.FeeBPerTokenCheckpoint[:]))

	feeA := new(big.Int).Mul(totalPositionLiquidity, feeAPerTokenStored)
	feeA.Rsh(feeA, LiquidityScale)
	feeB := new(big.Int).Mul(totalPositionLiquidity, feeBPerTokenStored)
	feeB.Rsh(feeB, LiquidityScale)

	feeTokenA = new(big.Int).Add(big.NewInt(int64(positionState.FeeAPending)), feeA)
	feeTokenB = new(big.Int).Add(big.NewInt(int64(positionState.FeeBPending)), feeB)

	rewards = make([]*big.Int, 0, len(positionState.RewardInfos))
	for _, item := range positionState.RewardInfos {
		rewards = append(rewards, big.NewInt(int64(item.RewardPendings)))
	}
	return feeTokenA, feeTokenB, rewards, nil
}

func GetRewardInfo(poolState dammv2gen.Pool, rewardIndex int, periodTime, currentTime *big.Int) (rewardPerPeriod, rewardBalance, totalRewardDistributed *big.Int, err error) {
	if rewardIndex < 0 || rewardIndex >= len(poolState.RewardInfos) {
		return nil, nil, nil, errors.New("invalid reward index")
	}
	poolReward := poolState.RewardInfos[rewardIndex]
	poolLiquidity := poolState.Liquidity.BigInt()

	rewardPerTokenStore := getRewardPerTokenStore(poolReward, poolLiquidity, currentTime)

	totalRewardDistributed = new(big.Int).Mul(rewardPerTokenStore, poolLiquidity)
	totalRewardDistributed.Rsh(totalRewardDistributed, 192)

	rewardDurationEnd := big.NewInt(int64(poolReward.RewardDurationEnd))
	if rewardDurationEnd.Cmp(currentTime) <= 0 {
		return big.NewInt(0), big.NewInt(0), totalRewardDistributed, nil
	}

	rewardPerPeriod = getRewardPerPeriod(poolReward, currentTime, periodTime)
	remainTime := new(big.Int).Sub(rewardDurationEnd, currentTime)
	rewardBalance = new(big.Int).Mul(poolReward.RewardRate.BigInt(), remainTime)
	rewardBalance.Rsh(rewardBalance, 64)

	if poolLiquidity.Sign() == 0 {
		return rewardPerPeriod, rewardBalance, big.NewInt(0), nil
	}

	return rewardPerPeriod.Rsh(rewardPerPeriod, 64), rewardBalance, totalRewardDistributed, nil
}

func GetUserRewardPending(poolState dammv2gen.Pool, positionState dammv2gen.Position, rewardIndex int, currentTime, periodTime *big.Int) (userRewardPerPeriod, userPendingReward *big.Int, err error) {
	poolLiquidity := poolState.Liquidity.BigInt()
	if poolLiquidity.Sign() == 0 {
		return big.NewInt(0), big.NewInt(0), nil
	}
	if rewardIndex < 0 || rewardIndex >= len(poolState.RewardInfos) {
		return nil, nil, errors.New("invalid reward index")
	}
	poolReward := poolState.RewardInfos[rewardIndex]
	userRewardInfo := positionState.RewardInfos[rewardIndex]

	rewardPerTokenStore := getRewardPerTokenStore(poolReward, poolLiquidity, currentTime)

	totalPositionLiquidity := new(big.Int).Add(positionState.UnlockedLiquidity.BigInt(), positionState.VestedLiquidity.BigInt())
	totalPositionLiquidity.Add(totalPositionLiquidity, positionState.PermanentLockedLiquidity.BigInt())

	userRewardPerTokenCheckPoint := leBytesToBigInt(userRewardInfo.RewardPerTokenCheckpoint[:])
	newReward := new(big.Int).Sub(rewardPerTokenStore, userRewardPerTokenCheckPoint)
	newReward.Mul(newReward, totalPositionLiquidity)
	newReward.Rsh(newReward, 192)

	rewardDurationEnd := big.NewInt(int64(poolReward.RewardDurationEnd))
	pending := new(big.Int).Add(big.NewInt(int64(userRewardInfo.RewardPendings)), newReward)
	if rewardDurationEnd.Cmp(currentTime) <= 0 {
		return big.NewInt(0), pending, nil
	}

	rewardPerPeriod := getRewardPerPeriod(poolReward, currentTime, periodTime)
	rewardPerTokenStorePerPeriod := new(big.Int).Lsh(rewardPerPeriod, 128)
	rewardPerTokenStorePerPeriod.Div(rewardPerTokenStorePerPeriod, poolLiquidity)
	userRewardPerPeriod = new(big.Int).Mul(totalPositionLiquidity, rewardPerTokenStorePerPeriod)
	userRewardPerPeriod.Rsh(userRewardPerPeriod, 192)

	return userRewardPerPeriod, pending, nil
}

func getRewardPerTokenStore(poolReward dammv2gen.RewardInfo, poolLiquidity *big.Int, currentTime *big.Int) *big.Int {
	if poolLiquidity.Sign() == 0 {
		return big.NewInt(0)
	}
	lastTimeRewardApplicable := minBigInt(currentTime, big.NewInt(int64(poolReward.RewardDurationEnd)))
	timePeriod := new(big.Int).Sub(lastTimeRewardApplicable, big.NewInt(int64(poolReward.LastUpdateTime)))
	currentTotalReward := new(big.Int).Mul(timePeriod, poolReward.RewardRate.BigInt())
	rewardPerTokenStore := new(big.Int).Lsh(currentTotalReward, 128)
	rewardPerTokenStore.Div(rewardPerTokenStore, poolLiquidity)

	totalRewardPerTokenStore := new(big.Int).Add(leBytesToBigInt(poolReward.RewardPerTokenStored[:]), rewardPerTokenStore)
	return totalRewardPerTokenStore
}

func getRewardPerPeriod(poolReward dammv2gen.RewardInfo, currentTime, periodTime *big.Int) *big.Int {
	timeRewardApplicable := new(big.Int).Add(currentTime, periodTime)
	rewardDurationEnd := big.NewInt(int64(poolReward.RewardDurationEnd))
	period := new(big.Int)
	if timeRewardApplicable.Cmp(rewardDurationEnd) <= 0 {
		period.Set(periodTime)
	} else {
		period.Sub(rewardDurationEnd, currentTime)
	}
	rewardPerPeriod := new(big.Int).Mul(poolReward.RewardRate.BigInt(), period)
	return rewardPerPeriod
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
