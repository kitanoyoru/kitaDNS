package app

import (
	"encoding/json"

	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/kitanoyoru/kitaDNS/internal/app/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// runtime.AppI implementation

func (app *App) Name() string {
	return app.BaseApp.Name()
}

func (app *App) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	return app.mm.BeginBlock(ctx)
}

func (app *App) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	return app.mm.EndBlock(ctx)
}

func (app *App) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState types.GenesisState

	req.ConsensusParams = &cmtproto.ConsensusParams{
		Abci: &cmtproto.ABCIParams{
			VoteExtensionsEnableHeight: 1,
		},
	}

	err := json.Unmarshal(req.AppStateBytes, &genesisState)
	if err != nil {
		return nil, err
	}

	err = app.UpgradeKeeper.SetModuleVersionMap(ctx, app.mm.GetVersionMap())
	if err != nil {
		return nil, err
	}

	return app.mm.InitGenesis(ctx, app.appCodec, genesisState)
}

func (app *App) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

func (app *App) ExportAppStateAndValidators(forZeroHeight bool, jailAllowedAddrs, modulesToExport []string) (servertypes.ExportedApp, error) {
	ctx := app.NewContextLegacy(true, cmtproto.Header{
		Height: app.LastBlockHeight(),
	})

	height := app.LastBlockHeight() + 1
	if forZeroHeight {
		height = 0
		if err := app.prepForZeroHeightGenesis(ctx, jailAllowedAddrs); err != nil {
			log.Fatal().Err(err).Send()
		}
	}

	genState, err := app.mm.ExportGenesis(ctx, app.appCodec)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	appState, err := json.MarshalIndent(genState, "", "  ")
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	validators, err := staking.WriteValudators(ctx, app.StakingKeeper)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	return servertypes.ExportedApp{
		AppState:        appState,
		Validators:      validators,
		Height:          height,
		ConsensusParams: app.GetConsensusParams(ctx),
	}, nil
}

func (app *App) prepForZeroHeightGenesis(ctx sdk.Context, jailAllowedAddrs []string) error {
	applyAllowedAddrs := false

	if len(jailAllowedAddrs) > 0 {
		applyAllowedAddrs = true
	}

	allowedAddrsMap := make(map[string]struct{})
	for _, addr := range jailAllowedAddrs {
		_, err := sdk.ValAddressFromBech32(addr)
		if err != nil {
			return err
		}

		allowedAddrsMap[addr] = struct{}{}
	}

	/* Withdraw validators' commission  */

	err := app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		valAddr, err := app.StakingKeeper.ValidatorAddressCodec().StringToBytes(val.GetOperator())
		if err != nil {
			log.Fatal().Err(err).Send()
		}

		_, err = app.DistrKeeper.WithdrawValidatorCommission(ctx, valAddr)
		if err != nil {
			log.Fatal().Err(err).Send()
		}

		return false
	})
	if err != nil {
		return err
	}

	/* Send reward to all delegations */

	delegations, err := app.StakingKeeper.GetAllDelegations(ctx)
	if err != nil {
		return err
	}

	for _, delegation := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(delegation.ValidatorAddress)
		if err != nil {
			return err
		}

		delAddr, err := sdk.AccAddressFromBech32(delegation.DelegatorAddress)
		if err != nil {
			return err
		}

		_, err = app.DistrKeeper.WithdrawDelegationRewards(ctx, delAddr, valAddr)
		if err != nil {
			return err
		}
	}

	app.DistrKeeper.DeleteAllValidatorSlashEvents(ctx)

	app.DistrKeeper.DeleteAllValidatorHistoricalRewards(ctx)

	height := ctx.BlockHeight()
	ctx = ctx.WithBlockHeight(0)

	// reinitialize validators
	err = app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		valAddr, err := app.StakingKeeper.ValidatorAddressCodec().StringToBytes(val.GetOperator())
		if err != nil {
			log.Fatal().Err(err).Send()
		}

		scraps, err := app.DistrKeeper.GetValidatorOutstandingRewardsCoins(ctx, valAddr)
		if err != nil {
			log.Fatal().Err(err).Send()
		}

		feePool, err := app.DistrKeeper.FeePool.Get(ctx)
		if err != nil {
			log.Fatal().Err(err).Send()
		}

		feePool.CommunityPool = feePool.CommunityPool.Add(scraps...)

		err = app.DistrKeeper.FeePool.Set(ctx, feePool)
		if err != nil {
			log.Fatal().Err(err).Send()
		}

		err = app.DistrKeeper.Hooks().AfterValidatorCreated(ctx, valAddr)
		if err != nil {
			log.Fatal().Err(err).Send()
		}

		return false
	})

	// reinitialize delegations
	for _, delegation := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(delegation.ValidatorAddress)
		if err != nil {
			return err
		}

		delAddr, err := sdk.AccAddressFromBech32(delegation.DelegatorAddress)
		if err != nil {
			return err
		}

		err = app.DistrKeeper.Hooks().BeforeDelegationCreated(ctx, delAddr, valAddr)
		if err != nil {
			return err
		}

		err = app.DistrKeeper.Hooks().AfterDelegationModified(ctx, delAddr, valAddr)
		if err != nil {
			return err
		}
	}

	ctx = ctx.WithBlockHeight(height)

	/* Handle staking rates */

	err = app.StakingKeeper.IterateRedelegations(ctx, func(_ int64, red stakingtypes.Redelegation) (stop bool) {
		for i := range red.Entries {
			red.Entries[i].CreationHeight = 0
		}

		if err := app.StakingKeeper.SetRedelegation(ctx, red); err != nil {
			log.Fatal().Err(err).Send()
		}

		return false
	})

	err = app.StakingKeeper.IterateUnbondingDelegations(ctx, func(_ int64, ubd stakingtypes.UnbondingDelegation) (stop bool) {
		for i := range ubd.Entries {
			ubd.Entries[i].CreationHeight = 0
		}

		if err := app.StakingKeeper.SetUnbondingDelegation(ctx, ubd); err != nil {
			log.Fatal().Err(err).Send()
		}

		return false
	})

	store := ctx.KVStore(app.GetKey(stakingtypes.StoreKey))
	iter := storetypes.KVStoreReversePrefixIterator(store, stakingtypes.ValidatorsKey)
	counter := int16(0)

	for ; iter.Valid(); iter.Next() {
		addr := sdk.ValAddress(stakingtypes.AddressFromValidatorsKey(iter.Key()))
		validator, err := app.StakingKeeper.GetValidator(ctx, addr)
		if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
			return errors.Wrap(err, "expected validator, not found")
		} else if err != nil {
			return err
		}

		validator.UnbondingHeight = 0
		if applyAllowedAddrs {
			if _, ok := allowedAddrsMap[addr.String()]; !ok {
				validator.Jailed = true
			}
		}

		if err := app.StakingKeeper.SetValidator(ctx, validator); err != nil {
			panic(err)
		}
		counter++
	}

	if err := iter.Close(); err != nil {
		return err
	}

	_, err = app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
	if err != nil {
		return err
	}

	return nil

}
