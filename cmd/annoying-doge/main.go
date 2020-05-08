package main

import (
	"annoying-doge-bot/internal/daemon"
	"fmt"
	"github.com/spf13/viper"
)

func main() {
	// Load config
	viper.SetConfigName("setting")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error loading config: %s \n", err))
	}
	fmt.Printf(
		"[INFO] Get config from %s successfully\n",
		viper.ConfigFileUsed())
	// Init watchdog
	dog := daemon.NewWatchDog()
	dog.Run()
}
