package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/citizenwallet/indexer/internal/storage"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/joho/godotenv"
	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	RPCChainName          string
	RPCURL                string `env:"RPC_URL,default=http://localhost:8545"`
	RPCWSURL              string `env:"RPC_WS_URL,default=ws://localhost:8545"`
	EntryPointAddress     string
	AccountFactoryAddress string
	ProfileAddress        string
	APIKEY                string
	SentryURL             string `env:"SENTRY_URL"`
	PinataBaseURL         string `env:"PINATA_BASE_URL"`
	PinataAPIKey          string `env:"PINATA_API_KEY"`
	PinataAPISecret       string `env:"PINATA_API_SECRET"`
	DiscordURL            string `env:"DISCORD_URL,required"`
	DBSecret              string `env:"DB_SECRET,required"`
}

func New(ctx context.Context, envpath, confpath string) (*Config, error) {
	if envpath != "" {
		log.Default().Println("loading env from file: ", envpath)
		err := godotenv.Load(envpath)
		if err != nil {
			return nil, err
		}
	}

	cfg := &Config{}
	err := envconfig.Process(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// does community.json exist
	exists := storage.Exists(fmt.Sprintf("%s/community.json", confpath))
	if !exists {
		return nil, fmt.Errorf("community.json not found")
	}

	// parse community.json
	b, err := storage.Read(fmt.Sprintf("%s/community.json", confpath))
	if err != nil {
		return nil, err
	}

	commconf := &indexer.CommunityConfig{}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		return nil, err
	}

	// set config values
	cfg.RPCChainName = commconf.Community.Name
	cfg.EntryPointAddress = commconf.ERC4337.EntrypointAddress
	cfg.AccountFactoryAddress = commconf.ERC4337.AccountFactoryAddress
	cfg.ProfileAddress = commconf.Profile.Address
	cfg.APIKEY = commconf.Indexer.Key

	return cfg, nil
}
