package damm_v2

import (
	"context"
	"fmt"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	dammv2 "github.com/krazyTry/meteora-go/damm_v2"
	"github.com/krazyTry/meteora-go/damm_v2/helpers"
)

func TestClaimFee(t *testing.T) {

	baseMint := solana.MustPublicKeyFromBase58("")

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	ctx1 := context.Background()

	cpAmm := dammv2.NewCpAmm(rpcClient, rpc.CommitmentFinalized)

	pools, err := cpAmm.FetchPoolStatesByTokenAMint(ctx1, baseMint)
	if err != nil {
		t.Fatal("cpAmm.FetchPoolStatesByTokenAMint() fail", err)
	}

	for _, v := range pools {
		fmt.Println("pool:", v.PublicKey)
		fmt.Println(v.Account)
	}

	pool := pools[0]

	positions, err := cpAmm.GetPositionsByUser(ctx1, partner.PublicKey())
	if err != nil {
		t.Fatal("cpAmm.GetPositionsByUser() fail", err)
	}

	var partnerPosition dammv2.UserPosition
	for _, v := range positions {
		if !v.PositionState.Pool.Equals(pool.PublicKey) {
			continue
		}
		partnerPosition = v
		break
	}

	baseFee, quoteFee, _, err := helpers.GetUnClaimLpFee(pool.Account, partnerPosition.PositionState)
	if err != nil {
		t.Fatal("helpers.GetUnclaimedLpFee() fail", err)
	}
	fmt.Println("baseFee:", baseFee)
	fmt.Println("quoteFee:", quoteFee)

	if quoteFee.Sign() > 0 {

		txBuilder, err := cpAmm.ClaimPositionFee(ctx1, dammv2.ClaimPositionFeeParams{
			Owner:              partner.PublicKey(),
			Position:           partnerPosition.Position,
			Pool:               pool.PublicKey,
			PositionNftAccount: partnerPosition.PositionNftAccount,
			PoolState:          pool.Account,
			// Receiver           *solanago.PublicKey
			// FeePayer           *solanago.PublicKey
			// TempWSolAccount    *solanago.PublicKey
		})
		if err != nil {
			t.Fatal("cpAmm.ClaimPositionFee() fail", err)
		}
		tx, err := txBuilder.SetFeePayer(partner.PublicKey()).Build()
		if err != nil {
			t.Fatal("claim txBuilder.Build() fail", err)
		}
		sig, err := SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(partner.PublicKey()):
				return &partner.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("claim SendTransaction() fail", err)
		}
		fmt.Println("claim success Success sig:", sig.String())
	}
}
