package chatbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Reminder struct {
	Username  string
	Task      string
	TargetTime time.Time
}

type ChatBot struct {
	chatUrl, chatUser, chatPwd     string
	chatUserId, chatAuthToken      string
	name, avatarUrl                string
	targets                        []string
	patternMatching                map[string]string
	alternativeRules               map[string]string
	searchUrl, searchCx, searchKey string
	loginHeader                    LoginData
	messageBlackMap                map[string]bool
	imageUrlBlackMap               map[string]bool
	reminders                      []Reminder
	remindersMutex                 sync.Mutex
}

func New() ChatBot {
	bot := ChatBot{
		chatUrl:       viper.GetString("rocket_chat.url"),
		chatUser:      viper.GetString("rocket_chat.user_name"),
		chatPwd:       viper.GetString("rocket_chat.password"),
		chatUserId:    viper.GetString("rocket_chat.user_id"),
		chatAuthToken: viper.GetString("rocket_chat.auth_token"),
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
		reminders:        make([]Reminder, 0),
	}
	return bot
}

func (bot *ChatBot) Login() error {
	var err error
	bot.loginHeader = LoginData{bot.chatAuthToken, bot.chatUserId}
	fmt.Printf("[INFO] Login user %s successfully\n", bot.chatUser)
	return err
}

