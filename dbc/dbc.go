package dbc

import (
	"fmt"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

var (
	errDammV2MetadataExist = fmt.Errorf("MigrationDammV2CreateMetadata exist")
	errDammV2LockerExist   = fmt.Errorf("CreateLocker exist")
)

type DBC struct {
	bSimulate bool

	wsClient         *ws.Client
	rpcClient        *rpc.Client
	config           *solana.Wallet
	feeClaimer       *solana.Wallet
	leftoverReceiver *solana.Wallet
	poolCreator      *solana.Wallet

	poolAuthority        solana.PublicKey
	eventAuthority       solana.PublicKey
	lockerEventAuthority solana.PublicKey

	dammPoolAuthority  solana.PublicKey
	dammEventAuthority solana.PublicKey
}

func NewDBC(
	wsClient *ws.Client,
	rpcClient *rpc.Client,
	config *solana.Wallet,
	poolCreator *solana.Wallet,
	poolPartner *solana.Wallet,
	leftoverReceiver *solana.Wallet,
) (*DBC, error) {

	poolAuthority, err := dbc.DerivePoolAuthorityPDA()
	if err != nil {
		return nil, err
	}

	eventAuthority, err := dbc.DeriveEventAuthorityPDA()
	if err != nil {
		return nil, err
	}

	lockerEventAuthority, err := dbc.DeriveLockerEventAuthority()
	if err != nil {
		return nil, err
	}

	dammPoolAuthority, err := dbc.DeriveDammV2PoolAuthority()
	if err != nil {
		return nil, err
	}

	dammEventAuthority, err := dbc.DeriveDammV2EventAuthority()
	if err != nil {
		return nil, err
	}

	return &DBC{
		wsClient:             wsClient,
		rpcClient:            rpcClient,
		config:               config,
		feeClaimer:           poolPartner,
		leftoverReceiver:     leftoverReceiver,
		poolCreator:          poolCreator,
		poolAuthority:        poolAuthority,
		eventAuthority:       eventAuthority,
		lockerEventAuthority: lockerEventAuthority,
		dammPoolAuthority:    dammPoolAuthority,
		dammEventAuthority:   dammEventAuthority,
	}, nil
}
