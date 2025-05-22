package fetcher

import (
	"arbTracker/configLoad"
	"arbTracker/types"

	"github.com/gagliardetto/solana-go"
)

const (
	paddingMeteoraAmm = 8
	XVault            = 3*32 + paddingMeteoraAmm
	SOLVault          = 4*32 + paddingMeteoraAmm
	XTokenVault       = 11 + paddingMeteoraAmm
	XLpMint           = XTokenVault + 96
	XPoolLp           = 5*32 + paddingMeteoraAmm
	SOLPoolLp         = 6*32 + paddingMeteoraAmm
	AdminTokenFeeX    = 7*32 + 2 + paddingMeteoraAmm
	AdminTokenFeeSOL  = 7*32 + 34 + paddingMeteoraAmm
	// AdminTokenFeeX =
	// AdminTokenFeeSOL =
)

func FetchMeteoraAMM(tokenCA string, mktList []string, config configLoad.Config) []types.AMMMeteora {
	var returnMeteora []types.AMMMeteora

	for numMkts, x := range mktList {
		if numMkts < config.MeteoraAmm {
			data, _ := GetMarketAccountData(solana.MustPublicKeyFromBase58(x))
			XVaultBase := ExtractVaultData(data, XVault)
			SOLVaultBase := ExtractVaultData(data, SOLVault)

			XVaultData, _ := GetMarketAccountData(XVaultBase)
			XTokenVaultData := ExtractVaultData(XVaultData, XTokenVault)
			XLpMintData := ExtractVaultData(XVaultData, XLpMint)

			SOLVaultData, _ := GetMarketAccountData(SOLVaultBase)
			SOLTokenVaultData := ExtractVaultData(SOLVaultData, XTokenVault)
			SOLLpMintData := ExtractVaultData(SOLVaultData, XLpMint)

			// fmt.Println("XVaultBase", XVaultBase)
			// fmt.Println("SOLVaultBase", SOLVaultBase)
			// fmt.Println("XTokenVaultData", XTokenVaultData)
			// fmt.Println("XLpMintData", XLpMintData)
			// fmt.Println("SOLTokenVaultData", SOLTokenVaultData)
			// fmt.Println("SOLLpMintData", SOLLpMintData)

			// XPoolLpData, _ := GetTokenHolders(XLpMintData)
			XPoolLpData := ExtractVaultData(data, XPoolLp)
			SOLPoolLpData := ExtractVaultData(data, SOLPoolLp)

			// fmt.Println("XPoolLpData", XPoolLpData)
			// fmt.Println("SOLPoolLpData", SOLPoolLpData)

			AdminTokenFeeXData := ExtractVaultData(data, AdminTokenFeeX)
			AdminTokenFeeSOLData := ExtractVaultData(data, AdminTokenFeeSOL)

			// fmt.Println("AdminTokenFeeXData", AdminTokenFeeXData)
			// fmt.Println("AdminTokenFeeSOLData", AdminTokenFeeSOLData)

			returnSingleMeteora := types.AMMMeteora{
				AMMProgramId:     solana.MustPublicKeyFromBase58("Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EQVn5UaB"), // DLMM program ID
				AMMVault:         solana.MustPublicKeyFromBase58("24Uqj9JCLxUeoC3hGfh5W3s9FM9uCHDS2SG3LYwBpyTi"),
				Pool:             solana.MustPublicKeyFromBase58(x),
				XVault:           XVaultBase,
				SOLVault:         SOLVaultBase,
				XTokenVault:      XTokenVaultData,
				SOLTokenVault:    SOLTokenVaultData,
				XLpMint:          XLpMintData,
				SOLLpMint:        SOLLpMintData,
				XPoolLp:          XPoolLpData,
				SOLPoolLp:        SOLPoolLpData,
				AdminTokenFeeX:   AdminTokenFeeXData,
				AdminTokenFeeSOL: AdminTokenFeeSOLData,
			}
			returnMeteora = append(returnMeteora, returnSingleMeteora)
		}

	}

	//subscribe to new bin feed

	return returnMeteora
}
