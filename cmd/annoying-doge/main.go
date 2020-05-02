package main

import (
	"annoying-doge-bot/internal/chatbot"
	"fmt"
	"github.com/spf13/viper"
)

func main() {
	// Get settings
	err := initViper()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	chatUrl := viper.GetString("rocket_chat.url")
	chatUser := viper.GetString("rocket_chat.user_name")
	chatPwd := viper.GetString("rocket_chat.password")
	botName := viper.GetString("chat_bot.display_name")
	botAvatarUrl := viper.GetString("chat_bot.avatar_url")
	botTargets := viper.GetStringSlice("chat_bot.target_channels")
	searchUrl := viper.GetString("google_search.url")
	searchCx := viper.GetString("google_search.cx")
	searchKey := viper.GetString("google_search.api_key")
	fmt.Printf(
		"[INFO] Get config from %s successfully\n",
		viper.ConfigFileUsed())

	// Login
	loginHeader, err := chatbot.Login(chatUrl, chatUser, chatPwd)
	if err != nil {
		panic(fmt.Errorf("Fatal error login by http post: %s \n", err))
	}

	// Get messages from target channels
	chatbot.ReplyMeme(chatUrl, botTargets, loginHeader, searchCx, searchKey, searchUrl, botName, botAvatarUrl)
}

func initViper() error {
	viper.SetConfigName("setting")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	err := viper.ReadInConfig()
	return err
}
