package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"net/http"
	"net/url"
	"path"
)

type loginData struct {
	AuthToken string `json:"authToken"`
	UserId    string `json:"userId"`
}

type apiResult struct {
	Status string    `json:"status"`
	Data   loginData `json:"data"`
}

func postAPI(url string, data url.Values, target interface{}) error {
	response, err := http.PostForm(url, data)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return json.NewDecoder(response.Body).Decode(target)
}

func main() {
	// Get settings
	viper.SetConfigName("setting")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
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

	// Login
	loginUrl, err := url.Parse(chatUrl)
	loginUrl.Path = path.Join(loginUrl.Path, "/api/v1/login")
	loginUrlString := loginUrl.String()
	loginResponse := new(apiResult)
	err = postAPI(
		loginUrlString,
		url.Values{"user": {chatUser}, "password": {chatPwd}},
		loginResponse)
	if err != nil {
		panic(fmt.Errorf("Fatal error login by http post: %s \n", err))
	}
	fmt.Println(loginResponse.Data)
}
