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
	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

func TestAddLiquidity(t *testing.T) {

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
	if len(pools) == 0 {
		fmt.Println("pool does not exist:", baseMint)
		return
	}
	pool := pools[0]
	poolAddress := pool.PublicKey

	positionSecondNftWallet := solana.NewWallet()

	txBuilder, positionNftSecond, positionNftSecondAccount, err := cpAmm.CreatePosition(ctx, dammv2.CreatePositionParams{
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

	inputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, pool.Account.TokenAMint)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	outputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, pool.Account.TokenBMint)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	inAmount := new(big.Int).SetUint64(0.1 * 1e9)
	depositQuote := cpAmm.GetDepositQuote(dammv2.GetDepositQuoteParams{
		InAmount:        inAmount,
		IsTokenA:        true,
		MinSqrtPrice:    pools[0].Account.SqrtMinPrice.BigInt(),
		MaxSqrtPrice:    pools[0].Account.SqrtMaxPrice.BigInt(),
		SqrtPrice:       pools[0].Account.SqrtPrice.BigInt(),
		InputTokenInfo:  inputTokenInfo,
		OutputTokenInfo: outputTokenInfo,
	})

	txBuilder, err = cpAmm.AddLiquidity(ctx, dammv2.AddLiquidityParams{
		Owner:                 owner,
		Pool:                  pools[0].PublicKey,
		PoolState:             pools[0].Account,
		Position:              positionNftSecond,
		PositionNftAccount:    positionNftSecondAccount,
		LiquidityDelta:        depositQuote.LiquidityDelta,
		MaxAmountTokenA:       inAmount,
		MaxAmountTokenB:       inAmount,
		TokenAAmountThreshold: shared.U64Max,
		TokenBAmountThreshold: shared.U64Max,
	})

	if err != nil {
		t.Fatal("cpAmm.AddLiquidity() fail", err)
	}
	tx, err = txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("AddLiquidity txBuilder.Build() fail", err)
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
		t.Fatal("AddLiquidity SendTransaction() fail", err)
	}
	fmt.Println("add liquidity success Success sig:", sig.String())
}
