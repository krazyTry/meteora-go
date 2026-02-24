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

func TestClaimPartnerFee(t *testing.T) {

	dbcService := dynamic_bonding_curve.NewDynamicBondingCurve(rpcClient, rpc.CommitmentFinalized)

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	leftover := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("leftover address:", leftover.PublicKey())

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

	if poolState.Account.PartnerQuoteFee > 0 {
		claimParams := dynamic_bonding_curve.ClaimTradingFeeParams{
			Pool:       poolState.Pubkey,
			FeeClaimer: partner.PublicKey(),
			Payer:      partner.PublicKey(),
			// MaxBaseAmount  *big.Int
			MaxQuoteAmount: new(big.Int).SetUint64(poolState.Account.PartnerQuoteFee),
			// Receiver       *solanago.PublicKey
			// TempWSolAcc    *solanago.PublicKey
		}
		pre, claimIx, post, err := dbcService.ClaimPartnerTradingFee(ctx1, claimParams)
		if err != nil {
			t.Fatal("ClaimPartnerTradingFee() fail", err)
		}
		instructions := []solana.Instruction{}
		instructions = append(pre, claimIx)
		instructions = append(instructions, post...)

		sig, err := SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(ownerWallet.PublicKey()):
				return &ownerWallet.PrivateKey
			case key.Equals(partner.PublicKey()):
				return &partner.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("claim SendTransaction() fail", err)
		}

		fmt.Println("claim token success Success sig:", sig.String())
	}
}
