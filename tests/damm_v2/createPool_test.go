package damm_v2

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	jsoniter "github.com/json-iterator/go"
	dammv2 "github.com/krazyTry/meteora-go/damm_v2"
	"github.com/krazyTry/meteora-go/damm_v2/helpers"
	"github.com/shopspring/decimal"
)

func TestCreatePool(t *testing.T) {
	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	mintWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("mint address:", mintWallet.PublicKey())

	ctx := context.Background()
	TokenMintto(t, ctx, rpcClient, wsClient, mintWallet, ownerWallet, partner)
	fmt.Println("mintto success")

	cpAmm := dammv2.NewCpAmm(rpcClient, rpc.CommitmentFinalized)

	baseAmount := big.NewInt(1_000_000)
	quoteAmount := big.NewInt(1) // SOL

	positionFirstNftWallet := solana.NewWallet()

	// positionSecondNftWallet := solana.NewWallet()

	inputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, mintWallet.PublicKey())
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	outputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, solana.WrappedSol)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	initialPoolTokenBaseAmount := helpers.GetInitialPoolTokenAmount(baseAmount, inputTokenInfo.Decimals)
	initialPoolTokenQuoteAmount := helpers.GetInitialPoolTokenAmount(quoteAmount, outputTokenInfo.Decimals)

	// 1 base token = 1 quote token
	initSqrtPrice := helpers.GetSqrtPriceFromPrice(decimal.NewFromFloat(1), inputTokenInfo.Decimals, outputTokenInfo.Decimals)

	tokenBaseProgram := inputTokenInfo.Owner
	tokenQuoteProgram := outputTokenInfo.Owner

	config := dammv2.DeriveConfigAddress(0)

	configState, err := cpAmm.FetchConfigState(ctx, config)
	if err != nil {
		t.Fatal("dammv2.FetchConfigState() fail", err)
	}

	liquidityDelta := cpAmm.GetLiquidityDelta(dammv2.LiquidityDeltaParams{
		MaxAmountTokenA: initialPoolTokenBaseAmount,
		MaxAmountTokenB: initialPoolTokenQuoteAmount,
		SqrtPrice:       initSqrtPrice,
		SqrtMinPrice:    configState.SqrtMinPrice.BigInt(),
		SqrtMaxPrice:    configState.SqrtMaxPrice.BigInt(),
	})

	txBuilder, _, _, _, err := cpAmm.CreatePool(
		ctx,
		dammv2.CreatePoolParams{
			Payer:           owner,
			Creator:         owner,
			PositionNft:     positionFirstNftWallet.PublicKey(),
			TokenAMint:      mintWallet.PublicKey(),
			TokenBMint:      solana.WrappedSol,
			TokenAAmount:    baseAmount,
			TokenBAmount:    quoteAmount,
			Config:          config,
			InitSqrtPrice:   initSqrtPrice,
			LiquidityDelta:  liquidityDelta,
			TokenAProgram:   tokenBaseProgram,
			TokenBProgram:   tokenQuoteProgram,
			ActivationPoint: nil,
			IsLockLiquidity: true,
		},
	)

	tx, err := txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("CreatePool txBuilder.Build() fail", err)
	}
	sig, err := SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		case key.Equals(positionFirstNftWallet.PublicKey()):
			return &positionFirstNftWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("CreatePool SendTransaction() fail", err)
	}
	fmt.Println("create pool success sig:", sig.String())

	pools, err := cpAmm.FetchPoolStatesByTokenAMint(ctx, mintWallet.PublicKey())
	if err != nil {
		t.Fatal("cpamm.GetPoolByBaseMint() fail", err)
	}
	if len(pools) == 0 {
		fmt.Println("pool does not exist:", mintWallet.PublicKey())
		return
	}
	for idx, pool := range pools {
		fmt.Println("===========================")
		fmt.Println("print pool info")
		fmt.Println("dammv2 Index", idx)
		fmt.Println("dammv2.PoolAddress", pool.PublicKey)
		fmt.Println("dammv2.TokenAMint", pool.Account.TokenAMint)
		fmt.Println("dammv2.TokenBMint", pool.Account.TokenBMint)
		fmt.Println("dammv2.SqrtMinPrice", pool.Account.SqrtMinPrice)
		fmt.Println("dammv2.SqrtMaxPrice", pool.Account.SqrtMaxPrice)
		fmt.Println("dammv2.SqrtPrice", pool.Account.SqrtPrice)
		fmt.Println("dammv2.Liquidity", pool.Account.Liquidity)
		j, _ := jsoniter.MarshalToString(pool)
		fmt.Println("dammv2.Json", j)
		fmt.Println("===========================")
	}
}
