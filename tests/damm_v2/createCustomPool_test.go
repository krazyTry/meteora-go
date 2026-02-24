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
	"github.com/krazyTry/meteora-go/damm_v2/shared"
	"github.com/krazyTry/meteora-go/gen/damm_v2"
	"github.com/shopspring/decimal"
)

func TestCreateCustomPool(t *testing.T) {
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

	liquidityDelta := cpAmm.GetLiquidityDelta(dammv2.LiquidityDeltaParams{
		MaxAmountTokenA: initialPoolTokenBaseAmount,
		MaxAmountTokenB: initialPoolTokenQuoteAmount,
		SqrtPrice:       initSqrtPrice,
		SqrtMinPrice:    shared.MinSqrtPrice,
		SqrtMaxPrice:    shared.MaxSqrtPrice,
	})

	baseFeeParam, err := helpers.GetFeeTimeSchedulerParams(
		5000, // 50%
		25,   // 0.25%
		shared.BaseFeeModeFeeTimeSchedulerExponential,
		60,   // 60 peridos
		3600, // 60 * 60
	)
	if err != nil {
		t.Fatal("helpers.GetFeeTimeSchedulerParams() fail", err)
	}

	var dynamicFeeParam *damm_v2.DynamicFeeParameters
	dynamicFeeParam, err = helpers.GetDynamicFeeParams(25, shared.MaxPriceChangeBpsDefault)
	if err != nil {
		t.Fatal("helpers.GetDynamicFeeParams() fail", err)
	}

	poolFees := dammv2.PoolFeesParams{
		BaseFee:    baseFeeParam,
		DynamicFee: dynamicFeeParam,
		Padding:    []uint8{},
	}

	txBuilder, _, _, _, err := cpAmm.CreateCustomPool(ctx, dammv2.InitializeCustomizeablePoolParams{
		Payer:           owner,
		Creator:         owner,
		PositionNft:     positionFirstNftWallet.PublicKey(),
		TokenAMint:      mintWallet.PublicKey(),
		TokenBMint:      solana.WrappedSol,
		TokenAAmount:    baseAmount,
		TokenBAmount:    quoteAmount,
		InitSqrtPrice:   initSqrtPrice,
		LiquidityDelta:  liquidityDelta,
		TokenAProgram:   tokenBaseProgram,
		TokenBProgram:   tokenQuoteProgram,
		ActivationPoint: nil,
		IsLockLiquidity: false,
		SqrtMinPrice:    shared.MinSqrtPrice,
		SqrtMaxPrice:    shared.MaxSqrtPrice,
		PoolFees:        poolFees,
		HasAlphaVault:   false,
		ActivationType:  dammv2.ActivationTypeTimestamp,
		CollectFeeMode:  dammv2.CollectFeeModeBothToken,
	})
	tx, err := txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("CreateCustomPool txBuilder.Build() fail", err)
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
		t.Fatal("CreateCustomPool SendTransaction() fail", err)
	}
	fmt.Println("create custom pool success sig:", sig.String())

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
