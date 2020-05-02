package chatbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
)

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

func PostAPI(url string, jsonStr []byte, header LoginData, target interface{}) error {
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

func GetAPI(url string, queries map[string]string, header LoginData, target interface{}) error {
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

func Login(
	chatUrl string,
	chatUser string,
	chatPwd string) (LoginData, error) {
	loginUrl, err := url.Parse(chatUrl)
	loginUrl.Path = path.Join(loginUrl.Path, "/api/v1/login")
	loginUrlString := loginUrl.String()
	loginResponse := new(LoginResult)
	loginHeader := LoginData{}
	loginJson := []byte(
		fmt.Sprintf(
			`{"user": "%s", "password": "%s"}`,
			chatUser,
			chatPwd))
	err = PostAPI(
		loginUrlString,
		loginJson,
		loginHeader,
		loginResponse)
	loginHeader = loginResponse.Data
	fmt.Printf("[INFO] Login user %s successfully\n", chatUser)
	return loginHeader, err
}

func PostMsg(
	chatUrl string,
	botTarget string,
	botName string,
	botAvatarUrl string,
	loginHeader LoginData,
	message string,
	imageUrl string) {
	// Send text to target channels
	postMsgUrl, err := url.Parse(chatUrl)
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
			botName,
			botAvatarUrl,
			imageUrl))
	err = PostAPI(
		postMsgUrlString,
		postMsgJson,
		loginHeader,
		postMsgResponse)
	if err != nil {
		panic(fmt.Errorf("Fatal error post message by http post: %s \n", err))
	}
	fmt.Println("[INFO] Post message successfully")
}

func ReplyMeme(
	chatUrl string,
	botTargets []string,
	loginHeader LoginData,
	searchCx string,
	searchKey string,
	searchUrl string,
	botName string,
	botAvatarUrl string) {
	channelsMsgUrl, err := url.Parse(chatUrl)
	channelsMsgUrl.Path = path.Join(channelsMsgUrl.Path, "/api/v1/channels.messages")
	channelsMsgUrlString := channelsMsgUrl.String()
	for _, botTarget := range botTargets {
		channelsMsgResponse := new(ChannelsMsgResult)
		queries := map[string]string{
			"roomName": botTarget,
			"count":    "5",
		}
		err = GetAPI(
			channelsMsgUrlString,
			queries,
			loginHeader,
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
			"cx":         searchCx,
			"key":        searchKey,
			"num":        "10",
			"searchType": "image",
		}
		err = GetAPI(
			searchUrl,
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
		PostMsg(
			chatUrl,
			botTarget,
			botName,
			botAvatarUrl,
			loginHeader,
			message,
			searchResponse.Items[0].Link)
	}
}
