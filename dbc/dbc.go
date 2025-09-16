package dbc

import (
	"errors"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

var (
	// ErrPoolCompleted dbc pool has been converted to dammv2
	ErrPoolCompleted = errors.New("virtual pool is completed")

	// ErrDammV2LockerNotRequired dbc pool does not require locking
	ErrDammV2LockerNotRequired = errors.New("locker not required")
	// ErrMigrationProgressState dbc pool state error
	ErrMigrationProgressState = errors.New("virtual pool state error")

	poolAuthority        solana.PublicKey
	eventAuthority       solana.PublicKey
	lockerEventAuthority solana.PublicKey

	dammPoolAuthority  solana.PublicKey
	dammEventAuthority solana.PublicKey

	rentExemptFee = uint64(2_039_280)
	transferFee   = uint64(5_000) // 0.000005 SOL
)

// Init performs initialization.
// It completes the generation of poolAuthority, eventAuthority, lockerEventAuthority, dammPoolAuthority, and dammEventAuthority in the dbc pool.
func init() {
	var err error
	poolAuthority, err = dbc.DerivePoolAuthorityPDA()
	if err != nil {
		panic(err)
	}

	eventAuthority, err = dbc.DeriveEventAuthorityPDA()
	if err != nil {
		panic(err)
	}

	lockerEventAuthority, err = dbc.DeriveLockerEventAuthority()
	if err != nil {
		panic(err)
	}

	dammPoolAuthority, err = dbc.DeriveDammV2PoolAuthority()
	if err != nil {
		panic(err)
	}

	dammEventAuthority, err = dbc.DeriveDammV2EventAuthority()
	if err != nil {
		panic(err)
	}
}

// DBC
type DBC struct {
	rpcClient        *rpc.Client    // solana rpc client
	config           *solana.Wallet // config wallet
	feeClaimer       *solana.Wallet // partner wallet
	leftoverReceiver *solana.Wallet // leftover receiver account wallet
	poolCreator      *solana.Wallet // pool creator account wallet
}

// NewDBC creates a meteora dbc object.
//
// Example:
//
// config := solana.NewWallet()
// poolCreator := solana.NewWallet()
// poolPartner := solana.NewWallet()
// leftoverReceiver := solana.NewWallet()
//
// meteoraDBC, _ := dbc.NewDBC(
//
//	rpcClient, // solana rpc client
//	config, // config wallet
//	poolCreator, // partner wallet
//	poolPartner, // leftover receiver account wallet
//	leftoverReceiver, // pool creator account wallet
//
// )
func NewDBC(
	rpcClient *rpc.Client,
	config *solana.Wallet,
	poolCreator *solana.Wallet,
	poolPartner *solana.Wallet,
	leftoverReceiver *solana.Wallet,
) (*DBC, error) {

	return &DBC{
		rpcClient:        rpcClient,
		config:           config,
		feeClaimer:       poolPartner,
		leftoverReceiver: leftoverReceiver,
		poolCreator:      poolCreator,
	}, nil
}
