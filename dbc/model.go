package dbc

import (
	"github.com/gagliardetto/solana-go"
	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
)

type Pool struct {
	*dbc.VirtualPool
	Address solana.PublicKey
}
