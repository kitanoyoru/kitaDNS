package app

import (
	"encoding/json"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/kitanoyoru/kitaDNS/internal/app/types"
)


// runtime.AppI implementation


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
