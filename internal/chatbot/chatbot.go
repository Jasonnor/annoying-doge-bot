package chatbot

import (
	"fmt"
	"github.com/spf13/viper"
	"math/rand"
	"net/url"
	"path"
	"strings"
	"time"
)

type ChatBot struct {
	chatUrl, chatUser, chatPwd     string
	name, avatarUrl                string
	targets                        []string
	alternativeRules               map[string]string
	searchUrl, searchCx, searchKey string
	loginHeader                    LoginData
	messageBlackMap                map[string]bool
}

func New() ChatBot {
	bot := ChatBot{
		chatUrl:          viper.GetString("rocket_chat.url"),
		chatUser:         viper.GetString("rocket_chat.user_name"),
		chatPwd:          viper.GetString("rocket_chat.password"),
		name:             viper.GetString("chat_bot.display_name"),
		avatarUrl:        viper.GetString("chat_bot.avatar_url"),
		targets:          viper.GetStringSlice("chat_bot.target_channels"),
		alternativeRules: viper.GetStringMapString("chat_bot.alternative_rules"),
		searchUrl:        viper.GetString("google_search.url"),
		searchCx:         viper.GetString("google_search.cx"),
		searchKey:        viper.GetString("google_search.api_key"),
		messageBlackMap:  make(map[string]bool),
	}
	return bot
}

func (bot *ChatBot) Login() error {
	loginUrl, err := url.Parse(bot.chatUrl)
	if err != nil {
		return err
	}
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
	if err != nil {
		return err
	}
	bot.loginHeader = loginResponse.Data
	fmt.Printf("[INFO] Login user %s successfully\n", bot.chatUser)
	return err
}

func (bot ChatBot) PostMsg(
	botTarget string,
	message string,
	imageUrl string) error {
	// Send text to target channels
	postMsgUrl, err := url.Parse(bot.chatUrl)
	if err != nil {
		return err
	}
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
			bot.name,
			bot.avatarUrl,
			imageUrl))
	err = PostAPI(
		postMsgUrlString,
		postMsgJson,
		bot.loginHeader,
		postMsgResponse)
	if err != nil {
		return err
	}
	fmt.Println("[INFO] Post message successfully")
	return err
}

func (bot *ChatBot) ReplyMeme() error {
	channelsMsgUrl, err := url.Parse(bot.chatUrl)
	if err != nil {
		return err
	}
	channelsMsgUrl.Path = path.Join(channelsMsgUrl.Path, "/api/v1/channels.messages")
	channelsMsgUrlString := channelsMsgUrl.String()
	for _, botTarget := range bot.targets {
		// Get messages from target channel
		channelsMsgResponse := new(ChannelsMsgResult)
		queries := map[string]string{
			"roomName": botTarget,
			"count":    "5",
		}
		err := GetAPI(
			channelsMsgUrlString,
			queries,
			bot.loginHeader,
			channelsMsgResponse)
		if err != nil {
			return err
		}
		fmt.Printf(
			"[INFO] Get messages from target channel %s successfully, total: %d\n",
			botTarget,
			channelsMsgResponse.Total)
		targetMessage := channelsMsgResponse.Messages[0]
		fmt.Printf("[DEBUG] Target message: %+v\n", targetMessage)
		if targetMessage.Alias == bot.name {
			fmt.Println("[INFO] No new message, skip")
			continue
		}

		// Check message in black list
		if bot.messageBlackMap[targetMessage.Msg] {
			fmt.Printf(
				"[INFO] Get message %s which is in black list, skip\n",
				targetMessage.Msg)
			continue
		}

		// Replace message by alternative rules
		searchString := targetMessage.Msg
		for originMsg, altMsg := range bot.alternativeRules {
			if strings.Contains(searchString, originMsg) {
				fmt.Printf(
					"[INFO] Match alternative rule, replace %s to %s\n",
					searchString, altMsg)
				searchString = altMsg
				break
			}
		}

		// Search memes by message
		searchText := `"` + searchString + `" 梗圖 | 迷因`
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
			return err
		}
		memeLength := len(searchResponse.Items)
		fmt.Printf(
			"[INFO] Search memes successfully, total: %d\n",
			memeLength)
		if memeLength == 0 {
			fmt.Printf(
				"[WARNING] No meme to show, add %s to black list and skip\n",
				targetMessage.Msg)
			bot.messageBlackMap[targetMessage.Msg] = true
			continue
		}

		// Randomly choose a meme
		rand.Seed(time.Now().UnixNano())
		randomMeme := searchResponse.Items[rand.Intn(memeLength)]
		fmt.Printf("[DEBUG] Target meme: %+v\n", randomMeme)

		// Replay message a meme
		message := "@" + targetMessage.User.Name
		err = bot.PostMsg(
			botTarget,
			message,
			randomMeme.Link)
		if err != nil {
			return err
		}
	}
	return err
}
