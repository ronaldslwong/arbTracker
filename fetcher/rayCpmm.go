package fetcher

import (
	"arbTracker/configLoad"
	"arbTracker/globals"
	"arbTracker/types"
	"context"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const (
	padding          = 8
	configOffset     = 0 + padding
	xVaultOffset     = 96 + padding
	solVaultOffset   = 64 + padding
	obsevationOffset = 288 + padding
)

func FetchRayCpmmPools(tokenCA string, mktList []string, config configLoad.Config) []types.RaydiumCPPool {
	var returnRay []types.RaydiumCPPool

	for numMkts, x := range mktList {
		if numMkts < config.RayCpmm {
			data, _ := GetRayMarketAccountData(solana.MustPublicKeyFromBase58(x))
			// pool, xAccount, sOLAccount, _ := getPoolTokens(x, tokenCA, config)
			// fmt.Println(DeriveAmmConfigPDA(solana.MustPublicKeyFromBase58(pool)))
			tempRay := types.RaydiumCPPool{
				RayProgramId:      solana.MustPublicKeyFromBase58("CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C"), // Pump program ID
				RayEventAuthority: solana.MustPublicKeyFromBase58("GpMZbSM2GgvTKHJirzeGfMFoaZ8UR2X7F4v8vHTvxFbL"), // Pump global config
				Pool:              solana.MustPublicKeyFromBase58(x),
				AmmConfig:         ExtractCpmmData(data, configOffset),
				XVault:            ExtractCpmmData(data, xVaultOffset),
				SOLVault:          ExtractCpmmData(data, solVaultOffset),
				Observation:       ExtractCpmmData(data, obsevationOffset),
			}

			// fmt.Println("config", tempRay.AmmConfig)
			// fmt.Println("XVault", tempRay.XVault)
			// fmt.Println("SOLVault", tempRay.SOLVault)
			// fmt.Println("Observation", tempRay.Observation)
			returnRay = append(returnRay, tempRay)
		}
	}

	return returnRay
}

func GetRayMarketAccountData(marketPDA solana.PublicKey) ([]byte, error) {
	info, err := globals.RPCClient.GetAccountInfoWithOpts(
		context.TODO(),
		marketPDA,
		&rpc.GetAccountInfoOpts{
			Encoding: "base64",
		},
	)
	if err != nil {
		return nil, err
	}

	rawData := info.Value.Data.GetBinary()

	return rawData, nil
}

func ExtractCpmmData(data []byte, offset int) solana.PublicKey {
	if len(data) < 259 {
		return solana.PublicKey{}
	}
	return solana.PublicKeyFromBytes(data[offset : offset+32])
}
