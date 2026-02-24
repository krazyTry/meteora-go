package damm_v2

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	dammv2 "github.com/krazyTry/meteora-go/damm_v2"
)

func TestRemoveLiquidity(t *testing.T) {
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

	positions, err := cpAmm.GetPositionsByUser(ctx, owner)
	if err != nil {
		t.Fatal("cpAmm.GetPositionsByUser() fail", err)
	}

	var (
		positionNftFirstAccount, positionNftFirst solana.PublicKey
		positionNftFirstState                     *dammv2.PositionState
	)
	for _, position := range positions {
		if !position.PositionState.Pool.Equals(pool.PublicKey) {
			continue
		}
		positionNftFirst = position.Position
		positionNftFirstAccount = position.PositionNftAccount
		positionNftFirstState = position.PositionState
		break
	}

	txBuilder, err := cpAmm.RemoveLiquidity(ctx, dammv2.RemoveLiquidityParams{
		Owner:                 owner,
		Pool:                  pool.PublicKey,
		PoolState:             pool.Account,
		Position:              positionNftFirst,
		PositionNftAccount:    positionNftFirstAccount,
		LiquidityDelta:        positionNftFirstState.UnlockedLiquidity.BigInt(),
		TokenAAmountThreshold: big.NewInt(0),
		TokenBAmountThreshold: big.NewInt(0),
		// Vestings              []VestingWithAccount
		// CurrentPoint          *big.Int
	})
	if err != nil {
		t.Fatal("cpAmm.RemoveLiquidity() fail", err)
	}
	tx, err := txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("RemoveLiquidity txBuilder.Build() fail", err)
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
		t.Fatal("RemoveLiquidity SendTransaction() fail", err)
	}
	fmt.Println("remove liquidity success Success sig:", sig.String())

	txBuilder, err = cpAmm.ClosePosition(ctx, dammv2.ClosePositionParams{
		Owner:              owner,
		Pool:               pool.PublicKey,
		Position:           positionNftFirst,
		PositionNftAccount: positionNftFirstAccount,
		PositionNftMint:    positionNftFirstState.NftMint,
	})

	if err != nil {
		t.Fatal("cpAmm.ClosePosition() fail", err)
	}
	tx, err = txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("ClosePosition txBuilder.Build() fail", err)
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
		t.Fatal("ClosePosition SendTransaction() fail", err)
	}
	fmt.Println("close position success Success sig:", sig.String())

}
