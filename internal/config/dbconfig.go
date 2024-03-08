package config

import (
	"context"
	"log"

	"github.com/joho/godotenv"
	"github.com/sethvargo/go-envconfig"
)

type DBConfig struct {
	DBUser       string `env:"DB_USER,required"`
	DBPassword   string `env:"DB_PASSWORD,required"`
	DBName       string `env:"DB_NAME,required"`
	DBHost       string `env:"DB_HOST,required"`
	DBReaderHost string `env:"DB_READER_HOST,required"`
	DBSecret     string `env:"DB_SECRET,required"`
}

func NewDBConfig(ctx context.Context, envpath string) (*DBConfig, error) {
	if envpath != "" {
		log.Default().Println("loading env from file: ", envpath)
		err := godotenv.Load(envpath)
		if err != nil {
			return nil, err
		}
	}

	cfg := &DBConfig{}
	err := envconfig.Process(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
