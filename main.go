package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type loginData struct {
	AuthToken string `json:"authToken" default:""`
	UserId    string `json:"userId" default:""`
}

type loginResult struct {
	Status string    `json:"status"`
	Data   loginData `json:"data"`
}

type user struct {
	Id       string `json:"_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type message struct {
	Id   string `json:"_id"`
	Msg  string `json:"msg"`
	User user   `json:"u"`
}

type channelsMsgResult struct {
	Success  bool      `json:"success"`
	Messages []message `json:"messages"`
	Total    int       `json:"total"`
}

type postMsgResult struct {
	Success bool   `json:"success"`
	Channel string `json:"channel"`
}

func postAPI(url string, data url.Values, header loginData, target interface{}) error {
	client := &http.Client{}
	body := strings.NewReader(data.Encode())
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-User-Id", header.UserId)
	req.Header.Set("X-Auth-Token", header.AuthToken)
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			panic(fmt.Errorf("Fatal error close response body: %s \n", err))
		}
	}()
	return json.NewDecoder(response.Body).Decode(target)
}

func getAPI(url string, queries map[string]string, header loginData, target interface{}) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-User-Id", header.UserId)
	req.Header.Set("X-Auth-Token", header.AuthToken)
	query := req.URL.Query()
	for key, value := range queries {
		query.Add(key, value)
	}
	req.URL.RawQuery = query.Encode()
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			panic(fmt.Errorf("Fatal error close response body: %s \n", err))
		}
	}()
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
	searchUrl := viper.GetString("google_search.url")
	searchCx := viper.GetString("google_search.cx")
	searchKey := viper.GetString("google_search.api_key")
	fmt.Println(searchUrl, searchCx, searchKey)

	// Login
	loginUrl, err := url.Parse(chatUrl)
	loginUrl.Path = path.Join(loginUrl.Path, "/api/v1/login")
	loginUrlString := loginUrl.String()
	loginResponse := new(loginResult)
	loginHeader := loginData{}
	err = postAPI(
		loginUrlString,
		url.Values{"user": {chatUser}, "password": {chatPwd}},
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
		channelsMsgResponse := new(channelsMsgResult)
		queries := map[string]string{
			"roomName": botTarget,
			"count":    "5",
		}
		err = getAPI(
			channelsMsgUrlString,
			queries,
			loginHeader,
			channelsMsgResponse)
		if err != nil {
			panic(fmt.Errorf("Fatal error get messages by http get: %s \n", err))
		}
		fmt.Println(channelsMsgResponse)
	}

	//postMsg(
	// chatUrl, botTargets, botName, botAvatarUrl, loginHeader, 'test')
}

func postMsg(
	chatUrl string,
	botTargets []string,
	botName string,
	botAvatarUrl string,
	loginHeader loginData,
	message string) {
	// Send text to target channels
	postMsgUrl, err := url.Parse(chatUrl)
	postMsgUrl.Path = path.Join(postMsgUrl.Path, "/api/v1/chat.postMessage")
	postMsgUrlString := postMsgUrl.String()
	for _, botTarget := range botTargets {
		postMsgResponse := new(postMsgResult)
		err = postAPI(
			postMsgUrlString,
			url.Values{
				"channel": {botTarget},
				"text":    {message},
				"alias":   {botName},
				"avatar":  {botAvatarUrl}},
			loginHeader,
			postMsgResponse)
		if err != nil {
			panic(fmt.Errorf("Fatal error post message by http post: %s \n", err))
		}
		fmt.Println(postMsgResponse)
	}
}
