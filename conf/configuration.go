package conf

import (
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Config the application's configuration
type Config struct {
	Port           int           `mapstructure:"port" json:"port"`
	JWTSecret      string        `mapstructure:"jwt_secret" json:"jwt_secret"`
	AdminGroupName string        `mapstructure:"admin_group_name" json:"admin_group_name"`
	StripeKey      string        `mapstructure:"stripe_key" json:"stripe_key"`
	LogConfig      LoggingConfig `mapstructure:"log" json:"log"`
	DBConfig       DBConfig      `mapstructure:"db" json:"db"`
}

type DBConfig struct {
	Driver      string `mapstructure:"driver" json:"driver"`
	ConnURL     string `mapstructure:"url" json:"url"`
	Namespace   string `mapstructure:"namespace" json:"namespace"`
	Automigrate bool   `mapstructure:"automigrate" json:"automigrate"`
}

// LoadConfig loads the config from a file if specified, otherwise from the environment
func LoadConfig(cmd *cobra.Command) (*Config, error) {
	viper.SetConfigType("json")

	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		return nil, err
	}

	viper.SetEnvPrefix("GOJOIN")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if configFile, _ := cmd.Flags().GetString("config"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("./")
		viper.AddConfigPath("$HOME/.gojoin/")
	}

	if err := viper.ReadInConfig(); err != nil {
		_, ok := err.(viper.ConfigFileNotFoundError)
		if !ok {
			return nil, errors.Wrap(err, "reading configuration from files")
		}
	}

	config := new(Config)
	if err := viper.Unmarshal(config); err != nil {
		return nil, errors.Wrap(err, "unmarshaling configuration")
	}

	config, err = populateConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "populating config")
	}

	return validateConfig(config)
}

func validateConfig(config *Config) (*Config, error) {
	if config.DBConfig.ConnURL == "" && os.Getenv("DATABASE_URL") != "" {
		config.DBConfig.ConnURL = os.Getenv("DATABASE_URL")
	}

	if config.DBConfig.Driver == "" && config.DBConfig.ConnURL != "" {
		u, err := url.Parse(config.DBConfig.ConnURL)
		if err != nil {
			return nil, errors.Wrap(err, "parsing db connection url")
		}
		config.DBConfig.Driver = u.Scheme
	}

	if config.Port == 0 && os.Getenv("PORT") != "" {
		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err != nil {
			return nil, errors.Wrap(err, "formatting PORT into int")
		}

		config.Port = port
	}

	if config.Port == 0 {
		config.Port = 7070
	}

	return config, nil
}
