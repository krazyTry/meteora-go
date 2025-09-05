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

func (m *DBC) SwapQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	swapBaseForQuote bool, // buy(quote=>base) sell(base => quote)
	amountIn *big.Int,
	slippageBps uint64,
	// hasReferral bool, // default false
) (*dbc.QuoteResult, *Pool, *dbc.PoolConfig, *big.Int, error) {
	return SwapQuote(ctx, m.rpcClient, baseMint, swapBaseForQuote, amountIn, slippageBps)
}

func SwapQuote(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
	swapBaseForQuote bool, // buy(quote=>base) sell(base => quote)
	amountIn *big.Int,
	slippageBps uint64,
	// hasReferral bool, // default false
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
		false,
		decimal.NewFromBigInt(currentPoint, 0),
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return quote, poolState, configState, currentPoint, nil
}

func (m *DBC) BuyQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
) (*dbc.QuoteResult, *Pool, *dbc.PoolConfig, *big.Int, error) {
	return m.SwapQuote(ctx, baseMint, false, amountIn, slippageBps)
}

func (m *DBC) SellQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
) (*dbc.QuoteResult, *Pool, *dbc.PoolConfig, *big.Int, error) {
	return m.SwapQuote(ctx, baseMint, true, amountIn, slippageBps)
}
