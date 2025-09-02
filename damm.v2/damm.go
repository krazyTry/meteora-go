package dammV2

import (
	"context"
	"fmt"
	"time"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

type DammV2 struct {
	bSimulate bool

	wsClient  *ws.Client
	rpcClient *rpc.Client

	poolCreator *solana.Wallet
	config      *solana.Wallet

	dammConfig  solana.PublicKey
	cpAmmConfig solana.PublicKey

	poolAuthority  solana.PublicKey
	eventAuthority solana.PublicKey
}

func NewDammV2(
	wsClient *ws.Client,
	rpcClient *rpc.Client,
	poolCreator *solana.Wallet,
	optFuns ...Option,
) (*DammV2, error) {

	poolAuthority, err := cp_amm.DerivePoolAuthorityPDA()
	if err != nil {
		return nil, err
	}

	eventAuthority, err := cp_amm.DeriveEventAuthorityPDA()
	if err != nil {
		return nil, err
	}

	m := &DammV2{
		wsClient:       wsClient,
		rpcClient:      rpcClient,
		poolCreator:    poolCreator,
		poolAuthority:  poolAuthority,
		eventAuthority: eventAuthority,
		dammConfig:     solana.PublicKey{},
		cpAmmConfig:    solana.PublicKey{},
	}

	for _, fn := range optFuns {
		if err = fn(m); err != nil {
			return nil, err
		}
	}

	if m.dammConfig.Equals(solana.PublicKey{}) && m.cpAmmConfig.Equals(solana.PublicKey{}) && m.config == nil {
		return nil, fmt.Errorf("dbcConfig or cpAmmConfig or config must exist")
	}

	return m, nil
}

type Option func(*DammV2) error

func WithDBCConfig(dbcConfig *solana.Wallet) Option {
	return func(m *DammV2) error {
		config, err := m.getDammConfigByDbcConfig(m.rpcClient, dbcConfig.PublicKey())
		if err != nil {
			return err
		}
		m.dammConfig = config
		return nil
	}
}

func WithCpAmmConfigIDX(idx uint8) Option {
	return func(m *DammV2) error {
		config, err := cp_amm.DeriveConfigAddress(idx)
		if err != nil {
			return err
		}
		m.cpAmmConfig = config
		return nil
	}
}

func WithCpAmmConfig(configurator *solana.Wallet) Option {
	return func(m *DammV2) error {
		m.config = configurator
		return nil
	}
}

func (m *DammV2) getDammConfigByDbcConfig(rpcClient *rpc.Client, dbcConfig solana.PublicKey) (solana.PublicKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	out, err := solanago.GetAccountInfo(ctx, rpcClient, dbcConfig)
	if err != nil {
		if err == rpc.ErrNotFound {
			return solana.PublicKey{}, nil
		}
		return solana.PublicKey{}, err
	}
	obj, err := dbc.ParseAnyAccount(out.GetBinary())
	if err != nil {
		return solana.PublicKey{}, err
	}

	config, ok := obj.(*dbc.PoolConfig)
	if !ok {
		return solana.PublicKey{}, fmt.Errorf("obj.(*dbc.PoolConfig) fail")
	}
	return dbc.GetDammV2Config(config.MigrationFeeOption), nil
}

func (m *DammV2) deriveCpAmmPoolPDA(quoteMint, baseMint solana.PublicKey) (solana.PublicKey, error) {
	switch {
	case m.cpAmmConfig.Equals(solana.PublicKey{}):
		return cp_amm.DeriveCpAmmPoolPDA(m.cpAmmConfig, baseMint, quoteMint)
	case m.dammConfig.Equals(solana.PublicKey{}):
		return dbc.DeriveDammV2PoolPDA(m.dammConfig, baseMint, quoteMint)
	case m.config != nil:
		return cp_amm.DeriveCpAmmPoolPDA(m.config.PublicKey(), baseMint, quoteMint)
	default:
		return cp_amm.DeriveCustomizablePoolAddress(baseMint, quoteMint)
	}
}

type Pool struct {
	*cp_amm.Pool
	Address solana.PublicKey
}

type Position struct {
	Position      solana.PublicKey
	PositionState *cp_amm.Position
}

type UserPosition struct {
	Position           solana.PublicKey
	PositionState      *cp_amm.Position
	PositionNftAccount solana.PublicKey
}
