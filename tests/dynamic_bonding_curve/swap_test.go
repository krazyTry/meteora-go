package dynamic_bonding_curve

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve"
)

func TestSwap(t *testing.T) {

	dbcService := dynamic_bonding_curve.NewDynamicBondingCurve(rpcClient, rpc.CommitmentFinalized)

	configAddress := solana.MustPublicKeyFromBase58("")

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	leftover := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("leftover address:", leftover.PublicKey())

	configState, err := dbcService.GetPoolConfig(context.Background(), configAddress)
	if err != nil {
		t.Fatal("GetConfig() fail", err)
	}

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	baseMint := solana.MustPublicKeyFromBase58("")

	ctx1 := context.Background()

	poolState, err := dbcService.GetPoolByBaseMint(ctx1, baseMint)
	if err != nil {
		t.Fatal("GetPoolByPoolAddress() fail", err)
	}
	fmt.Println("token info Pool:", poolState)

	swapResult, err := dbcService.SwapQuote(dynamic_bonding_curve.SwapQuoteParams{
		VirtualPool:      poolState.Account,
		Config:           configState,
		SwapBaseForQuote: false,
		AmountIn:         big.NewInt(0.1 * 1e9),
		SlippageBps:      10000,
		// HasReferral                    bool
		// EligibleForFirstSwapWithMinFee bool
		CurrentPoint: dynamic_bonding_curve.CurrentPointForActivation(ctx1, rpcClient, rpc.CommitmentFinalized, dynamic_bonding_curve.ActivationType(configState.ActivationType)),
	})
	if err != nil {
		t.Fatal("SwapQuote() fail", err)
	}

	swapParams := dynamic_bonding_curve.SwapParams{
		Owner:            owner,
		Pool:             poolState.Pubkey,
		AmountIn:         big.NewInt(0.1 * 1e9),
		MinimumAmountOut: swapResult.MinimumAmountOut,
		SwapBaseForQuote: false,
		// ReferralTokenAccount *solanago.PublicKey
		// Payer                *solanago.PublicKey
	}
	pre, swapIx, post, err := dbcService.Swap(ctx1, swapParams)
	if err != nil {
		t.Fatal("Swap() fail", err)
	}

	instructions := []solana.Instruction{}
	instructions = append(pre, swapIx)
	instructions = append(instructions, post...)

	sig, err := SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(ownerWallet.PublicKey()):
			return &ownerWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("swap SendTransaction() fail", err)
	}
	fmt.Println("swap token success Success sig:", sig.String())
}
