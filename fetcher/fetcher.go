package fetcher

import (
	"arbTracker/configLoad"
	"arbTracker/dexScreener"
	"arbTracker/types"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gagliardetto/solana-go"
)

// fetcher.go
func FetchTradeConfigForMint(hotMint types.HotMints, cuLimit uint64, poolLiqFilter float64, config configLoad.Config, ctx context.Context, ata solana.PublicKey) (*types.TradeConfig, error) {
	mktList := dexScreener.QueryDexScreener(hotMint.TokenCA, 0, poolLiqFilter, 0)

	// raydium, _ := FetchRaydiumPools(mktList.)
	// raydiumCP, _ := FetchRaydiumCPPools(mint)
	if len(mktList.Meteora) > 0 && len(mktList.Pumpswap) > 0 {
		pump := FetchPumpPools(mktList.TokenCa, mktList.Pumpswap, config)
		fmt.Println("asfdasdf", mktList.Meteora[0])
		var temp []string
		temp = append(temp, mktList.Meteora[0])
		// temp = append(temp, mktList.Meteora[1])
		// temp = append(temp, mktList.Meteora[2])
		meteora := FetchMeteoraDLMM(mktList.TokenCa, temp, config, ctx)
		var raydium []types.RaydiumPool
		raydium = append(raydium, types.RaydiumPool{})
		var raydiumCP []types.RaydiumCPPool
		raydiumCP = append(raydiumCP, types.RaydiumCPPool{})

		// fmt.Println(meteora)

		return &types.TradeConfig{
			Mint:      mktList.TokenCa,
			CUPrice:   uint64(hotMint.CuPrice),
			CULimit:   cuLimit,
			Ata:       ata,
			Raydium:   raydium,
			RaydiumCP: raydiumCP,
			Pump:      pump,
			DLMM:      meteora,
		}, nil
	} else {
		return &types.TradeConfig{}, errors.New("At least one pool needs to be available")
	}
}

func FetchPumpPools(tokenCA string, mktList []string, config configLoad.Config) []types.PumpPool {
	var returnPump []types.PumpPool

	for _, x := range mktList {
		pool, xAccount, sOLAccount, _ := getPoolTokens(x, tokenCA, config)
		tempPump := types.PumpPool{
			ProgramId:      solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA"),  // Pump program ID
			GlobalConfig:   solana.MustPublicKeyFromBase58("ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw"), // Pump global config
			Authority:      solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR"), // Pump authority
			FeeWallet:      solana.MustPublicKeyFromBase58("JCRGumoE9Qi5BBgULTgdgTLjSgkCMSbF62ZZfGs84JeU"), // Pump fee wallet
			Pool:           solana.MustPublicKeyFromBase58(pool),
			XAccount:       solana.MustPublicKeyFromBase58(xAccount),
			SOLAccount:     solana.MustPublicKeyFromBase58(sOLAccount),
			FeeTokenWallet: solana.MustPublicKeyFromBase58("DWpvfqzGWuVy9jVSKSShdM2733nrEsnnhsUStYbkj6Nn"),
		}
		returnPump = append(returnPump, tempPump)
	}

	return returnPump
}

func getPoolTokens(poolAddress string, token_ca string, config configLoad.Config) (string, string, string, error) {
	url := config.RPCEndpoint

	payload := strings.NewReader(fmt.Sprintf(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "getTokenAccountsByOwner",
		"params": [
			"%s",
			{"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"},
			{"encoding": "jsonParsed"}
		]
	}`, poolAddress))

	resp, err := http.Post(url, "application/json", payload)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to query token accounts: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read response: %v", err)
	}
	// fmt.Println(string(body))

	var result struct {
		Result struct {
			Value []struct {
				Pubkey  string `json:"pubkey"`
				Account struct {
					Data struct {
						Parsed struct {
							Info struct {
								Mint string `json:"mint"`
							} `json:"info"`
						} `json:"parsed"`
					} `json:"data"`
				} `json:"account"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", "", fmt.Errorf("failed to parse response: %v", err)
	}

	var outputToken string
	var outputSol string
	for _, account := range result.Result.Value {
		if account.Account.Data.Parsed.Info.Mint == token_ca {
			outputToken = account.Pubkey
		}
		if account.Account.Data.Parsed.Info.Mint == "So11111111111111111111111111111111111111112" {
			outputSol = account.Pubkey
		}

	}
	// if len(tokens) > 0 {
	// 	tokens = append(tokens, poolAddress)
	// }
	return poolAddress, outputToken, outputSol, nil
}
