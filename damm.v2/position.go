package dammV2

import (
	"context"
	"fmt"
	"maps"
	"math/big"
	"slices"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/krazyTry/meteora-go/u128"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func cpAmmCreatePosition(
	m *DammV2,
	owner solana.PublicKey,
	positionNft solana.PublicKey,
	positionNftAccount solana.PublicKey,
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	payer solana.PublicKey,
	tokenProgram solana.PublicKey,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	systemProgram := solana.SystemProgramID
	program := cp_amm.ProgramID

	return cp_amm.NewCreatePositionInstruction(
		owner,
		positionNft,
		positionNftAccount,
		cpammPool,
		position,
		poolAuthority,
		payer,
		tokenProgram,
		systemProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) CreatePositionInstruction(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	ownerPositionNft *solana.Wallet,
	virtualPool *Pool,
) ([]solana.Instruction, error) {
	position, err := cp_amm.DerivePositionAddress(ownerPositionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	positionNftAccount, err := cp_amm.DerivePositionNftAccount(ownerPositionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	createIx, err := cpAmmCreatePosition(
		m,
		owner.PublicKey(),
		ownerPositionNft.PublicKey(),
		positionNftAccount,
		virtualPool.Address,
		position,
		payer.PublicKey(),
		cp_amm.GetTokenProgram(virtualPool.TokenAFlag),
	)
	if err != nil {
		return nil, err
	}
	return []solana.Instruction{createIx}, nil
}

func (m *DammV2) CreatePosition(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {

	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	positionNft := solana.NewWallet()

	instructions, err := m.CreatePositionInstruction(ctx,
		payer,
		owner,
		positionNft,
		virtualPool,
	)

	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func cpAmmClosePosition(
	m *DammV2,
	positionNft solana.PublicKey,
	positionNftAccount solana.PublicKey,
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	rentReceiver solana.PublicKey,
	owner solana.PublicKey,
	tokenProgram solana.PublicKey,
) (solana.Instruction, error) {
	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	return cp_amm.NewClosePositionInstruction(
		positionNft,
		positionNftAccount,
		cpammPool,
		position,
		poolAuthority,
		rentReceiver,
		owner,
		tokenProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) ClosePositionInstruction(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	ownerPosition *UserPosition,
	virtualPool *Pool,
) ([]solana.Instruction, error) {
	closeIx, err := cpAmmClosePosition(m,
		ownerPosition.PositionState.NftMint,
		ownerPosition.PositionNftAccount,
		virtualPool.Address,
		ownerPosition.Position,
		payer.PublicKey(),
		owner.PublicKey(),
		cp_amm.GetTokenProgram(virtualPool.TokenAFlag),
	)

	if err != nil {
		return nil, err
	}
	return []solana.Instruction{closeIx}, nil
}

func (m *DammV2) ClosePosition(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, virtualPool.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := m.ClosePositionInstruction(ctx,
		payer,
		owner,
		userPosition,
		virtualPool,
	)

	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func cpAmmLockPosition(
	m *DammV2,
	// Params:
	cliffPoint *uint64,
	periodFrequency uint64,
	cliffUnlockLiquidity *big.Int,
	liquidityPerPeriod *big.Int,
	numberOfPeriod uint16,

	// Accounts:
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	vesting solana.PublicKey,
	positionNftAccount solana.PublicKey,
	owner solana.PublicKey,
	payer solana.PublicKey,
) (solana.Instruction, error) {

	systemProgram := solana.SystemProgramID
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	param := &cp_amm.VestingParameters{
		CliffPoint:           cliffPoint,
		PeriodFrequency:      periodFrequency,
		CliffUnlockLiquidity: u128.GenUint128FromString(cliffUnlockLiquidity.String()),
		LiquidityPerPeriod:   u128.GenUint128FromString(liquidityPerPeriod.String()),
		NumberOfPeriod:       numberOfPeriod,
	}

	return cp_amm.NewLockPositionInstruction(
		// Params:
		param,

		// Accounts:
		cpammPool,
		position,
		vesting,
		positionNftAccount,
		owner,
		payer,
		systemProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) LockPositionInstruction(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	ownerPosition *UserPosition,
	virtualPool *Pool,
	cliffPoint *uint64,
	periodFrequency uint64,
	cliffUnlockLiquidity *big.Int,
	liquidityPerPeriod *big.Int,
	numberOfPeriod uint16,
	vesting solana.PublicKey,
) ([]solana.Instruction, error) {

	cpammPool := virtualPool.Address

	lockIx, err := cpAmmLockPosition(m,
		cliffPoint,
		periodFrequency,
		cliffUnlockLiquidity,
		liquidityPerPeriod,
		numberOfPeriod,
		cpammPool,
		ownerPosition.Position,
		vesting,
		ownerPosition.PositionNftAccount,
		owner.PublicKey(),
		payer.PublicKey(),
	)

	if err != nil {
		return nil, err
	}
	return []solana.Instruction{lockIx}, nil
}

func (m *DammV2) LockPosition(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
	cliffPoint *uint64,
	periodFrequency uint64,
	cliffUnlockLiquidity *big.Int,
	liquidityPerPeriod *big.Int,
	numberOfPeriod uint16,
	vesting solana.PublicKey,
) (string, error) {

	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, virtualPool.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}

	userPosition = userPositions[0]
	instructions, err := m.LockPositionInstruction(ctx,
		payer,
		owner,
		userPosition,
		virtualPool,
		cliffPoint,
		periodFrequency,
		cliffUnlockLiquidity,
		liquidityPerPeriod,
		numberOfPeriod,
		vesting,
	)
	if err != nil {
		return "", err
	}
	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func cpAmmPermanentLockPosition(
	m *DammV2,
	// Params:
	permanentLockLiquidity *big.Int,

	// Accounts:
	pool solana.PublicKey,
	position solana.PublicKey,
	positionNftAccount solana.PublicKey,
	owner solana.PublicKey,
) (solana.Instruction, error) {
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	return cp_amm.NewPermanentLockPositionInstruction(
		// Params:
		u128.GenUint128FromString(permanentLockLiquidity.String()),

		// Accounts:
		pool,
		position,
		positionNftAccount,
		owner,
		eventAuthority,
		program,
	)
}

func (m *DammV2) PermanentLockPositionInstruction(
	ctx context.Context,
	owner *solana.Wallet,
	ownerPosition *UserPosition,
	virtualPool *Pool,
	permanentLockLiquidity *big.Int,
) ([]solana.Instruction, error) {
	lockIx, err := cpAmmPermanentLockPosition(m,
		permanentLockLiquidity,
		virtualPool.Address,
		ownerPosition.Position,
		ownerPosition.PositionNftAccount,
		owner.PublicKey(),
	)
	if err != nil {
		return nil, err
	}
	return []solana.Instruction{lockIx}, nil
}

func (m *DammV2) PermanentLockPosition(
	ctx context.Context,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
	permanentLockLiquidity *big.Int,
) (string, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, virtualPool.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}
	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := m.PermanentLockPositionInstruction(ctx,
		owner,
		userPosition,
		virtualPool,
		permanentLockLiquidity,
	)
	if err != nil {
		return "", err
	}
	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
		instructions,
		owner.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner.PublicKey()):
				return &owner.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func cpAmmSplitPosition(
	m *DammV2,
	unlockedLiquidityPercentage uint8,
	permanentLockedLiquidityPercentage uint8,
	feeAPercentage uint8,
	feeBPercentage uint8,
	reward0Percentage uint8,
	reward1Percentage uint8,

	// Accounts:
	cpammPool solana.PublicKey,
	firstPosition solana.PublicKey,
	firstPositionNftAccount solana.PublicKey,
	secondPosition solana.PublicKey,
	secondPositionNftAccount solana.PublicKey,
	firstOwner solana.PublicKey,
	secondOwner solana.PublicKey,
) (solana.Instruction, error) {
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	param := cp_amm.SplitPositionParameters{
		UnlockedLiquidityPercentage:        unlockedLiquidityPercentage,
		PermanentLockedLiquidityPercentage: permanentLockedLiquidityPercentage,
		FeeAPercentage:                     feeAPercentage,
		FeeBPercentage:                     feeBPercentage,
		Reward0Percentage:                  reward0Percentage,
		Reward1Percentage:                  reward1Percentage,
	}

	return cp_amm.NewSplitPositionInstruction(
		// Params:
		param,

		// Accounts:
		cpammPool,
		firstPosition,
		firstPositionNftAccount,
		secondPosition,
		secondPositionNftAccount,
		firstOwner,
		secondOwner,
		eventAuthority,
		program,
	)
}

func (m *DammV2) SplitPositionInstruction(
	ctx context.Context,
	owner *solana.Wallet,
	ownerPosition *UserPosition,
	newOwner *solana.Wallet,
	newOwnerPositionNft *solana.Wallet,
	virtualPool *Pool,
	unlockedLiquidityPercentage uint8,
	permanentLockedLiquidityPercentage uint8,
	feeAPercentage uint8,
	feeBPercentage uint8,
	reward0Percentage uint8,
	reward1Percentage uint8,
) ([]solana.Instruction, error) {

	position, err := cp_amm.DerivePositionAddress(newOwnerPositionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	positionNftAccount, err := cp_amm.DerivePositionNftAccount(newOwnerPositionNft.PublicKey())
	if err != nil {
		return nil, err
	}
	splitIx, err := cpAmmSplitPosition(m,
		unlockedLiquidityPercentage,
		permanentLockedLiquidityPercentage,
		feeAPercentage,
		feeBPercentage,
		reward0Percentage,
		reward1Percentage,
		virtualPool.Address,
		ownerPosition.Position,
		ownerPosition.PositionNftAccount,
		position,
		positionNftAccount,
		owner.PublicKey(),
		newOwner.PublicKey(),
	)
	if err != nil {
		return nil, err
	}
	return []solana.Instruction{splitIx}, nil
}

func (m *DammV2) SplitPosition(
	ctx context.Context,
	baseMint solana.PublicKey,
	owner *solana.Wallet,
	newOwner *solana.Wallet,
	unlockedLiquidityPercentage uint8,
	permanentLockedLiquidityPercentage uint8,
	feeAPercentage uint8,
	feeBPercentage uint8,
	reward0Percentage uint8,
	reward1Percentage uint8,
) (string, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, virtualPool.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}

	userPosition = userPositions[0]

	positionNft := solana.NewWallet()

	instructions, err := m.SplitPositionInstruction(ctx,
		owner,
		userPosition,
		newOwner,
		positionNft,
		virtualPool,
		unlockedLiquidityPercentage,
		permanentLockedLiquidityPercentage,
		feeAPercentage,
		feeBPercentage,
		reward0Percentage,
		reward1Percentage,
	)
	if err != nil {
		return "", nil
	}
	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
		instructions,
		owner.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner.PublicKey()):
				return &owner.PrivateKey
			case key.Equals(newOwner.PublicKey()):
				return &newOwner.PrivateKey
			case key.Equals(positionNft.PublicKey()):
				return &positionNft.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func (m *DammV2) getPositionNftAccountsByUser(ctx context.Context, user solana.PublicKey) (map[solana.PublicKey]*solanago.Account, error) {
	out, err := m.rpcClient.GetTokenAccountsByOwner(
		ctx,
		user,
		&rpc.GetTokenAccountsConfig{
			ProgramId: &solana.Token2022ProgramID,
		},
		&rpc.GetTokenAccountsOpts{
			Commitment: rpc.CommitmentFinalized,
			Encoding:   solana.EncodingBase64,
		},
	)
	if err != nil {
		return nil, err
	}

	data := make(map[solana.PublicKey]*solanago.Account)

	for _, keyedAcc := range out.Value {
		tokenAccount, err := new(solanago.AccountLayout).Decode(keyedAcc.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		tokenAccount.Address = keyedAcc.Pubkey

		if tokenAccount.Amount == 1 {
			position, err := cp_amm.DerivePositionAddress(tokenAccount.Mint)
			if err != nil {
				return nil, err
			}
			data[position] = tokenAccount
		}
	}
	return data, nil
}

func (m *DammV2) GetUserPositionByUserAndPoolPDA(ctx context.Context, poolPDA solana.PublicKey, user solana.PublicKey) ([]*UserPosition, error) {
	// func (m *DammV2) GetUserPositionByBaseMint(ctx context.Context, cpammPool *Pool, user solana.PublicKey) ([]*UserPosition, error) {
	userPositionNftAccounts, err := m.getPositionNftAccountsByUser(ctx, user)
	if err != nil {
		return nil, err
	}

	if len(userPositionNftAccounts) == 0 {
		return nil, nil
	}

	positions, err := m.GetPositionsByPoolPDA(ctx, poolPDA)
	if err != nil {
		return nil, err
	}

	var positionResult []*UserPosition

	for _, v := range positions {
		account, ok := userPositionNftAccounts[v.Position]
		if !ok {
			continue
		}

		positionResult = append(positionResult, &UserPosition{
			Position:           v.Position,
			PositionState:      v.PositionState,
			PositionNftAccount: account.Address,
		})
	}

	return positionResult, nil
}

// fetchMultiplePositions
func (m *DammV2) fetchMultiplePositions(ctx context.Context, addresses []solana.PublicKey) (map[solana.PublicKey]*cp_amm.Position, error) {
	accounts, err := solanago.GetMultipleAccountInfo(ctx, m.rpcClient, addresses)
	if err != nil {
		return nil, err
	}

	data := make(map[solana.PublicKey]*cp_amm.Position)
	for _, account := range accounts.Value {
		if account == nil {
			continue
		}
		obj, err := cp_amm.ParseAnyAccount(account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		position, ok := obj.(*cp_amm.Position)
		if !ok {
			return nil, fmt.Errorf("obj.(*cp_amm.Position) fail")
		}

		data[position.NftMint] = position
	}
	return data, nil
}

func (m *DammV2) GetUserPositionsByUser(ctx context.Context, user solana.PublicKey) ([]*UserPosition, error) {
	userPositionNftAccounts, err := m.getPositionNftAccountsByUser(ctx, user)
	if err != nil {
		return nil, err
	}

	if len(userPositionNftAccounts) == 0 {
		return nil, nil
	}

	positionAddresses := slices.Collect(maps.Keys(userPositionNftAccounts))

	positionStates, err := m.fetchMultiplePositions(ctx, positionAddresses)
	if err != nil {
		return nil, err
	}

	var positionResult []*UserPosition
	for _, account := range userPositionNftAccounts {
		state, ok := positionStates[account.Mint]
		if !ok {
			continue
		}

		position, err := cp_amm.DerivePositionAddress(account.Mint)
		if err != nil {
			return nil, err
		}

		positionResult = append(positionResult, &UserPosition{
			PositionNftAccount: account.Address,
			Position:           position,
			PositionState:      state,
		})
	}
	return positionResult, nil
}

func (m *DammV2) GetPositionsByPoolPDA(ctx context.Context, poolPDA solana.PublicKey) ([]*Position, error) {
	opt := solanago.GenProgramAccountFilter(
		cp_amm.AccountKeyPosition,
		&solanago.Filter{
			Owner:  poolPDA,
			Offset: 8,
		},
	)

	outs, err := m.rpcClient.GetProgramAccountsWithOpts(ctx, cp_amm.ProgramID, opt)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	var list []*Position
	for _, out := range outs {
		obj, err := cp_amm.ParseAnyAccount(out.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		position, ok := obj.(*cp_amm.Position)
		if !ok {
			return nil, fmt.Errorf("obj.(*cp_amm.Position) fail")
		}

		list = append(list, &Position{
			Position:      out.Pubkey,
			PositionState: position,
		})
	}

	return list, nil
}
