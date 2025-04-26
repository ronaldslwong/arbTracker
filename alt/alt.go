package alt

import (
	"context"

	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table" // Updated import
	"github.com/gagliardetto/solana-go/rpc"
)

func LoadAlt(ctx context.Context, client *rpc.Client) (*addresslookuptable.AddressLookupTableState, error) {
	altAddress := solana.MustPublicKeyFromBase58("4sKLJ1Qoudh8PJyqBeuKocYdsZvxTcRShUt9aKqwhgvC")

	lookupTableAccount, err := addresslookuptable.GetAddressLookupTable(
		context.Background(),
		client,
		altAddress, // ALT public key you're using
	)

	return lookupTableAccount, err
}
