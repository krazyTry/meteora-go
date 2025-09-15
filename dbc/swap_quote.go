package dbc

import (
	"context"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	"github.com/shopspring/decimal"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	solanago "github.com/krazyTry/meteora-go/solana"
)

// SwapQuote gets the exact swap out quotation for quote and base swaps.
// It depends on the SwapQuote function.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// result, poolState, configState, currentPoint, _ := m.SwapQuote(
//
//	ctx,
//	baseMint, // pool (token) address
//	false, // buy(quote=>base) sell(base => quote)
//	amountIn, // amount to spend on selling or buying
//	slippageBps, // slippage // 250 = 2.5%
//	hasReferral, // default false, contact meteora
//
// )
func (m *DBC) SwapQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	swapBaseForQuote bool, // buy(quote=>base) sell(base => quote)
	amountIn *big.Int,
	slippageBps uint64,
	hasReferral bool, // default false
) (*dbc.QuoteResult, *Pool, *dbc.PoolConfig, *big.Int, error) {
	return SwapQuote(ctx, m.rpcClient, baseMint, swapBaseForQuote, amountIn, slippageBps, hasReferral)
}

// SwapQuote gets the exact swap out quotation for quote and base swaps.
//
// Example:
//
// result, poolState, configState, currentPoint, _ := SwapQuote(
//
//	ctx,
//	rpcClient,
//	baseMint, // pool (token) address
//	false, // buy(quote=>base) sell(base => quote)
//	amountIn, // amount to spend on selling or buying
//	slippageBps, // slippage // 250 = 2.5%
//	hasReferral, // default false, contact meteora
//
// )
func SwapQuote(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
	swapBaseForQuote bool, // buy(quote=>base) sell(base => quote)
	amountIn *big.Int,
	slippageBps uint64,
	hasReferral bool, // default false
) (*dbc.QuoteResult, *Pool, *dbc.PoolConfig, *big.Int, error) {
	poolState, err := GetPoolByBaseMint(ctx, rpcClient, baseMint)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	configState, err := GetConfig(ctx, rpcClient, poolState.Config)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if poolState.IsMigrated == dbc.IsMigratedCompleted {
		return nil, nil, nil, nil, ErrPoolCompleted
	}

	currentPoint, err := solanago.CurrenPoint(ctx, rpcClient, uint8(configState.ActivationType))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	quote, err := dbc.SwapQuote(
		poolState.VirtualPool,
		configState,
		swapBaseForQuote,
		decimal.NewFromBigInt(amountIn, 0),
		decimal.NewFromUint64(uint64(slippageBps)),
		hasReferral,
		decimal.NewFromBigInt(currentPoint, 0),
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return quote, poolState, configState, currentPoint, nil
}

// BuyQuote gets the exact quotation for buying a specified amount of base token using quote token.
//
// Example:
//
// result, poolState, configState, currentPoint, _ := BuyQuote(
//
//	ctx,
//	baseMint, // pool (token) address
//	amountIn, // amount to spend on buying
//	slippageBps, // slippage // 250 = 2.5%
//	hasReferral, // default false, contact meteora
//
// )
func (m *DBC) BuyQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
	hasReferral bool,
) (*dbc.QuoteResult, *Pool, *dbc.PoolConfig, *big.Int, error) {
	return m.SwapQuote(ctx, baseMint, false, amountIn, slippageBps, hasReferral)
}

// SellQuote gets the exact quotation for selling a specified amount of base token to receive quote token.
//
// Example:
//
// result, poolState, configState, currentPoint, _ := SellQuote(
//
//	ctx,
//	baseMint, // pool (token) address
//	amountIn, // amount to spend on selling
//	slippageBps, // slippage // 250 = 2.5%
//	hasReferral, // default false, contact meteora
//
// )
func (m *DBC) SellQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
	hasReferral bool,
) (*dbc.QuoteResult, *Pool, *dbc.PoolConfig, *big.Int, error) {
	return m.SwapQuote(ctx, baseMint, true, amountIn, slippageBps, hasReferral)
}
