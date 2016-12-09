package cmd

import (
	"log"

	"os"

	"github.com/netlify/netlify-subscriptions/api"
	"github.com/netlify/netlify-subscriptions/conf"
	"github.com/netlify/netlify-subscriptions/models"
	"github.com/spf13/cobra"
	"github.com/stripe/stripe-go"
)

var rootCmd = cobra.Command{
	Run: run,
}

// RootCommand will setup and return the root command
func RootCommand() *cobra.Command {
	rootCmd.PersistentFlags().StringP("config", "c", "", "the config file to use")
	rootCmd.Flags().IntP("port", "p", 0, "the port to use")

	rootCmd.AddCommand(&versionCmd)

	return &rootCmd
}

func run(cmd *cobra.Command, args []string) {
	config, err := conf.LoadConfig(cmd)
	if err != nil {
		log.Fatal("Failed to load config: " + err.Error())
	}

	logger, err := conf.ConfigureLogging(&config.LogConfig)
	if err != nil {
		log.Fatal("Failed to configure logging: " + err.Error())
	}

	logger.Infof("Connecting to DB")
	db, err := models.Connect(&config.DBConfig)
	if err != nil {
		logger.Fatal("Failed to connect to db: " + err.Error())
	}

	logger.Info("Configuring stripe access")
	stripe.Key = config.StripeKey

	logger.Infof("Starting API on port %d", config.Port)
	a := api.NewAPI(config, db, &api.StripeProxy{}, Version)
	err = a.Serve()
	if err != nil {
		logger.WithError(err).Error("Error while running API: %v", err)
		os.Exit(1)
	}
	logger.Info("API Shutdown")
}