type PostMessageRequest struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
	Alias   string `json:"alias"`
	Avatar  string `json:"avatar"`
	//Attachments string `json:"attachments"`
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
		req := PostMessageRequest{
			Channel: botTarget,
			Text:    message,
			Alias:   bot.name,
			Avatar:  bot.avatarUrl,
		}
		postMsgJson, _ = json.Marshal(req)
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
			//"roomName": botTarget,
			"roomId": botTarget,
			"count":  "1",
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

		// Skip message with empty message string
		if len(targetMessage.Msg) == 0 {
			fmt.Println("[INFO] Get message with empty message string, skip\n")

		// Reminder feature
		if strings.HasPrefix(targetMessage.Msg, "@doge 提醒我") {
			fmt.Printf("[INFO] Get message contain @doge 提醒我, trigger reminder\n")

			// Extract time and task from the message
			reminderText := strings.TrimPrefix(targetMessage.Msg, "@doge 提醒我")
			reminderText = strings.TrimSpace(reminderText)

			// Find the first space to separate time and task
			spaceIndex := strings.Index(reminderText, " ")
			if spaceIndex == -1 {
				// No space found, invalid format
				err = bot.PostMsg(botTarget, "提醒格式錯誤，請使用「@doge 提醒我 [時間] [任務]」，例如「@doge 提醒我 5分後 喝水」", "")
				if err != nil {
					fmt.Printf("[ERROR] Got error while post message: ")
					fmt.Println(err)
				}
				continue
			}

			timeStr := reminderText[:spaceIndex]
			task := strings.TrimSpace(reminderText[spaceIndex:])

			// Parse time format
			duration, err := parseTimeFormat(timeStr)
			if err != nil {
				errMessage := fmt.Sprintf("無法解析時間格式「%s」，請使用「X分後」、「X秒後」、「HH:mm」或「yyyy/MM/dd-HH:mm:ss」格式（秒是可選的），且不可小於當前時間", timeStr)
				err = bot.PostMsg(botTarget, errMessage, "")
				if err != nil {
					fmt.Printf("[ERROR] Got error while post message: ")
					fmt.Println(err)
				}
				continue
			}

			// Schedule reminder
			bot.scheduleReminder(targetMessage.User.Username, task, duration, botTarget)

			// Confirm message
			confirmMessage := fmt.Sprintf("已設定提醒：%v 後提醒您「%s」", duration, task)
			err = bot.PostMsg(botTarget, confirmMessage, "")
			if err != nil {
				fmt.Printf("[ERROR] Got error while post message: ")
				fmt.Println(err)
			}
			continue
		}

		// List all reminders feature
		if strings.HasPrefix(targetMessage.Msg, "@doge 列出所有提醒") || strings.HasPrefix(targetMessage.Msg, "@doge 列出提醒") {
			fmt.Printf("[INFO] Get message to list all reminders\n")

			// Get all reminders
			remindersList := bot.listAllReminders()

			// Send the list of reminders
			err = bot.PostMsg(botTarget, remindersList, "")
			if err != nil {
				fmt.Printf("[ERROR] Got error while post message: ")
				fmt.Println(err)
			}
			continue
		}

		searchString := targetMessage.Msg

		// Reply to the target message if pattern match
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

		// Replace messages by alternative rules
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
		if strings.Contains(searchString, " gif") {
			fmt.Println("[INFO] Detect gif, add animated imgType query")
			searchQueries["q"] = strings.ReplaceAll(searchQueries["q"], " gif", "")
			searchQueries["imgType"] = "animated"
		}
		fmt.Println("[INFO] Search query: %s", searchQueries["q"])
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
		message := "@" + targetMessage.User.Username
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

// parseTimeFormat parses the time format from the message
func parseTimeFormat(timeStr string) (time.Duration, error) {
	// Parse X分後 (X minutes later)
	minRegex := regexp.MustCompile(`^(\d+)分後`)
	minMatches := minRegex.FindStringSubmatch(timeStr)
	if len(minMatches) > 1 {
		minutes, err := strconv.Atoi(minMatches[1])
		if err != nil {
			return 0, err
		}
		return time.Duration(minutes) * time.Minute, nil
	}

	// Parse X秒後 (X seconds later)
	secRegex := regexp.MustCompile(`^(\d+)秒後`)
	secMatches := secRegex.FindStringSubmatch(timeStr)
	if len(secMatches) > 1 {
		seconds, err := strconv.Atoi(secMatches[1])
		if err != nil {
			return 0, err
		}
		return time.Duration(seconds) * time.Second, nil
	}

	// Parse yyyy/MM/dd-HH:mm:ss format (seconds optional)
	dateTimeRegex := regexp.MustCompile(`^(\d{4})/(\d{1,2})/(\d{1,2})-(\d{1,2}):(\d{2})(?::(\d{2}))?`)
	dateTimeMatches := dateTimeRegex.FindStringSubmatch(timeStr)
	if len(dateTimeMatches) > 5 {
		year, err := strconv.Atoi(dateTimeMatches[1])
		if err != nil {
			return 0, err
		}
		month, err := strconv.Atoi(dateTimeMatches[2])
		if err != nil {
			return 0, err
		}
		day, err := strconv.Atoi(dateTimeMatches[3])
		if err != nil {
			return 0, err
		}
		hour, err := strconv.Atoi(dateTimeMatches[4])
		if err != nil {
			return 0, err
		}
		minute, err := strconv.Atoi(dateTimeMatches[5])
		if err != nil {
			return 0, err
		}

		// Seconds are optional
		seconds := 0
		if len(dateTimeMatches) > 6 && dateTimeMatches[6] != "" {
			seconds, err = strconv.Atoi(dateTimeMatches[6])
			if err != nil {
				return 0, err
			}
		}

		now := time.Now()
		targetTime := time.Date(year, time.Month(month), day, hour, minute, seconds, 0, now.Location())

		// Check if the target time is in the future
		if targetTime.Before(now) {
			return 0, fmt.Errorf("target time must be in the future")
		}

		return targetTime.Sub(now), nil
	}

	// Parse time string (HH:MM format)
	timeRegex := regexp.MustCompile(`^(\d{1,2}):(\d{2})`)
	timeMatches := timeRegex.FindStringSubmatch(timeStr)
	if len(timeMatches) > 2 {
		hour, err := strconv.Atoi(timeMatches[1])
		if err != nil {
			return 0, err
		}
		minute, err := strconv.Atoi(timeMatches[2])
		if err != nil {
			return 0, err
		}

		now := time.Now()
		targetTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

		// If the target time is in the past, set it to tomorrow
		if targetTime.Before(now) {
			targetTime = targetTime.Add(24 * time.Hour)
		}

		return targetTime.Sub(now), nil
	}

	return 0, fmt.Errorf("unsupported time format")
}

// listAllReminders returns a formatted string containing all scheduled reminders
func (bot *ChatBot) listAllReminders() string {
	bot.remindersMutex.Lock()
	defer bot.remindersMutex.Unlock()

	if len(bot.reminders) == 0 {
		return "目前沒有任何提醒。"
	}

	var result strings.Builder
	result.WriteString("所有提醒：\n")

	for i, reminder := range bot.reminders {
		timeLeft := time.Until(reminder.TargetTime)
		timeLeftStr := formatDuration(timeLeft)
		result.WriteString(fmt.Sprintf("%d. %s後提醒 @%s：%s\n", 
			i+1, timeLeftStr, reminder.Username, reminder.Task))
	}

	return result.String()
}

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	hours := d / time.Hour
	d -= hours * time.Hour

	minutes := d / time.Minute
	d -= minutes * time.Minute

	seconds := d / time.Second

	if hours > 0 {
		return fmt.Sprintf("%d小時%d分鐘", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%d分鐘%d秒", minutes, seconds)
	} else {
		return fmt.Sprintf("%d秒", seconds)
	}
}

// scheduleReminder schedules a reminder and sends a notification when the time arrives
func (bot *ChatBot) scheduleReminder(username string, task string, duration time.Duration, channel string) {
	targetTime := time.Now().Add(duration)

	// Create and store the reminder
	reminder := Reminder{
		Username:   username,
		Task:       task,
		TargetTime: targetTime,
	}

	bot.remindersMutex.Lock()
	bot.reminders = append(bot.reminders, reminder)
	bot.remindersMutex.Unlock()

	go func() {
		fmt.Printf("[INFO] Scheduled reminder for @%s in %v: %s\n", username, duration, task)

		// Helper function to send the reminder and clean up
		sendReminder := func() {
			// Remove the reminder from the list
			bot.remindersMutex.Lock()
			for i, r := range bot.reminders {
				if r.Username == username && r.Task == task && r.TargetTime.Equal(targetTime) {
					// Remove this reminder
					bot.reminders = append(bot.reminders[:i], bot.reminders[i+1:]...)
					break
				}
			}
			bot.remindersMutex.Unlock()

			message := fmt.Sprintf("@%s %s", username, task)
			err := bot.PostMsg(channel, message, "")
			if err != nil {
				fmt.Printf("[ERROR] Failed to send reminder: %v\n", err)
			} else {
				fmt.Printf("[INFO] Sent reminder to @%s: %s\n", username, task)
			}
		}

		// Set up a timer for the exact target time
		timer := time.NewTimer(duration)

		// Also set up a backup ticker that checks less frequently
		// This ensures we don't miss the reminder if the system sleeps
		backupTicker := time.NewTicker(30 * time.Second)
		defer backupTicker.Stop()
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				// Timer fired at the target time
				sendReminder()
				return

			case <-backupTicker.C:
				// Backup check in case the timer was missed
				now := time.Now()
				if now.After(targetTime) {
					sendReminder()
					return
				}
			}
		}
	}()
}
