package meteora

import (
	dammV2 "github.com/krazyTry/meteora-go/damm.v2"
	"github.com/krazyTry/meteora-go/dbc"
)

// NewDBClient creates a new DBC client.
//
// Example:
//
// meteoraDBC, _ := NewDBClient(rpcClient, config, poolCreator, poolPartner, leftoverReceiver)
//
// meteoraDBC.CreatePoolWithFirstBuy(ctx1, wsClient, ownerWallet, mintWallet, name, symbol, uri, amountIn, 250)
//
// meteoraDBC.SellQuote(ctx1, baseMint, amountIn, 250, false)
var NewDBClient = dbc.NewDBC

// NewDammV2Client creates a new DAMM V2 client.
//
// Example:
//
// dammV2Client, _ := NewDammV2Client(rpcClient, poolCreator)
//
// meteoraDammV2.CreatePool(ctx, wsClient,payer, 0, 1, baseMint, solana.WrappedSol, baseAmount, quoteAmount, nil, true)
//
// meteoraDammV2.CreatePosition(ctx1, wsClient, payer, poolPartner, baseMint)
var NewDammV2Client = dammV2.NewDammV2
