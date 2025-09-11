package solana

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	"reflect"

	binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/tidwall/gjson"
)

func GetRentExempt(ctx context.Context, rpcClient *rpc.Client) (uint64, error) {
	lamports, err := rpcClient.GetMinimumBalanceForRentExemption(
		ctx,
		165, // SPL Token account 固定大小为 165 bytes
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return 0, err
	}
	return lamports, nil
}

func SOLBalance(ctx context.Context, rpcClient *rpc.Client, wallet solana.PublicKey) (uint64, error) {
	balanceResult, err := rpcClient.GetBalance(ctx, wallet, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}
	return balanceResult.Value, nil
}

func MintBalance(ctx context.Context, rpcClient *rpc.Client, wallet, baseMint solana.PublicKey) (uint64, error) {
	resp, err := rpcClient.GetTokenAccountsByOwner(ctx, wallet, &rpc.GetTokenAccountsConfig{
		ProgramId: &solana.TokenProgramID,
	}, &rpc.GetTokenAccountsOpts{
		Encoding:   solana.EncodingJSONParsed,
		Commitment: rpc.CommitmentFinalized,
	})
	if err != nil {
		return 0, err
	}
	/*
		{
			"parsed": {
				"info": {
					"isNative": false,
					"mint": "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
					"owner": "5HfLhj117ucm2FoqjfcSeZMf91CuJbzxZ9BeRRpZWN6m",
					"state": "initialized",
					"tokenAmount": {
						"amount": "0",
						"decimals": 6,
						"uiAmount": 0.0,
						"uiAmountString": "0"
					}
				},
				"type": "account"
			},
			"program": "spl-token",
			"space": 165
		}
	*/
	for _, v := range resp.Value {
		mint := gjson.GetBytes(v.Account.Data.GetRawJSON(), "parsed.info.mint").String()
		if mint != baseMint.String() {
			continue
		}
		amount := gjson.GetBytes(v.Account.Data.GetRawJSON(), "parsed.info.tokenAmount.amount").Uint()
		return amount, nil
	}
	return 0, nil
}

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

func discriminator(name string) []byte {
	hash := sha256.Sum256([]byte("account:" + name))
	var out [8]byte
	copy(out[:], hash[:8])
	return out[:]
}

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

func GenProgramAccountFilter(key string, filter *Filter) *rpc.GetProgramAccountsOpts {
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
	if filter == nil {
		return opt
	}

	opt.Filters = append(opt.Filters, rpc.RPCFilter{
		Memcmp: &rpc.RPCFilterMemcmp{
			Offset: filter.Offset,
			Bytes:  filter.Owner[:],
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

func GetLatestBlockhash(ctx context.Context, rpcClient *rpc.Client) (solana.Hash, error) {

	recent, err := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.Hash{}, err
	}
	return recent.Value.Blockhash, nil
}

func SendTransaction(
	ctx context.Context,
	rpcClient *rpc.Client,
	wsClient *ws.Client,
	instructions []solana.Instruction,
	payer solana.PublicKey,
	sign func(key solana.PublicKey) *solana.PrivateKey,
) (solana.Signature, error) {

	latestBlockhash, err := GetLatestBlockhash(ctx, rpcClient)
	if err != nil {
		return solana.Signature{}, err
	}

	tx, err := solana.NewTransaction(instructions, latestBlockhash, solana.TransactionPayer(payer))
	if err != nil {
		return solana.Signature{}, err
	}

	if _, err = tx.Sign(sign); err != nil {
		return solana.Signature{}, err
	}

	if IsSimulate {
		if _, err = rpcClient.SimulateTransactionWithOpts(
			ctx,
			tx,
			&rpc.SimulateTransactionOpts{
				SigVerify:  false,
				Commitment: rpc.CommitmentFinalized,
			}); err != nil {
			return solana.Signature{}, err
		}
		return solana.Signature{}, nil
	}

	sig, err := rpcClient.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return solana.Signature{}, err
	}

	confirmed, err := sendandconfirmtransaction.WaitForConfirmation(ctx, wsClient, sig, nil)
	if confirmed {
		if err != nil {
			return solana.Signature{}, fmt.Errorf("transaction confirmed but failed: %w", err)
		}
		return sig, nil
	}
	statusResp, err := rpcClient.GetSignatureStatuses(ctx, true, sig)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("rpc GetSignatureStatuses error: %w", err)
	}
	status := statusResp.Value[0]
	if status == nil {
		return solana.Signature{}, fmt.Errorf("transaction not found (maybe dropped)")
	}
	if status.Err != nil {
		return solana.Signature{}, fmt.Errorf("transaction confirmed but failed: %v", status.Err)
	}
	txResp, err := rpcClient.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{Commitment: rpc.CommitmentFinalized})
	if err != nil {
		return solana.Signature{}, fmt.Errorf("rpc GetTransaction error: %w", err)
	}
	if txResp != nil && txResp.Meta != nil && txResp.Meta.Err != nil {
		return solana.Signature{}, fmt.Errorf("transaction finalized but failed: %v", txResp.Meta.Err)
	}
	return sig, nil
}
