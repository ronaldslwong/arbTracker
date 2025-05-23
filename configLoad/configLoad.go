package configLoad

import (
	"fmt"
	"log"

	"github.com/BurntSushi/toml"
)

// Config structure that matches the TOML fields
type Config struct {
	GRPCEndpoint          string   `toml:"grpcEndpoint"`
	RPCEndpoint           string   `toml:"rpcEndpoint"`
	SendRPC               []string `toml:sendRPC`
	BufferSize            int      `toml:"bufferSize"`
	NumWorkers            int      `toml:"numWorkers"`
	WindowSeconds         int      `toml:"windowSeconds"`
	CuPricePercentile     float64  `toml:"cuPricePercentile"`
	CheckInterval         int      `toml:"checkInterval"`
	TotVolumeFilter       float64  `toml:totalVolumeFilter`
	PoolLiqFilter         float64  `toml:"poolLiqFilter"`
	BinsToSearch          int      `toml:"binsToSearch"`
	LoopInterval          int      `toml:loopInterval`
	NumArbsFilter         int      `toml:numArbsFilter`
	AccountsMonitor       []string `toml:accountsMonitor`
	CuLimit               int      `toml:cuLimit`
	MaxCUPrice            int      `toml:maxCUPrice`
	MintsIgnore           []string `toml:mintsIgnore`
	DynamicLoopInterval   int      `toml:dynamicLoopInterval`
	TargetMinLandingRate  float64  `toml:targetMinLandingRate`
	TargetMaxLandingRate  float64  `toml:targetMaxLandingRate`
	PriceAdjustmentFactor float64  `toml:priceAdjustmentFactor`
	TrackWallet           string   `toml:trackWallet`
	SlotsToCheck          int      `toml:slotsToCheck`
	AltAddress            string   `toml:altAddress`
	MeteoraMkts           int      `toml:meteoraMkts`
	PumpMkts              int      `toml:pumpMkts`
	ShowTx                bool     `toml:showTx`
	ObservationWindow     int      `toml:observationWindow`
	BinMovesInWindow      int      `toml:binMovesInWindow`
	RayCpmm               int      `toml:rayCpmm`
	MeteoraAmm            int      `toml:meteoraAmm`
}

// LoadConfig reads from the TOML file and returns a Config struct
func LoadConfig(path string) Config {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	fmt.Println("Config Loaded:", cfg)
	return cfg
}
