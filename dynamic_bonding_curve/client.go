package dynamic_bonding_curve

import (
	"github.com/gagliardetto/solana-go/rpc"
)

// DynamicBondingCurveClient groups high-level services.
type DynamicBondingCurveClient struct {
	Pool       *PoolService
	Partner    *PartnerService
	Creator    *CreatorService
	Migration  *MigrationService
	State      *StateService
	Commitment rpc.CommitmentType
	RPC        *rpc.Client
}

// NewDynamicBondingCurveClient constructs a client with the given RPC connection.
func NewDynamicBondingCurveClient(rpcClient *rpc.Client, commitment rpc.CommitmentType) *DynamicBondingCurveClient {
	return &DynamicBondingCurveClient{
		Pool:       NewPoolService(rpcClient, commitment),
		Partner:    NewPartnerService(rpcClient, commitment),
		Creator:    NewCreatorService(rpcClient, commitment),
		Migration:  NewMigrationService(rpcClient, commitment),
		State:      NewStateService(rpcClient, commitment),
		Commitment: commitment,
		RPC:        rpcClient,
	}
}

// Create is a convenience constructor using confirmed commitment by default.
func Create(rpcClient *rpc.Client, commitment rpc.CommitmentType) *DynamicBondingCurveClient {
	if commitment == "" {
		commitment = rpc.CommitmentConfirmed
	}
	return NewDynamicBondingCurveClient(rpcClient, commitment)
}
