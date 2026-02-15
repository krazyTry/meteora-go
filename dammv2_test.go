package meteora

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	dammv2 "github.com/krazyTry/meteora-go/damm_v2"
)

func TestDAMMv2(t *testing.T) {
	return
	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	mintWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("mint address:", mintWallet.PublicKey())

	TokenMintto(t, context.Background(), rpcClient, wsClient, mintWallet, ownerWallet, partner)
}

func TestClaimDAMMv2(t *testing.T) {
	return

	baseMint := solana.MustPublicKeyFromBase58("")

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	// payer := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	// fmt.Println("payer address:", payer.PublicKey())

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	leftover := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("leftover address:", leftover.PublicKey())

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

	// positions, err := cpAmm.GetPositionsByUser(ctx1, owner)
	// if err != nil {
	// 	t.Fatal("cpAmm.GetPositionsByUser() fail", err)
	// }
	// var userPosition dammv2.UserPosition
	// for _, v := range positions {
	// 	if !v.PositionState.Pool.Equals(pool.PublicKey) {
	// 		continue
	// 	}
	// 	userPosition = v
	// 	break
	// }
	// fmt.Println("userPosition", userPosition)

	{
		// var (
		// 	txBuilder dammv2.TxBuilder
		// 	tx        *solana.Transaction
		// 	sig       solana.Signature
		// )
		// position, positionNftAccount := userPosition.Position, userPosition.PositionNftAccount
		// positionNft := userPosition.PositionState.NftMint

		positionNftWallet := solana.NewWallet()
		positionNft := positionNftWallet.PublicKey()
		txBuilder, position, positionNftAccount, err := cpAmm.CreatePosition(ctx1, dammv2.CreatePositionParams{
			Owner:       owner,
			Payer:       owner,
			Pool:        pool.PublicKey,
			PositionNft: positionNft,
		})
		if err != nil {
			t.Fatal("cpAmm.CreatePosition() fail", err)
		}
		tx, err := txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("CreatePosition txBuilder.Build() fail", err)
		}
		sig, err := SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			case key.Equals(positionNft):
				return &positionNftWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("CreatePosition SendTransaction() fail", err)
		}
		fmt.Println("create position success sig:", sig.String())

		balance, err := MintBalance(ctx1, rpcClient, owner, baseMint)
		if err != nil {
			t.Fatal("MintBalance() fail", err)
		}

		inputTokenInfo, err := dammv2.GetTokenInfo(ctx1, rpcClient, pool.Account.TokenAMint)
		if err != nil {
			t.Fatal("dammv2.GetTokenInfo() fail", err)
		}

		outputTokenInfo, err := dammv2.GetTokenInfo(ctx1, rpcClient, pool.Account.TokenBMint)
		if err != nil {
			t.Fatal("dammv2.GetTokenInfo() fail", err)
		}

		inAmount := new(big.Int).SetUint64(0.1 * 1e9)
		depositQuote := cpAmm.GetDepositQuote(dammv2.GetDepositQuoteParams{
			InAmount:        inAmount,
			IsTokenA:        true,
			MinSqrtPrice:    pool.Account.SqrtMinPrice.BigInt(),
			MaxSqrtPrice:    pool.Account.SqrtMaxPrice.BigInt(),
			SqrtPrice:       pool.Account.SqrtPrice.BigInt(),
			InputTokenInfo:  inputTokenInfo,
			OutputTokenInfo: outputTokenInfo,
		})

		txBuilder, err = cpAmm.AddLiquidity(ctx1, dammv2.AddLiquidityParams{
			Owner:                 owner,
			Pool:                  pool.PublicKey,
			PoolState:             pool.Account,
			Position:              position,
			PositionNftAccount:    positionNftAccount,
			LiquidityDelta:        depositQuote.LiquidityDelta,
			MaxAmountTokenA:       inAmount,
			MaxAmountTokenB:       inAmount,
			TokenAAmountThreshold: dammv2.U64Max,
			TokenBAmountThreshold: dammv2.U64Max,
		})

		if err != nil {
			t.Fatal("cpAmm.AddLiquidity() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("AddLiquidity txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
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

		txBuilder, err = cpAmm.RemoveLiquidity(ctx1, dammv2.RemoveLiquidityParams{
			Owner:                 owner,
			Pool:                  pool.PublicKey,
			PoolState:             pool.Account,
			Position:              position,
			PositionNftAccount:    positionNftAccount,
			LiquidityDelta:        depositQuote.LiquidityDelta,
			TokenAAmountThreshold: big.NewInt(0),
			TokenBAmountThreshold: big.NewInt(0),
			// Vestings              []VestingWithAccount
			// CurrentPoint          *big.Int
		})
		if err != nil {
			t.Fatal("cpAmm.RemoveLiquidity() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("RemoveLiquidity txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
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

		txBuilder, err = cpAmm.ClosePosition(ctx1, dammv2.ClosePositionParams{
			Owner:              owner,
			Pool:               pool.PublicKey,
			Position:           position,
			PositionNftAccount: positionNftAccount,
			PositionNftMint:    positionNft,
		})

		if err != nil {
			t.Fatal("cpAmm.ClosePosition() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("ClosePosition txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
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

		balance, err = MintBalance(ctx1, rpcClient, owner, baseMint)
		if err != nil {
			t.Fatal("MintBalance() fail", err)
		}

		currentPoint := dammv2.CurrentPointForActivation(ctx1, rpcClient, rpc.CommitmentFinalized, dammv2.ActivationType(pool.Account.ActivationType))

		quote, err := cpAmm.GetQuote(dammv2.GetQuoteParams{
			InAmount:        new(big.Int).SetUint64(balance / 3),
			InputTokenMint:  pool.Account.TokenAMint,
			Slippage:        5000,
			PoolState:       pool.Account,
			CurrentPoint:    currentPoint,
			InputTokenInfo:  inputTokenInfo,
			OutputTokenInfo: outputTokenInfo,
			TokenADecimal:   dammv2.TokenDecimalNine,
			TokenBDecimal:   dammv2.TokenDecimalNine,
			// HasReferral     bool
		})
		if err != nil {
			t.Fatal("cpAmm.GetQuote() fail", err)
		}

		txBuilder, err = cpAmm.Swap(ctx1, dammv2.SwapParams{
			Payer:            owner,
			Pool:             pool.PublicKey,
			PoolState:        pool.Account,
			InputTokenMint:   pool.Account.TokenAMint,
			OutputTokenMint:  pool.Account.TokenBMint,
			AmountIn:         new(big.Int).SetUint64(balance / 3),
			MinimumAmountOut: quote.MinSwapOutAmount,
			// ReferralTokenAccount *solanago.PublicKey
			// Receiver             *solanago.PublicKey
		})
		if err != nil {
			t.Fatal("cpAmm.Swap() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("Swap txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("Swap SendTransaction() fail", err)
		}
		fmt.Println("swap success Success sig:", sig.String())

		currentPoint = dammv2.CurrentPointForActivation(ctx1, rpcClient, rpc.CommitmentFinalized, dammv2.ActivationType(pool.Account.ActivationType))

		quote2, err := cpAmm.GetQuote2(dammv2.GetQuote2Params{
			InputTokenMint:  pool.Account.TokenAMint,
			Slippage:        10000,
			PoolState:       pool.Account,
			CurrentPoint:    currentPoint,
			InputTokenInfo:  inputTokenInfo,
			OutputTokenInfo: outputTokenInfo,
			TokenADecimal:   dammv2.TokenDecimalNine,
			TokenBDecimal:   dammv2.TokenDecimalNine,
			// HasReferral     bool
			SwapMode: dammv2.SwapModeExactIn,
			AmountIn: new(big.Int).SetUint64(balance - balance/3),
			// AmountOut       *big.Int
		})
		if err != nil {
			t.Fatal("cpAmm.GetQuote() fail", err)
		}

		txBuilder, err = cpAmm.Swap2(ctx1, dammv2.Swap2Params{
			Payer:           owner,
			Pool:            pool.PublicKey,
			PoolState:       pool.Account,
			InputTokenMint:  pool.Account.TokenAMint,
			OutputTokenMint: pool.Account.TokenBMint,
			// ReferralTokenAccount *solanago.PublicKey
			// Receiver             *solanago.PublicKey
			SwapMode:         dammv2.SwapModeExactIn,
			AmountIn:         new(big.Int).SetUint64(balance - balance/3),
			MinimumAmountOut: quote2.MinimumAmountOut,
			// AmountOut            *big.Int
			// MaximumAmountIn      *big.Int
		})

		if err != nil {
			t.Fatal("cpAmm.Swap2() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("Swap2 txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("Swap2 SendTransaction() fail", err)
		}
		fmt.Println("swap2 success Success sig:", sig.String())

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

		pools, err := cpAmm.GetMultiplePools(ctx1, []solana.PublicKey{pool.PublicKey})
		if err != nil {
			t.Fatal("cpAmm.GetMultiplePools() fail", err)
		}

		baseFee, quoteFee, _, err := cpAmm.GetUnClaimLpFee(pools[0], partnerPosition.PositionState)
		if err != nil {
			t.Fatal("cpAmm.GetUnclaimedLpFee() fail", err)
		}
		fmt.Println("baseFee:", baseFee)
		fmt.Println("quoteFee:", quoteFee)

		if quoteFee.Sign() > 0 {

			txBuilder, err = cpAmm.ClaimPositionFee(ctx1, dammv2.ClaimPositionFeeParams{
				Owner:              partner.PublicKey(),
				Position:           partnerPosition.Position,
				Pool:               pool.PublicKey,
				PositionNftAccount: partnerPosition.PositionNftAccount,
				PoolState:          pools[0],
				// Receiver           *solanago.PublicKey
				// FeePayer           *solanago.PublicKey
				// TempWSolAccount    *solanago.PublicKey
			})
			if err != nil {
				t.Fatal("cpAmm.ClaimPositionFee() fail", err)
			}
			tx, err = txBuilder.SetFeePayer(partner.PublicKey()).Build()
			if err != nil {
				t.Fatal("claim txBuilder.Build() fail", err)
			}
			sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
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
}
