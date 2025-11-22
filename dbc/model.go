package dbc

import (
	"github.com/gagliardetto/solana-go"
	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
)

type Pool struct {
	// dbc pool state
	*dbc.VirtualPool
	// dbc pool address
	Address solana.PublicKey
}

type Config struct {
	// dbc config state
	*dbc.PoolConfig
	// dbc config address
	Address solana.PublicKey
}
