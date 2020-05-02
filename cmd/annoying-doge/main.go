package main

import (
	"annoying-doge-bot/internal/chatbot"
	"fmt"
)

func main() {
	// Get settings and init chat bot
	bot, initErr := chatbot.InitChatBot()
	if initErr != nil {
		panic(fmt.Errorf("Fatal error init chat bot: %s \n", initErr))
	}
	loginErr := bot.Login()
	if loginErr != nil {
		panic(fmt.Errorf("Fatal error login by http post: %s \n", loginErr))
	}
	bot.ReplyMeme()
}
