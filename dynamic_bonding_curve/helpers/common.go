package helpers

import (
	"context"
	"errors"
	"math/big"
	"time"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func GetFirstKey(key1, key2 solanago.PublicKey) []byte {
	buf1 := key1.Bytes()
	buf2 := key2.Bytes()
	if bytesCompare(buf1, buf2) == 1 {
		return buf1
	}
	return buf2
}

func GetSecondKey(key1, key2 solanago.PublicKey) []byte {
	buf1 := key1.Bytes()
	buf2 := key2.Bytes()
	if bytesCompare(buf1, buf2) == 1 {
		return buf2
	}
	return buf1
}

func bytesCompare(a, b []byte) int {
	min := len(a)
	if len(b) < min {
		min = len(b)
	}
	for i := 0; i < min; i++ {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	if len(a) > len(b) {
		return 1
	}
	if len(a) < len(b) {
		return -1
	}
	return 0
}

// GetAccountCreationTimestamp returns creation time via first signature lookup.
func GetAccountCreationTimestamp(ctx context.Context, client *rpc.Client, account solanago.PublicKey) (*time.Time, error) {
	limit := 1
	sigs, err := client.GetSignaturesForAddressWithOpts(ctx, account, &rpc.GetSignaturesForAddressOpts{Limit: &limit})
	if err != nil {
		return nil, err
	}
	if len(sigs) == 0 || sigs[0].BlockTime == nil {
		return nil, nil
	}
	t := time.Unix(int64(*sigs[0].BlockTime), 0)
	return &t, nil
}

func GetAccountCreationTimestamps(ctx context.Context, client *rpc.Client, accounts []solanago.PublicKey) ([]*time.Time, error) {
	out := make([]*time.Time, 0, len(accounts))
	for _, acc := range accounts {
		t, err := GetAccountCreationTimestamp(ctx, client, acc)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// GetTotalTokenSupply computes total supply and checks overflow (u64).
func GetTotalTokenSupply(swapBaseAmount, migrationBaseThreshold *big.Int, lockedVestingParams struct {
	AmountPerPeriod   *big.Int
	NumberOfPeriod    *big.Int
	CliffUnlockAmount *big.Int
}) (*big.Int, error) {
	if swapBaseAmount == nil || migrationBaseThreshold == nil || lockedVestingParams.AmountPerPeriod == nil || lockedVestingParams.NumberOfPeriod == nil || lockedVestingParams.CliffUnlockAmount == nil {
		return nil, errors.New("nil input")
	}
	totalCirculating := new(big.Int).Add(swapBaseAmount, migrationBaseThreshold)
	locked := new(big.Int).Mul(lockedVestingParams.AmountPerPeriod, lockedVestingParams.NumberOfPeriod)
	locked.Add(locked, lockedVestingParams.CliffUnlockAmount)
	total := new(big.Int).Add(totalCirculating, locked)
	if total.Sign() < 0 || total.BitLen() > 64 {
		return nil, errors.New("math overflow")
	}
	return total, nil
}

func CreateSqrtPrices(prices []string, tokenADecimal, tokenBDecimal TokenDecimal) ([]*big.Int, error) {
	list := make([]*big.Int, len(prices))
	for k := range prices {
		price, err := GetSqrtPriceFromPrice(prices[k], int(tokenADecimal), int(tokenBDecimal))
		if err != nil {
			return nil, err
		}
		list[k] = price
	}
	return list, nil
}
