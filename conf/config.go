package conf

import (
	"strings"

	//"github.com/benoitmasson/viper"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Config the application's configuration
type Config struct {
	Port           int64         `json:"port"`
	JWTSecret      string        `json:"jwt_secret"`
	AdminGroupName string        `json:"admin_group_name"`
	StripeKey      string        `json:"stripe_key"`
	LogConfig      LoggingConfig `json:"log"`
	DBConfig       DBConfig      `json:"db"`
}

type DBConfig struct {
	Driver      string `json:"driver"`
	ConnURL     string `json:"url"`
	Namespace   string `json:"namespace"`
	Automigrate bool   `json:"automigrate"`
}

// LoadConfig loads the config from a file if specified, otherwise from the environment
func LoadConfig(cmd *cobra.Command) (*Config, error) {
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		return nil, err
	}

	viper.SetEnvPrefix("NETLIFY")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if configFile, _ := cmd.Flags().GetString("config"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("./")
		viper.AddConfigPath("$HOME/.example")
	}

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	return populateConfig(new(Config))
}
