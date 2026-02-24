package damm_v2

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	dammv2 "github.com/krazyTry/meteora-go/damm_v2"
	"github.com/krazyTry/meteora-go/damm_v2/helpers"
)

func TestSwap(t *testing.T) {

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	baseMint := solana.MustPublicKeyFromBase58("")
	ctx := context.Background()

	cpAmm := dammv2.NewCpAmm(rpcClient, rpc.CommitmentFinalized)

	pools, err := cpAmm.FetchPoolStatesByTokenAMint(ctx, baseMint)
	if err != nil {
		t.Fatal("cpAmm.FetchPoolStatesByTokenAMint() fail", err)
	}

	pool := pools[0]

	balance, err := MintBalance(ctx, rpcClient, owner, baseMint)
	if err != nil {
		t.Fatal("MintBalance() fail", err)
	}

	currentPoint := dammv2.CurrentPointForActivation(ctx, rpcClient, rpc.CommitmentFinalized, dammv2.ActivationType(pool.Account.ActivationType))

	inputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, pool.Account.TokenAMint)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	outputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, pool.Account.TokenBMint)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	quote, err := cpAmm.GetQuote(dammv2.GetQuoteParams{
		InAmount:        new(big.Int).SetUint64(balance / 3),
		InputTokenMint:  pool.Account.TokenAMint,
		Slippage:        5000,
		PoolState:       pool.Account,
		CurrentPoint:    currentPoint,
		InputTokenInfo:  inputTokenInfo,  // Optional
		OutputTokenInfo: outputTokenInfo, // Optional
		TokenADecimal:   9,
		TokenBDecimal:   9,
		// HasReferral     bool
	})
	if err != nil {
		t.Fatal("cpAmm.GetQuote() fail", err)
	}

	txBuilder, err := cpAmm.Swap(ctx, dammv2.SwapParams{
		Payer:            owner,
		Pool:             pool.PublicKey,
		PoolState:        pool.Account,
		InputTokenMint:   pool.Account.TokenAMint,
		OutputTokenMint:  pool.Account.TokenBMint,
		AmountIn:         new(big.Int).SetUint64(balance / 3),
		MinimumAmountOut: quote.MinSwapOutAmount,
		// ReferralTokenAccount *solanago.PublicKey
		// Receiver             *solanago.PublicKey
	})
	if err != nil {
		t.Fatal("cpAmm.Swap() fail", err)
	}
	tx, err := txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("Swap txBuilder.Build() fail", err)
	}
	sig, err := SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("Swap SendTransaction() fail", err)
	}
	fmt.Println("swap success Success sig:", sig.String())
}
