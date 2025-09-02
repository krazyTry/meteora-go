package solana

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/rpc"
)

func CurrenPoint(ctx context.Context, rpcClient *rpc.Client, activationType uint8) (*big.Int, error) {
	var (
		currentPoint *big.Int
	)

	currentSlot, err := rpcClient.GetSlot(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("failed to get slot: %w", err)
	}

	switch activationType {
	case 1:
		var currentTime *solana.UnixTimeSeconds
		if currentTime, err = rpcClient.GetBlockTime(ctx, currentSlot); err != nil {
			return nil, fmt.Errorf("failed to get block time: %w", err)
		}
		currentPoint = big.NewInt(currentTime.Time().Unix())
	case 0:
		currentPoint = big.NewInt(int64(currentSlot))
	default:
		return nil, fmt.Errorf("activationType error")
	}

	return currentPoint, nil
}

func GetLatestBlockhash(ctx context.Context, rpcClient *rpc.Client) (solana.Hash, error) {

	recent, err := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.Hash{}, err
	}
	return recent.Value.Blockhash, nil
}

func discriminator(name string) []byte {
	hash := sha256.Sum256([]byte("account:" + name))
	var out [8]byte
	copy(out[:], hash[:8])
	return out[:]
}

func GenProgramAccountFilter(key string, owner solana.PublicKey, offset uint64) *rpc.GetProgramAccountsOpts {

	opt := &rpc.GetProgramAccountsOpts{
		Commitment: rpc.CommitmentFinalized,
		Encoding:   solana.EncodingBase64,
		Filters: []rpc.RPCFilter{
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 0,
					Bytes:  discriminator(key),
				},
			},
		},
	}
	if owner.Equals(solana.PublicKey{}) {
		return opt
	}

	opt.Filters = append(opt.Filters, rpc.RPCFilter{
		Memcmp: &rpc.RPCFilterMemcmp{
			Offset: offset,
			Bytes:  owner[:],
		},
	})
	return opt
}

func GetAccountInfo(ctx context.Context, rpcClient *rpc.Client, account solana.PublicKey) (*rpc.GetAccountInfoResult, error) {
	return rpcClient.GetAccountInfoWithOpts(ctx, account, &rpc.GetAccountInfoOpts{Commitment: rpc.CommitmentFinalized})
}

func GetMultipleAccountInfo(ctx context.Context, rpcClient *rpc.Client, accounts []solana.PublicKey) (*rpc.GetMultipleAccountsResult, error) {
	return rpcClient.GetMultipleAccountsWithOpts(ctx, accounts, &rpc.GetMultipleAccountsOpts{Commitment: rpc.CommitmentFinalized, Encoding: solana.EncodingBase64})
}

func GetCurrentEpoch(ctx context.Context, rpcClient *rpc.Client) (uint64, error) {
	epochInfo, err := rpcClient.GetEpochInfo(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}
	return epochInfo.Epoch, nil
}

func PrepareTokenATA(
	ctx context.Context,
	rpcClient *rpc.Client,
	owner solana.PublicKey,
	tokenMint solana.PublicKey,
	payer solana.PublicKey,
	instructions *[]solana.Instruction,
) (solana.PublicKey, error) {
	tokenATA, _, err := solana.FindAssociatedTokenAddress(
		owner,
		tokenMint,
	)

	if err != nil {
		return solana.PublicKey{}, err
	}

	exists, err := GetAccountInfo(ctx, rpcClient, tokenATA)
	if err != nil && err != rpc.ErrNotFound {
		return solana.PublicKey{}, err
	}

	if exists == nil {
		ix := associatedtokenaccount.NewCreateInstruction(
			payer, owner, tokenMint,
		).Build()
		*instructions = append(*instructions, ix)
	}
	return tokenATA, nil
}

func GetMultipleToken(ctx context.Context, rpcClient *rpc.Client, tokens ...solana.PublicKey) ([]*Token, error) {
	outs, err := GetMultipleAccountInfo(ctx, rpcClient, tokens)
	if err != nil {
		return nil, err
	}
	list := make([]*Token, len(outs.Value))
	for i, out := range outs.Value {
		if out == nil {
			continue
		}

		token, err := new(TokenLayout).Decode(out.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		token.Owner = out.Owner

		list[i] = token
	}
	return list, nil
}
