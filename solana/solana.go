package solana

import "github.com/gagliardetto/solana-go"

// IsSimulate indicates whether simulation mode is enabled
var IsSimulate bool

// Filter represents a filter for querying accounts by owner and offset
type Filter struct {
	Owner  solana.PublicKey // Account owner to filter by
	Offset uint64           // Offset for pagination
}
