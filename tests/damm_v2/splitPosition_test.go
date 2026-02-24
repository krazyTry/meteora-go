package damm_v2

import (
	"context"
	"fmt"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	dammv2 "github.com/krazyTry/meteora-go/damm_v2"
)

func TestSplitPosition(t *testing.T) {

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
	poolAddress := pools[0].PublicKey

	positions, err := cpAmm.GetPositionsByUser(ctx, owner)
	if err != nil {
		t.Fatal("cpAmm.GetPositionsByUser() fail", err)
	}

	var positionNftFirstAccount, positionNftFirst solana.PublicKey
	for _, position := range positions {
		if !position.PositionState.Pool.Equals(poolAddress) {
			continue
		}
		positionNftFirst = position.Position
		positionNftFirstAccount = position.PositionNftAccount
		break
	}

	positionSecondNftWallet := solana.NewWallet()

	txBuilder, positionNftSecond, _, err := cpAmm.CreatePosition(ctx, dammv2.CreatePositionParams{
		Owner:       owner,
		Payer:       owner,
		Pool:        poolAddress,
		PositionNft: positionSecondNftWallet.PublicKey(),
	})
	if err != nil {
		t.Fatal("cpAmm.CreatePosition() fail", err)
	}
	tx, err := txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("CreatePosition txBuilder.Build() fail", err)
	}
	sig, err := SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		case key.Equals(positionSecondNftWallet.PublicKey()):
			return &positionSecondNftWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("CreatePosition SendTransaction() fail", err)
	}
	fmt.Println("create position success sig:", sig.String())

	txBuilder, err = cpAmm.SplitPosition(ctx, dammv2.SplitPositionParams{
		FirstPositionOwner:                 owner,
		SecondPositionOwner:                owner,
		Pool:                               poolAddress,
		FirstPosition:                      positionNftFirst,
		FirstPositionNftAccount:            positionNftFirstAccount,
		SecondPosition:                     positionNftSecond,
		SecondPositionNftAccount:           dammv2.DerivePositionNftAccount(positionSecondNftWallet.PublicKey()),
		PermanentLockedLiquidityPercentage: 0,
		UnlockedLiquidityPercentage:        50,
		FeeAPercentage:                     50,
		FeeBPercentage:                     50,
		Reward0Percentage:                  50,
		Reward1Percentage:                  50,
		// InnerVestingLiquidityPercentage    uint8
	})
	if err != nil {
		t.Fatal("cpAmm.SplitPosition() fail", err)
	}

	tx, err = txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("SplitPosition txBuilder.Build() fail", err)
	}
	sig, err = SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("SplitPosition SendTransaction() fail", err)
	}
	fmt.Println("split position success sig:", sig.String())
}
