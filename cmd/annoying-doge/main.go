package main

import (
	"annoying-doge-bot/internal/chatbot"
	"fmt"
	"github.com/spf13/viper"
)

func main() {
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
	// Get settings and init chat bot
	bot := chatbot.New()
	loginErr := bot.Login()
	if loginErr != nil {
		panic(fmt.Errorf("Fatal error login by http post: %s \n", loginErr))
	}
	replyErr := bot.ReplyMeme()
	if replyErr != nil {
		panic(fmt.Errorf("Fatal error chatbot reply meme: %s \n", replyErr))
	}
}
