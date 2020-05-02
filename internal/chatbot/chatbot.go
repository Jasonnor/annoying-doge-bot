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
