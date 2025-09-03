package dbc

import (
	"context"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
)

func dbcCreatePartnerMetadata(
	m *DBC,
	// Params:
	name string,
	website string,
	logo string,

	// Accounts:
	partnerMetadata solana.PublicKey,
	payer solana.PublicKey,
	feeClaimer solana.PublicKey,
) (solana.Instruction, error) {
	eventAuthority := m.eventAuthority

	program := dbc.ProgramID
	systemProgram := solana.SystemProgramID

	metadata := dbc.CreatePartnerMetadataParameters{
		Name:    name,
		Website: website,
		Logo:    logo,
	}
	return dbc.NewCreatePartnerMetadataInstruction(
		// Params:
		metadata,

		// Accounts:
		partnerMetadata,
		payer,
		feeClaimer,
		systemProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) CreatePartnerMetadataInstruction(
	ctx context.Context,
	payer *solana.Wallet,
	name string,
	website string,
	logo string,
) (solana.Instruction, error) {

	partnerMetadata, err := dbc.DerivePartnerMetadataPDA(m.feeClaimer.PublicKey())
	if err != nil {
		return nil, err
	}

	return dbcCreatePartnerMetadata(
		m,
		name,
		website,
		logo,
		partnerMetadata,
		payer.PublicKey(),
		m.feeClaimer.PublicKey(),
	)
}

func dbcCreateVirtualPoolMetadata(
	m *DBC,
	// Params:
	name string,
	website string,
	logo string,

	// Accounts:
	dbcPool solana.PublicKey,
	virtualPoolMetadata solana.PublicKey,
	creator solana.PublicKey,
	payer solana.PublicKey,
) (solana.Instruction, error) {
	eventAuthority := m.eventAuthority
	program := dbc.ProgramID
	systemProgram := solana.SystemProgramID

	metadata := dbc.CreateVirtualPoolMetadataParameters{
		Name:    name,
		Website: website,
		Logo:    logo,
	}

	return dbc.NewCreateVirtualPoolMetadataInstruction(
		// Params:
		metadata,

		// Accounts:
		dbcPool,
		virtualPoolMetadata,
		creator,
		payer,
		systemProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) CreateVirtualPoolMetadataInstruction(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.Wallet,
	name string,
	website string,
	logo string,
) (solana.Instruction, error) {
	virtualPoolMetadata, err := dbc.DeriveDbcPoolMetadataPDA(baseMint.PublicKey())
	if err != nil {
		return nil, err
	}

	return dbcCreateVirtualPoolMetadata(
		m,
		name,
		website,
		logo,
		baseMint.PublicKey(),
		virtualPoolMetadata,
		m.poolCreator.PublicKey(),
		payer.PublicKey(),
	)
}
