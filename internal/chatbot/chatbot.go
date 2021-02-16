package chatbot

import (
	"fmt"
	"github.com/spf13/viper"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type ChatBot struct {
	chatUrl, chatUser, chatPwd     string
	name, avatarUrl                string
	targets                        []string
	patternMatching                map[string]string
	alternativeRules               map[string]string
	searchUrl, searchCx, searchKey string
	loginHeader                    LoginData
	messageBlackMap                map[string]bool
	imageUrlBlackMap               map[string]bool
}

func New() ChatBot {
	bot := ChatBot{
		chatUrl:          viper.GetString("rocket_chat.url"),
		chatUser:         viper.GetString("rocket_chat.user_name"),
		chatPwd:          viper.GetString("rocket_chat.password"),
		name:             viper.GetString("chat_bot.display_name"),
		avatarUrl:        viper.GetString("chat_bot.avatar_url"),
		targets:          viper.GetStringSlice("chat_bot.target_channels"),
		patternMatching:  viper.GetStringMapString("chat_bot.pattern_matching"),
		alternativeRules: viper.GetStringMapString("chat_bot.alternative_rules"),
		searchUrl:        viper.GetString("google_search.url"),
		searchCx:         viper.GetString("google_search.cx"),
		searchKey:        viper.GetString("google_search.api_key"),
		messageBlackMap:  make(map[string]bool),
		imageUrlBlackMap: make(map[string]bool),
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
	var postMsgJson []byte
	if imageUrl == "" {
		postMsgJson = []byte(
			fmt.Sprintf(
				`{"channel": "%s", 
				"text": "%s", 
				"alias": "%s", 
				"avatar": "%s"}`,
				botTarget,
				message,
				bot.name,
				bot.avatarUrl))
	} else {
		postMsgJson = []byte(
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
	}
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

func (bot ChatBot) DeleteMsg(
	roomId string,
	msgId string) error {
	deleteMsgUrl, err := url.Parse(bot.chatUrl)
	if err != nil {
		return err
	}
	deleteMsgUrl.Path = path.Join(deleteMsgUrl.Path, "/api/v1/chat.delete")
	deleteMsgUrlString := deleteMsgUrl.String()
	deleteMsgResponse := new(DeleteMsgResult)
	deleteMsgJson := []byte(
		fmt.Sprintf(`{"roomId": "%s",  "msgId": "%s"}`,
			roomId,
			msgId))
	err = PostAPI(
		deleteMsgUrlString,
		deleteMsgJson,
		bot.loginHeader,
		deleteMsgResponse)
	if err != nil {
		return err
	}
	fmt.Printf("[INFO] Delete message response: %+v", deleteMsgResponse)
	return err
}

func (bot *ChatBot) ReplyMeme() error {
	channelsMsgUrl, err := url.Parse(bot.chatUrl)
	if err != nil {
		return err
	}
	channelsMsgUrl.Path = path.Join(channelsMsgUrl.Path, "/api/v1/channels.messages")
	channelsMsgUrlString := channelsMsgUrl.String()
ChannelLoop:
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
		if len(channelsMsgResponse.Messages) == 0 {
			fmt.Println("[WARNING] No message from channels response, skip")
			continue
		}
		targetMessage := channelsMsgResponse.Messages[0]
		fmt.Printf("[DEBUG] Target message: %+v\n", targetMessage)
		if targetMessage.Alias == bot.name {
			// Delete emoji message by bot if contains emojis below
			_, containNoEntry := targetMessage.Reactions[":no_entry:"]
			_, containNoEntrySign := targetMessage.Reactions[":no_entry_sign:"]
			_, containU7981 := targetMessage.Reactions[":u7981:"]
			_, containX := targetMessage.Reactions[":x:"]
			_, containWastebasket := targetMessage.Reactions[":wastebasket:"]
			if containNoEntry || containNoEntrySign || containU7981 || containX || containWastebasket {
				// Add message image url to black list
				targetImageUrl := targetMessage.Attachments[0].ImageUrl
				bot.imageUrlBlackMap[targetImageUrl] = true
				fmt.Printf(
					"[INFO] Add image url %s to black list\n",
					targetImageUrl)
				// Get room id by name
				channelsInfoUrl, err := url.Parse(bot.chatUrl)
				if err != nil {
					return err
				}
				channelsInfoUrl.Path = path.Join(channelsInfoUrl.Path, "/api/v1/channels.info")
				channelsInfoUrlString := channelsInfoUrl.String()
				channelsInfoResponse := new(ChannelsInfoResult)
				queries := map[string]string{
					"roomName": botTarget,
				}
				err = GetAPI(
					channelsInfoUrlString,
					queries,
					bot.loginHeader,
					channelsInfoResponse)
				if err != nil {
					return err
				}
				// Delete message
				fmt.Printf(
					"[INFO] Delete message %s emoji contains :no_entry:\n",
					targetMessage.Msg)
				err = bot.DeleteMsg(channelsInfoResponse.Channel.Id, targetMessage.Id)
				if err != nil {
					return err
				}
				continue
			} else {
				fmt.Println("[INFO] No new message, skip")
				continue
			}
		}

		// Check message in black list
		if bot.messageBlackMap[targetMessage.Msg] {
			fmt.Printf(
				"[INFO] Get message %s which is in black list, skip\n",
				targetMessage.Msg)
			continue
		}

		// Skip message contains #silent
		if strings.Contains(targetMessage.Msg, "#silent") {
			fmt.Printf(
				"[INFO] Get message %s which should be silent, skip\n",
				targetMessage.Msg)
			continue
		}

		// Skip message emoji contains :shushing_face:
		if _, ok := targetMessage.Reactions[":shushing_face:"]; ok {
			fmt.Printf(
				"[INFO] Get message %s emoji contains :shushing_face:, skip\n",
				targetMessage.Msg)
			continue
		}

		searchString := targetMessage.Msg

		// Reply to target message if match pattern
		for patternMsg, replyMsg := range bot.patternMatching {
			if strings.Contains(searchString, patternMsg) {
				fmt.Printf(
					"[INFO] Match pattern %s, reply %s\n",
					patternMsg, replyMsg)
				err = bot.PostMsg(
					botTarget,
					replyMsg,
					"")
				if err != nil {
					return err
				}
				continue ChannelLoop
			}
		}

		// Replace message by alternative rules
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
		searchText := `` + searchString + ` 梗圖 | meme`
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
		memes := searchResponse.Items
		memesLength := len(memes)
		fmt.Printf(
			"[INFO] Search memes successfully, total: %d\n",
			memesLength)
		if memesLength == 0 {
			fmt.Printf(
				"[WARNING] No meme to show, add %s to black list and skip\n",
				targetMessage.Msg)
			bot.messageBlackMap[targetMessage.Msg] = true
			continue
		}

		// Randomly choose a meme
		randomIndex := rand.Intn(memesLength)
		randomMeme := memes[randomIndex]
		for memesLength > 1 {
			fmt.Printf(
				"[DEBUG] Target #%d meme: %+v\n",
				randomIndex,
				randomMeme)
			// Check image url contains .jpg, .jpeg, .png or gif
			isValidImage := strings.Contains(
				randomMeme.Link, ".jpg") || strings.Contains(
				randomMeme.Link, ".png") || strings.Contains(
				randomMeme.Link, ".jpeg") || strings.Contains(
				randomMeme.Link, ".gif")
			isInBlackList := bot.imageUrlBlackMap[randomMeme.Link]
			// Check image url exist
			resp, err := http.Head(randomMeme.Link)
			if err != nil || resp.StatusCode != http.StatusOK || !isValidImage || isInBlackList {
				fmt.Printf(
					"[INFO] Target #%d url not exist, choose another one\n",
					randomIndex)
				// Remove image not exist meme
				memes = append(
					memes[:randomIndex],
					memes[randomIndex+1:]...)
				memesLength := len(memes)
				randomIndex = rand.Intn(memesLength)
				randomMeme = memes[randomIndex]
			} else {
				break
			}
		}
		if memesLength <= 1 {
			fmt.Println("[WARNING] All of memes image url not existed, skip")
			continue
		}

		// Reply message a meme
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
