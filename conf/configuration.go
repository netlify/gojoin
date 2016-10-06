package conf

import (
	"net/url"
	"os"
	"strconv"
	"strings"

	//"github.com/benoitmasson/viper"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Config the application's configuration
type Config struct {
	Port           int64         `mapstructure:"port" json:"port"`
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

	viper.SetEnvPrefix("NETLIFY_SUBSCRIPTIONS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if configFile, _ := cmd.Flags().GetString("config"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("./")
		viper.AddConfigPath("$HOME/.netlify-subscriptions/")
	}

	if err := viper.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "reading configuration from files")
	}

	config := new(Config)
	if err := viper.Unmarshal(config); err != nil {
		return nil, errors.Wrap(err, "unmarshaling configuration")
	}

	if err := populateConfig(config); err != nil {
		return nil, errors.Wrap(err, "populating config")
	}

	return validateConfig(config)
}

func validateConfig(config *Config) (*Configuration, error) {
	if config.DB.ConnURL == "" && os.Getenv("DATABASE_URL") != "" {
		config.DB.ConnURL = os.Getenv("DATABASE_URL")
	}

	if config.DB.Driver == "" && config.DB.ConnURL != "" {
		u, err := url.Parse(config.DB.ConnURL)
		if err != nil {
			return nil, errors.Wrap(err, "parsing db connection url")
		}
		config.DB.Driver = u.Scheme
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