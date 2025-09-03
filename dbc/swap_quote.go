package dbc

import (
	"context"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func (m *DBC) SwapQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	swapBaseForQuote bool, // buy(quote=>base) sell(base => quote)
	amountIn *big.Int,
	slippageBps uint64,
	// hasReferral bool, // default false
) (*dbc.QuoteResult, *dbc.VirtualPool, *dbc.PoolConfig, *big.Int, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if virtualPool.IsMigrated == dbc.IsMigratedCompleted {
		return nil, nil, nil, nil, ErrPoolCompleted
	}

	currentPoint, err := solanago.CurrenPoint(ctx, m.rpcClient, uint8(config.ActivationType))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	quote, err := dbc.SwapQuote(virtualPool, config, swapBaseForQuote, amountIn, slippageBps, false, currentPoint)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return quote, virtualPool, config, currentPoint, nil
}

func (m *DBC) BuyQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
) (*dbc.QuoteResult, *dbc.VirtualPool, *dbc.PoolConfig, *big.Int, error) {
	return m.SwapQuote(ctx, baseMint, false, amountIn, slippageBps)
}

func (m *DBC) SellQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
) (*dbc.QuoteResult, *dbc.VirtualPool, *dbc.PoolConfig, *big.Int, error) {
	return m.SwapQuote(ctx, baseMint, true, amountIn, slippageBps)
}
