package app

import (
	"os"
	"path"

	"cosmossdk.io/math"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkaddress "github.com/cosmos/cosmos-sdk/types/address"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const (
	Bech32Prefix = "kita"
	AppName      = "dns-shop"
	DefaultDenom = "kita"
	StateFolder  = ".private"
)

var (
	Bech32PrefixAccAddr  = Bech32Prefix
	Bech32PrefixAccPub   = Bech32Prefix + "pub"
	Bech32PrefixValAddr  = Bech32Prefix + "valoper"
	Bech32PrefixValPub   = Bech32Prefix + "valoperpub"
	Bech32PrefixConsAddr = Bech32Prefix + "valcons"
	Bech32PrefixConsPub  = Bech32Prefix + "valconspub"
)

var (
	DefaultNodeName string

	modulePermissions = map[string][]string{
		authtypes.FeeCollectorName: nil,
		distrtypes.ModuleName:      nil,
		stakingtypes.BondedPoolName: {
			authtypes.Burner,
			authtypes.Staking,
		},
		stakingtypes.NotBondedPoolName: {
			authtypes.Burner,
			authtypes.Staking,
		},
		govtypes.ModuleName: {
			authtypes.Burner,
		},
	}
)

func init() {
	createAndSetBlockchainStateFolder()
	registerDenoms()
	setAddressPrefixes()
}

func createAndSetBlockchainStateFolder() error {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	DefaultNodeName = path.Join(wd, StateFolder)

	return nil
}

func registerDenoms() {
	if err := sdk.RegisterDenom(DefaultDenom, math.LegacyOneDec()); err != nil {
		log.Fatal().Err(err).Send()
	}
}

func setAddressPrefixes() {
	config := sdk.GetConfig()

	config.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)

	config.SetAddressVerifier(func(bytes []byte) error {
		if len(bytes) == 0 {
			return errors.Wrap(sdkerrors.ErrUnknownAddress, "addresses cannot be empty")
		}

		if len(bytes) > sdkaddress.MaxAddrLen {
			return errors.Wrapf(sdkerrors.ErrUnknownAddress, "address max length is %d, got %d", sdkaddress.MaxAddrLen, len(bytes))
		}

		if len(bytes) != 20 && len(bytes) != 32 {
			return errors.Wrapf(sdkerrors.ErrUnknownAddress, "address length must be 20 or 32 bytes, got %d", len(bytes))
		}

		return nil
	})

}
