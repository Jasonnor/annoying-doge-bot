package main

import (
	"annoying-doge-bot/internal/chatbot"
	"fmt"
	"github.com/spf13/viper"
	"net/url"
	"path"
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
	fmt.Println(chatUrl, chatUser, chatPwd)
	botName := viper.GetString("chat_bot.display_name")
	botAvatarUrl := viper.GetString("chat_bot.avatar_url")
	botTargets := viper.GetStringSlice("chat_bot.target_channels")
	fmt.Println(botName, botAvatarUrl, botTargets)
	searchUrl := viper.GetString("google_search.url")
	searchCx := viper.GetString("google_search.cx")
	searchKey := viper.GetString("google_search.api_key")
	fmt.Println(searchUrl, searchCx, searchKey)

	// Login
	loginUrl, err := url.Parse(chatUrl)
	loginUrl.Path = path.Join(loginUrl.Path, "/api/v1/login")
	loginUrlString := loginUrl.String()
	loginResponse := new(chatbot.LoginResult)
	loginHeader := chatbot.LoginData{}
	loginJson := []byte(
		fmt.Sprintf(
			`{"user": "%s", "password": "%s"}`,
			chatUser,
			chatPwd))
	err = chatbot.PostAPI(
		loginUrlString,
		loginJson,
		loginHeader,
		loginResponse)
	if err != nil {
		panic(fmt.Errorf("Fatal error login by http post: %s \n", err))
	}
	loginHeader = loginResponse.Data
	fmt.Println(loginResponse.Data)

	// Get messages from target channels
	channelsMsgUrl, err := url.Parse(chatUrl)
	channelsMsgUrl.Path = path.Join(channelsMsgUrl.Path, "/api/v1/channels.messages")
	channelsMsgUrlString := channelsMsgUrl.String()
	for _, botTarget := range botTargets {
		channelsMsgResponse := new(chatbot.ChannelsMsgResult)
		queries := map[string]string{
			"roomName": botTarget,
			"count":    "5",
		}
		err = chatbot.GetAPI(
			channelsMsgUrlString,
			queries,
			loginHeader,
			channelsMsgResponse)
		if err != nil {
			panic(fmt.Errorf("Fatal error get messages by http get: %s \n", err))
		}
		fmt.Println(channelsMsgResponse)

		searchText := channelsMsgResponse.Messages[0].Msg + " 梗圖 | 迷因"
		searchResponse := new(chatbot.SearchResult)
		searchQueries := map[string]string{
			"q":          searchText,
			"cx":         searchCx,
			"key":        searchKey,
			"num":        "10",
			"searchType": "image",
		}
		err = chatbot.GetAPI(
			searchUrl,
			searchQueries,
			chatbot.LoginData{},
			searchResponse)
		if err != nil {
			panic(fmt.Errorf("Fatal error search by http get: %s \n", err))
		}
		fmt.Println(searchResponse)

		// Replay message a meme
		message := "@" + channelsMsgResponse.Messages[0].User.Name
		chatbot.PostMsg(
			chatUrl,
			botTarget,
			botName,
			botAvatarUrl,
			loginHeader,
			message,
			searchResponse.Items[0].Link)
	}
}

func initViper() error {
	viper.SetConfigName("setting")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	err := viper.ReadInConfig()
	return err
}
