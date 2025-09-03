package meteora

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/tidwall/gjson"
)

func testInit() (*rpc.Client, *ws.Client, *context.Context, *context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(context.Background())

	wsClient, err := ws.Connect(ctx, rpc.DevNet_WS)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	rpcClient := rpc.New(rpc.DevNet_RPC)

	return rpcClient, wsClient, &ctx, &cancel, nil
}

func testBalance(ctx context.Context, rpcClient *rpc.Client, wallet solana.PublicKey) (uint64, error) {
	ctx1, _ := context.WithTimeout(ctx, time.Second*5)
	balanceResult, err := rpcClient.GetBalance(ctx1, wallet, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}
	lamports := balanceResult.Value
	sol := float64(lamports) / 1e9 // 1 SOL = 1e9 lamports

	fmt.Printf("wallet address:%v \t sol holdings:%v \n", wallet, sol)
	return lamports, nil
}

func testMintBalance(ctx context.Context, rpcClient *rpc.Client, wallet, baseMint solana.PublicKey) (uint64, error) {
	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*5)
	defer cancel1()
	resp, err := rpcClient.GetTokenAccountsByOwner(ctx1, wallet, &rpc.GetTokenAccountsConfig{
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
	mintBalance := make(map[string]uint64)
	for _, v := range resp.Value {
		mitm := gjson.GetBytes(v.Account.Data.GetRawJSON(), "parsed.info.mint").String()
		amount := gjson.GetBytes(v.Account.Data.GetRawJSON(), "parsed.info.tokenAmount.amount").Uint()
		if amount == 0 || mitm == "" {
			continue
		}
		mintBalance[mitm] = amount
	}

	fmt.Printf("trader address:%v \t mint:%v \t holdings:%v \n", wallet, baseMint, mintBalance[baseMint.String()])
	return mintBalance[baseMint.String()], nil
}

func testTransferSOL(ctx context.Context,
	rpcClient *rpc.Client,
	wsClient *ws.Client,
	from *solana.Wallet,
	to solana.PublicKey,
	amountIn uint64,
) (string, error) {

	if amountIn < 5000 {
		return "", fmt.Errorf("amountIn < 5000")
	}

	amountIn -= 5000
	transferix := system.NewTransferInstruction(
		amountIn,
		from.PublicKey(),
		to,
	).Build()

	blockhash, err := solanago.GetLatestBlockhash(ctx, rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction([]solana.Instruction{transferix}, blockhash, solana.TransactionPayer(from.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(from.PublicKey()):
			return &from.PrivateKey
		default:
			return nil
		}
	}); err != nil {
		return "", err
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
		return "", err
	}

	if _, err = sendandconfirmtransaction.WaitForConfirmation(ctx, wsClient, sig, nil); err != nil {
		return "", err
	}
	return sig.String(), nil
}
