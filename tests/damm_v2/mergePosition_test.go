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

func TestMergePosition(t *testing.T) {

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

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

	inputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, pool.Account.TokenAMint)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	outputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, pool.Account.TokenBMint)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

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

	pools, err = cpAmm.FetchPoolStatesByTokenAMint(ctx, baseMint)
	if err != nil {
		t.Fatal("cpamm.GetPoolByBaseMint() fail", err)
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

	positionBState, err := cpAmm.FetchPositionState(ctx, positionNftSecond)
	if err != nil {
		t.Fatal("FetchPositionState() fail", err)
	}
	vestings, err := cpAmm.GetAllVestingsByPosition(ctx, positionNftSecond)
	if err != nil {
		t.Fatal("GetAllVestingsByPosition() fail", err)
	}
	poolStates, err := cpAmm.GetMultiplePools(ctx, []solana.PublicKey{poolAddress})
	if err != nil {
		t.Fatal("GetMultiplePools() fail", err)
	}

	fmt.Println("UnlockedLiquidity", positionBState.UnlockedLiquidity.BigInt())

	currentPoint := dammv2.CurrentPointForActivation(ctx, rpcClient, rpc.CommitmentFinalized, dammv2.ActivationType(poolStates[0].ActivationType))

	txBuilder, err = cpAmm.MergePosition(ctx, dammv2.MergePositionParams{
		Owner:                                owner,
		PositionA:                            positionNftFirst,
		PositionB:                            positionNftSecond,
		PoolState:                            poolStates[0],
		PositionBNftAccount:                  positionNftSecondAccount,
		PositionANftAccount:                  positionNftFirstAccount,
		PositionBState:                       positionBState,
		PositionBVestings:                    vestings,
		CurrentPoint:                         currentPoint,
		TokenAAmountAddLiquidityThreshold:    shared.U64Max,
		TokenBAmountAddLiquidityThreshold:    shared.U64Max,
		TokenAAmountRemoveLiquidityThreshold: new(big.Int).SetUint64(0),
		TokenBAmountRemoveLiquidityThreshold: new(big.Int).SetUint64(0),
	})
	if err != nil {
		t.Fatal("cpAmm.MergePosition() fail", err)
	}

	tx, err = txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("MergePosition txBuilder.Build() fail", err)
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
		t.Fatal("MergePosition SendTransaction() fail", err)
	}
	fmt.Println("merge position success sig:", sig.String())
}
