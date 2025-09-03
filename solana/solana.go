package solana

import "github.com/gagliardetto/solana-go"

var IsSimulate bool

type Filter struct {
	Owner  solana.PublicKey
	Offset uint64
}
