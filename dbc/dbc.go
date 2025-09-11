package dbc

import (
	"errors"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

var (
	ErrPoolCompleted = errors.New("virtual pool is completed")

	ErrDammV2LockerNotRequired = errors.New("locker not required")
	ErrMigrationProgressState  = errors.New("virtual pool state error")

	poolAuthority        solana.PublicKey
	eventAuthority       solana.PublicKey
	lockerEventAuthority solana.PublicKey

	dammPoolAuthority  solana.PublicKey
	dammEventAuthority solana.PublicKey

	transferFee = uint64(5000) // 0.000005 SOL
)

func Init() error {
	var err error
	poolAuthority, err = dbc.DerivePoolAuthorityPDA()
	if err != nil {
		return err
	}

	eventAuthority, err = dbc.DeriveEventAuthorityPDA()
	if err != nil {
		return err
	}

	lockerEventAuthority, err = dbc.DeriveLockerEventAuthority()
	if err != nil {
		return err
	}

	dammPoolAuthority, err = dbc.DeriveDammV2PoolAuthority()
	if err != nil {
		return err
	}

	dammEventAuthority, err = dbc.DeriveDammV2EventAuthority()
	if err != nil {
		return err
	}
	return nil
}

type DBC struct {
	rpcClient        *rpc.Client
	config           *solana.Wallet
	feeClaimer       *solana.Wallet
	leftoverReceiver *solana.Wallet
	poolCreator      *solana.Wallet
}

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
