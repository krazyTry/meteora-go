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

func TestSwap2(t *testing.T) {

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

	mintWallet := solana.NewWallet()
	baseMint := mintWallet.PublicKey()

	ctx1 := context.Background()

	poolState, err := dbcService.GetPoolByBaseMint(ctx1, baseMint)
	if err != nil {
		t.Fatal("GetPoolByPoolAddress() fail", err)
	}
	fmt.Println("token info Pool:", poolState)

	swapResult2, err := dbcService.SwapQuote2(dynamic_bonding_curve.SwapQuote2Params{
		VirtualPool:      poolState.Account,
		Config:           configState,
		SwapBaseForQuote: false,
		// HasReferral                    bool
		// EligibleForFirstSwapWithMinFee bool
		CurrentPoint: dynamic_bonding_curve.CurrentPointForActivation(ctx1, rpcClient, rpc.CommitmentFinalized, dynamic_bonding_curve.ActivationType(configState.ActivationType)),
		SlippageBps:  10000,
		SwapMode:     dynamic_bonding_curve.SwapModeExactIn,
		AmountIn:     big.NewInt(0.1 * 1e9),
		// AmountOut                      *big.Int
	})
	if err != nil {
		t.Fatal("SwapQuote2() fail", err)
	}

	swap2Params := dynamic_bonding_curve.Swap2Params{
		Owner:            owner,
		Pool:             poolState.Pubkey,
		SwapBaseForQuote: false,
		// ReferralTokenAccount *solanago.PublicKey
		// Payer                *solanago.PublicKey
		SwapMode:         dynamic_bonding_curve.SwapModeExactIn,
		AmountIn:         big.NewInt(0.1 * 1e9),
		MinimumAmountOut: swapResult2.MinimumAmountOut,
		// AmountOut            *big.Int
		// MaximumAmountIn      *big.Int
	}

	pre2, swapIx2, post2, err := dbcService.Swap2(ctx1, swap2Params)
	if err != nil {
		t.Fatal("Swap2() fail", err)
	}

	instructions := []solana.Instruction{}
	instructions = append(pre2, swapIx2)
	instructions = append(instructions, post2...)

	sig, err := SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(ownerWallet.PublicKey()):
			return &ownerWallet.PrivateKey
		default:
			return nil
		}
	})

	if err != nil {
		t.Fatal("swap2 SendTransaction() fail", err)
	}
	fmt.Println("swap2 token success Success sig:", sig.String())
}
