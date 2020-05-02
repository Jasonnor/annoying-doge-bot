package chatbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"net/http"
	"net/url"
	"path"
)

type ChatBot struct {
	chatUrl, chatUser, chatPwd     string
	botName, botAvatarUrl          string
	botTargets                     []string
	searchUrl, searchCx, searchKey string
	loginHeader                    LoginData
}

type LoginData struct {
	AuthToken string `json:"authToken" default:""`
	UserId    string `json:"userId" default:""`
}

type LoginResult struct {
	Status string    `json:"status"`
	Data   LoginData `json:"data"`
}

type User struct {
	Id       string `json:"_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type Message struct {
	Id   string `json:"_id"`
	Msg  string `json:"msg"`
	User User   `json:"u"`
}

type ChannelsMsgResult struct {
	Success  bool      `json:"success"`
	Messages []Message `json:"messages"`
	Total    int       `json:"total"`
}

// See: https://developers.google.com/custom-search/v1/reference/rest/v1/Search
type SearchItem struct {
	Title string `json:"title"`
	Link  string `json:"link"`
}

type SearchResult struct {
	Items []SearchItem `json:"items"`
}

type PostMsgResult struct {
	Success bool   `json:"success"`
	Channel string `json:"channel"`
}

func InitChatBot() (ChatBot, error) {
	viper.SetConfigName("setting")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	err := viper.ReadInConfig()
	fmt.Printf(
		"[INFO] Get config from %s successfully\n",
		viper.ConfigFileUsed())
	bot := ChatBot{
		chatUrl:      viper.GetString("rocket_chat.url"),
		chatUser:     viper.GetString("rocket_chat.user_name"),
		chatPwd:      viper.GetString("rocket_chat.password"),
		botName:      viper.GetString("chat_bot.display_name"),
		botAvatarUrl: viper.GetString("chat_bot.avatar_url"),
		botTargets:   viper.GetStringSlice("chat_bot.target_channels"),
		searchUrl:    viper.GetString("google_search.url"),
		searchCx:     viper.GetString("google_search.cx"),
		searchKey:    viper.GetString("google_search.api_key"),
	}
	return bot, err
}

func PostAPI(
	url string,
	jsonStr []byte,
	header LoginData,
	target interface{}) error {
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
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

func GetAPI(
	url string,
	queries map[string]string,
	header LoginData,
	target interface{}) error {
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

func (bot *ChatBot) Login() error {
	loginUrl, err := url.Parse(bot.chatUrl)
	loginUrl.Path = path.Join(loginUrl.Path, "/api/v1/login")
	loginUrlString := loginUrl.String()
	loginResponse := new(LoginResult)
	loginHeader := LoginData{}
	loginJson := []byte(
		fmt.Sprintf(
			`{"user": "%s", "password": "%s"}`,
			bot.chatUser,
			bot.chatPwd))
	err = PostAPI(
		loginUrlString,
		loginJson,
		loginHeader,
		loginResponse)
	bot.loginHeader = loginResponse.Data
	fmt.Printf("[INFO] Login user %s successfully\n", bot.chatUser)
	return err
}

func (bot ChatBot) PostMsg(
	botTarget string,
	message string,
	imageUrl string) {
	// Send text to target channels
	postMsgUrl, err := url.Parse(bot.chatUrl)
	postMsgUrl.Path = path.Join(postMsgUrl.Path, "/api/v1/chat.postMessage")
	postMsgUrlString := postMsgUrl.String()
	postMsgResponse := new(PostMsgResult)
	postMsgJson := []byte(
		fmt.Sprintf(
			`{"channel": "%s", 
				"text": "%s", 
				"alias": "%s", 
				"avatar": "%s", 
				"attachments": [{"image_url": "%s"}]}`,
			botTarget,
			message,
			bot.botName,
			bot.botAvatarUrl,
			imageUrl))
	err = PostAPI(
		postMsgUrlString,
		postMsgJson,
		bot.loginHeader,
		postMsgResponse)
	if err != nil {
		panic(fmt.Errorf("Fatal error post message by http post: %s \n", err))
	}
	fmt.Println("[INFO] Post message successfully")
}

func (bot ChatBot) ReplyMeme() {
	channelsMsgUrl, err := url.Parse(bot.chatUrl)
	channelsMsgUrl.Path = path.Join(channelsMsgUrl.Path, "/api/v1/channels.messages")
	channelsMsgUrlString := channelsMsgUrl.String()
	for _, botTarget := range bot.botTargets {
		channelsMsgResponse := new(ChannelsMsgResult)
		queries := map[string]string{
			"roomName": botTarget,
			"count":    "5",
		}
		err = GetAPI(
			channelsMsgUrlString,
			queries,
			bot.loginHeader,
			channelsMsgResponse)
		if err != nil {
			panic(fmt.Errorf("Fatal error get messages by http get: %s \n", err))
		}
		fmt.Printf(
			"[INFO] Get messages from target channel %s successfully, total: %d\n",
			botTarget,
			channelsMsgResponse.Total)

		searchText := channelsMsgResponse.Messages[0].Msg + " 梗圖 | 迷因"
		searchResponse := new(SearchResult)
		searchQueries := map[string]string{
			"q":          searchText,
			"cx":         bot.searchCx,
			"key":        bot.searchKey,
			"num":        "10",
			"searchType": "image",
		}
		err = GetAPI(
			bot.searchUrl,
			searchQueries,
			LoginData{},
			searchResponse)
		if err != nil {
			panic(fmt.Errorf("Fatal error search by http get: %s \n", err))
		}
		fmt.Printf(
			"[INFO] Search memes successfully, total: %d\n",
			len(searchResponse.Items))
		fmt.Printf(
			"[DEBUG] Target meme: %+v\n",
			searchResponse.Items[0])

		// Replay message a meme
		message := "@" + channelsMsgResponse.Messages[0].User.Name
		bot.PostMsg(
			botTarget,
			message,
			searchResponse.Items[0].Link)
	}
}
