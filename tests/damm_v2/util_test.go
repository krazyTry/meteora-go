package damm_v2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/tidwall/gjson"
)

var (
	wsClient, _ = ws.Connect(context.Background(), rpc.DevNet_WS)
	rpcClient   = rpc.New(rpc.DevNet_RPC)

	// wsClient, _ = ws.Connect(context.Background(), rpc.MainNetBeta_WS)
	// rpcClient   = rpc.New(rpc.MainNetBeta_RPC)
)

func Balance(ctx context.Context, rpcClient *rpc.Client, wallet solana.PublicKey) (uint64, error) {
	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*5)
	defer cancel1()
	balanceResult, err := rpcClient.GetBalance(ctx1, wallet, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}
	lamports := balanceResult.Value
	sol := float64(lamports) / 1e9 // 1 SOL = 1e9 lamports

	fmt.Printf("wallet address:%v \t sol holdings:%v \n", wallet, sol)
	return lamports, nil
}

func MintBalance(ctx context.Context, rpcClient *rpc.Client, wallet, baseMint solana.PublicKey) (uint64, error) {
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

func TransferSOL(ctx context.Context,
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

	sig, err := SendInstruction(ctx, rpcClient, wsClient, []solana.Instruction{transferix}, from.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(from.PublicKey()):
			return &from.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func Transfer(ctx context.Context, rpcClient *rpc.Client, wsClient *ws.Client) (string, error) {
	payerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	userWallet := solana.NewWallet()

	baseMint := solana.MustPublicKeyFromBase58("")

	fmt.Println("payerWallet:", payerWallet.PublicKey(), payerWallet.PrivateKey)
	fmt.Println("ownerWallet:", ownerWallet.PublicKey(), ownerWallet.PrivateKey)
	fmt.Println("userWallet:", userWallet.PublicKey(), userWallet.PrivateKey)

	// 给payer 转手续费
	transferIx := system.NewTransferInstruction(
		1*1e9,
		ownerWallet.PublicKey(),
		payerWallet.PublicKey(),
	).Build()

	instructions := []solana.Instruction{transferIx}
	// var instructions []solana.Instruction

	sendTokenAccount, _, _ := solana.FindAssociatedTokenAddress(ownerWallet.PublicKey(), baseMint)

	receiveTokenAccount, _ := PrepareTokenATAWithFirst(ctx, rpcClient, userWallet.PublicKey(), baseMint, payerWallet.PublicKey(), &instructions)

	transferIx1 := token.NewTransferCheckedInstruction(
		1*1e9,
		9,
		sendTokenAccount,
		baseMint,
		receiveTokenAccount,
		ownerWallet.PublicKey(),
		nil,
	).Build()

	instructions = append(instructions, transferIx1)

	sig, err := SendInstruction(ctx, rpcClient, wsClient, instructions, payerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(payerWallet.PublicKey()):
			return &payerWallet.PrivateKey
		case key.Equals(ownerWallet.PublicKey()):
			return &ownerWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		return "", err
	}

	return sig.String(), nil
}

func GetLatestBlockhash(ctx context.Context, rpcClient *rpc.Client) (solana.Hash, error) {
	recent, err := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.Hash{}, err
	}
	return recent.Value.Blockhash, nil
}

func PrepareTokenATAWithFirst(
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

	ix := associatedtokenaccount.NewCreateInstruction(
		payer, owner, tokenMint,
	).Build()
	*instructions = append(*instructions, ix)

	return tokenATA, nil
}

func SendInstruction(
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

func SendTransaction(
	ctx context.Context,
	rpcClient *rpc.Client,
	wsClient *ws.Client,
	tx *solana.Transaction,
	sign func(key solana.PublicKey) *solana.PrivateKey,
) (solana.Signature, error) {

	latestBlockhash, err := GetLatestBlockhash(ctx, rpcClient)
	if err != nil {
		return solana.Signature{}, err
	}

	// tx, err := solana.NewTransaction(instructions, latestBlockhash, solana.TransactionPayer(payer))
	// if err != nil {
	// 	return solana.Signature{}, err
	// }

	tx.Message.RecentBlockhash = latestBlockhash

	if _, err = tx.Sign(sign); err != nil {
		return solana.Signature{}, err
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

func TokenMintto(t *testing.T, ctx context.Context, rpcClient *rpc.Client, wsClient *ws.Client, mint, payer, partner *solana.Wallet) {
	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	defer cancel1()

	mintAmount := uint64(10_00_000_000 * 1e9)

	lamports, err := rpcClient.GetMinimumBalanceForRentExemption(
		ctx,
		uint64(token.MINT_SIZE),
		rpc.CommitmentFinalized,
	)
	if err != nil {
		t.Fatal("rpcClient.GetMinimumBalanceForRentExemption fail", err)
	}

	createIx := system.NewCreateAccountInstruction(
		lamports,
		token.MINT_SIZE,
		solana.TokenProgramID,
		payer.PublicKey(),
		mint.PublicKey(),
	).Build()

	initializeIx := token.NewInitializeMint2InstructionBuilder().
		SetDecimals(9).
		SetMintAuthority(payer.PublicKey()).
		SetMintAccount(mint.PublicKey()).Build()

	ata, _, _ := solana.FindAssociatedTokenAddress(payer.PublicKey(), mint.PublicKey())

	ix := associatedtokenaccount.NewCreateInstruction(
		payer.PublicKey(), payer.PublicKey(), mint.PublicKey(),
	).Build()

	mintIx := token.NewMintToInstruction(
		mintAmount, // 数量 (1000 token, decimals=9)
		mint.PublicKey(),
		ata,
		payer.PublicKey(),
		nil,
	).Build()

	ata1, _, _ := solana.FindAssociatedTokenAddress(partner.PublicKey(), mint.PublicKey())

	ix1 := associatedtokenaccount.NewCreateInstruction(
		payer.PublicKey(), partner.PublicKey(), mint.PublicKey(),
	).Build()

	mintTx1 := token.NewMintToInstruction(
		10_000_000*1e9,
		mint.PublicKey(),
		ata1,
		payer.PublicKey(),
		nil,
	).Build()
	ixs := []solana.Instruction{createIx, initializeIx, ix, ix1, mintIx, mintTx1}

	sig, err := SendInstruction(ctx1, rpcClient, wsClient, ixs, payer.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(payer.PublicKey()):
			return &payer.PrivateKey
		case key.Equals(mint.PublicKey()):
			return &mint.PrivateKey
		// case key.Equals(partner.PublicKey()):
		// 	return &partner.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("SendInstruction fail", err)
	}
	fmt.Println("TokenMintto success", sig.String())
}
