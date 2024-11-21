package app

import (
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
)

func (app *App) ExportAppStateAndValidators(forZeroHeight bool, jailAllowedAddrs, modulesToExport []string) (servertypes.ExportedApp, error) {
	ctx := app.NewContextLegacy(true, cmtproto.Header{
		Height: app.LastBlockHeight(),
	})


	height := app.LastBlockHeight() + 1




	return servertypes.ExportedApp{}, nil
}
