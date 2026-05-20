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
	"os"
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

type LlmApiRequest struct {
	Prompt string  `json:"prompt"`
	Tools  *string `json:"tools,omitempty"`
}

type LlmApiResponse struct {
	Content string `json:"content"`
}

type ChatModel struct {
	Name        string
	Trigger     string
	Endpoint    string
	AvatarURL   string
	Description string
}

var commonModels = []ChatModel{
	{Name: "Gemini", Trigger: "@gemini ", Endpoint: "/api/v1/gemini/chat", AvatarURL: "https://i.imgur.com/2Uut5uw.png", Description: "Use Gemini 3.5 Flash for general text responses."},
	{Name: "Llama", Trigger: "@llama ", Endpoint: "/api/v1/llama/chat", AvatarURL: "https://i.imgur.com/5bsBgBf.png", Description: "Use Llama 3.3 70B for general text responses."},
	{Name: "Nemotron", Trigger: "@nemo ", Endpoint: "/api/v1/nemotron/chat", AvatarURL: "https://raw.githubusercontent.com/lobehub/lobe-icons/refs/heads/master/packages/static-png/dark/nvidia-color.png", Description: "Use NVIDIA Nemotron 3 Super for general text responses."},
	{Name: "GLM", Trigger: "@glm ", Endpoint: "/api/v1/glm/chat", AvatarURL: "https://upload.wikimedia.org/wikipedia/commons/thumb/f/f4/Z.ai_%28company_logo%29.svg/1280px-Z.ai_%28company_logo%29.svg.png", Description: "Use Z.ai GLM 4.5 Air for general text responses."},
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

func (bot ChatBot) InvokeLLM(botTarget string, model ChatModel, prompt string) error {
	fmt.Printf("[INFO] Get message contain %s, trigger %s\n", model.Trigger, model.Name)
	request := LlmApiRequest{
		Prompt: prompt,
	}
	client := &http.Client{}
	reqBytes, err := json.Marshal(request)
	if err != nil {
		return err
	}

	apiUrl := "http://localhost:8888" + model.Endpoint

	req, err := http.NewRequest("POST", apiUrl, bytes.NewReader(reqBytes))
	if err != nil {
		fmt.Printf("[ERROR] Failed to create request for %s, error: %v\n", model.Name, err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[ERROR] client.Do error for %s: %v\n", model.Name, err)
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[ERROR] io.ReadAll error for %s: %v\n", model.Name, err)
		return err
	}

	if resp.StatusCode != 200 {
		fmt.Printf("[ERROR] %s not 200 response, status code: %v, body: %s\n", model.Name, resp.StatusCode, string(body))
		return fmt.Errorf("model %s returned status %d", model.Name, resp.StatusCode)
	}

	var llmResp LlmApiResponse
	err = json.Unmarshal(body, &llmResp)
	if err != nil {
		fmt.Printf("[ERROR] json.Unmarshal error for %s: %v\n", model.Name, err)
		return err
	}

	// Reply message
	oldBotAvatarURL := bot.avatarUrl
	bot.avatarUrl = model.AvatarURL
	err = bot.PostMsg(botTarget, llmResp.Content, "")
	bot.avatarUrl = oldBotAvatarURL

	if err != nil {
		fmt.Printf("[ERROR] Got error while post message for %s: %v\n", model.Name, err)
		return err
	}
	return nil
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
			_, containX := targetMessage.Reactions[":x:"]
			_, containWastebasket := targetMessage.Reactions[":wastebasket:"]
			if containX || containWastebasket {
				// Add message image url to black list
				if len(targetMessage.Attachments) > 0 {
					targetImageUrl := targetMessage.Attachments[0].ImageUrl
					bot.imageUrlBlackMap[targetImageUrl] = true
					fmt.Printf(
						"[INFO] Add image url %s to black list\n",
						targetImageUrl)
				}
				if err != nil {
					return err
				}
				// Delete message
				fmt.Printf(
					"[INFO] Delete message %s emoji contains :x:\n",
					targetMessage.Msg)
				err = bot.DeleteMsg(botTarget, targetMessage.Id)
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

		// Skip message contains #silent or #s
		if strings.Contains(targetMessage.Msg, "?s") || strings.Contains(targetMessage.Msg, "#s") {
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
			fmt.Println("[INFO] Get message with empty message string, skip")
			continue
		}

		//lowerMessage := strings.ToLower(targetMessage.Msg)
		languageModelTriggered := false

		// Help
		if strings.Contains(targetMessage.Msg, "?h") {
			fmt.Printf("[INFO] Get message contain #h, trigger help\n")
			helpString := "**Command List**\n```\n" +
				"- Do not reply anything to this message: {your message} ?s or #s\n" +
				"- Help: ?h\n" +
				"- FLUX Image: @draw {image prompt}\n" +
				"  - Create a FLUX image from the text you provide.\n" +
				"- Reset Chat History: @reset\n" +
				"  - Clear all chat history in current session.\n" +
				"- Update system prompt (For @jason.wu only!): @system {system prompt}\n" +
				"  - System prompt will be updated for all models that support system instruction.\n" +
				"- Ask all active chatbot (For @jason.wu only!): @allbot {text prompt}\n" +
				"  - Trigger all active chatbot ("

			modelNames := []string{}
			for _, model := range commonModels {
				modelNames = append(modelNames, model.Name)
			}
			helpString += strings.Join(modelNames, ", ") + ").\n"

			for _, model := range commonModels {
				helpString += fmt.Sprintf("- %s: %s{text prompt}\n  - %s\n", model.Name, model.Trigger, model.Description)
			}

			helpString += "- Gemini Search Retrieval: @gemini_search {text prompt} or @gs {text prompt}\n" +
				"  - Use Gemini 3.5 Flash with search to find relevant information.\n" +
				"- Gemini Code Execution: @gemini_code {text prompt}\n" +
				"  - Use Gemini 3.5 Flash with code execution for programming tasks.\n" +
				"- Reminder: @doge 提醒我 {time} {task}\n" +
				"  - Set a reminder for a specific time. Time formats: X分後, X秒後, HH:mm, or yyyy/MM/dd-HH:mm:ss (seconds optional).\n" +
				"  - Example: @doge 提醒我 5分後 喝水\n" +
				"- List Reminders: @doge 列出所有提醒 or @doge 列出提醒\n" +
				"  - List all scheduled reminders with their trigger times and tasks.\n" +
				"- Meme Image: {any message}\n" +
				"  - Find a meme image from the message you send.\n" +
				"```\n" +
				"**Emoji React Command**\n" +
				"- Delete Doge's message: :x: or :wastebasket:\n" +
				"- Do not reply anything to this message: :shushing_face:"
			// Reply message
			err = bot.PostMsg(botTarget, helpString, "")
			if err != nil {
				fmt.Printf("[ERROR] Got error while post message: ")
				fmt.Println(err)
			}
			continue
		}

		// FLUX Image
		if strings.Contains(targetMessage.Msg, "@draw ") {
			fmt.Println("[INFO] Get message contain @draw, trigger FLUX Image")
			prompt := strings.Replace(targetMessage.Msg, "@draw ", "", -1)
			prompt = strings.Replace(prompt, "\"", "'", -1)
			fmt.Printf("[INFO] Prompt: %s\n", prompt)
			scriptFolder, err := os.Getwd()
			if err != nil {
				fmt.Printf("[ERROR] Failed to get working directory: %v\n", err)
				continue
			}
			fullCommand := fmt.Sprintf(
				"%s/venv/bin/python %s/scripts/generate-image.py -p \"%s\"",
				scriptFolder,
				scriptFolder,
				prompt,
			)
			cmd := exec.Command("bash", "-c", fullCommand)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
				// Send error messages
				errorStrList := strings.Split(stderr.String(), "\n")
				lastErrorStr := errorStrList[len(errorStrList)-2]
				errMessage := fmt.Sprintf("[ERROR] Got error from Hugging Face: %s", lastErrorStr)
				err = bot.PostMsg(botTarget, errMessage, "")
				if err != nil {
					fmt.Printf("[ERROR] Got error while post message: ")
					fmt.Println(err)
				}
				continue
			}
			imageURL := strings.TrimSuffix(string(out), "\n")
			fmt.Printf("[INFO] FLUX Image Generated image URL: %s\n", imageURL)

			// Reply message a meme
			message := "@" + targetMessage.User.Username
			oldBotAvatarURL := bot.avatarUrl
			bot.avatarUrl = "https://aimodelflux.com/wp-content/uploads/2024/08/flux.webp"
			err = bot.PostMsg(
				botTarget,
				message,
				imageURL)
			bot.avatarUrl = oldBotAvatarURL
			if err != nil {
				return err
			}
			continue
		}

		// Reset
		if strings.Contains(targetMessage.Msg, "@reset") {
			fmt.Printf("[INFO] Get message contain @reset, clear current chat history.\n")
			request := LlmApiRequest{
				Prompt: "default",
			}
			client := &http.Client{}
			reqBytes, err := json.Marshal(request)
			req, err := http.NewRequest("PATCH", "http://localhost:8888/api/v1/system_prompt", bytes.NewReader(reqBytes))
			if err != nil {
				fmt.Printf("[ERROR] Failed to create request, error: %v\n", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, _ := client.Do(req)
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("[ERROR] io.ReadAll(resp.Body) error: %v\n", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				bodyString := string(body)
				fmt.Printf("[ERROR] not 200 response, status code: %v, body: %v\n", resp.StatusCode, bodyString)
			}

			// Reply message
			err = bot.PostMsg(botTarget, "Chat cleared.", "")
			if err != nil {
				fmt.Printf("[ERROR] Got error while post message: ")
				fmt.Println(err)
			}
			continue
		}

		// Update system prompt
		if strings.Contains(targetMessage.Msg, "@system ") && targetMessage.User.Username == "jason.wu" {
			fmt.Printf("[INFO] Get message contain @system, update system prompt\n")
			targetMessage.Msg = strings.ReplaceAll(targetMessage.Msg, "@system ", "")
			request := LlmApiRequest{
				Prompt: targetMessage.Msg,
			}
			client := &http.Client{}
			reqBytes, err := json.Marshal(request)
			req, err := http.NewRequest("PATCH", "http://localhost:8888/api/v1/system_prompt", bytes.NewReader(reqBytes))
			if err != nil {
				fmt.Printf("[ERROR] Failed to create request, error: %v\n", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, _ := client.Do(req)
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("[ERROR] io.ReadAll(resp.Body) error: %v\n", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				bodyString := string(body)
				fmt.Printf("[ERROR] not 200 response, status code: %v, body: %v\n", resp.StatusCode, bodyString)
			}
			var geminiResponse LlmApiResponse
			err = json.Unmarshal(body, &geminiResponse)
			if err != nil {
				fmt.Printf("[ERROR] json.NewDecoder(resp.Body).Decode(new(LlmApiResponse)) error: %v\n", err)
			}

			// Reply message
			err = bot.PostMsg(botTarget, geminiResponse.Content, "")
			if err != nil {
				fmt.Printf("[ERROR] Got error while post message: ")
				fmt.Println(err)
			}
			continue
		} else if strings.Contains(targetMessage.Msg, "@system ") {
			err = bot.PostMsg(botTarget, "You are not @jason.wu, access denied.", "")
			if err != nil {
				fmt.Printf("[ERROR] Got error while post message: ")
				fmt.Println(err)
			}
			continue
		}

		// Determine which LLM is triggered
		if strings.Contains(targetMessage.Msg, "@allbot ") && targetMessage.User.Username == "jason.wu" {
			allTriggers := ""
			for _, model := range commonModels {
				allTriggers += model.Trigger + " "
			}
			targetMessage.Msg = strings.ReplaceAll(targetMessage.Msg, "@allbot ", allTriggers)
		} else if strings.Contains(targetMessage.Msg, "@allbot ") {
			err = bot.PostMsg(botTarget, "You are not @jason.wu, access denied.", "")
			if err != nil {
				fmt.Printf("[ERROR] Got error while post message: ")
				fmt.Println(err)
			}
			continue
		}
		triggerGemini := false
		triggerGeminiCode := false
		triggerGeminiSearch := false
		triggeredCommonModels := make([]ChatModel, 0)

		if strings.Contains(targetMessage.Msg, "@gemini_code ") {
			triggerGemini = true
			triggerGeminiCode = true
			targetMessage.Msg = strings.ReplaceAll(targetMessage.Msg, "@gemini_code ", "")
		}
		if strings.Contains(targetMessage.Msg, "@gemini_search ") || strings.Contains(targetMessage.Msg, "@gs ") {
			triggerGemini = true
			triggerGeminiSearch = true
			targetMessage.Msg = strings.ReplaceAll(targetMessage.Msg, "@gemini_search ", "")
			targetMessage.Msg = strings.ReplaceAll(targetMessage.Msg, "@gs ", "")
		}

		for _, model := range commonModels {
			if strings.Contains(targetMessage.Msg, model.Trigger) {
				if model.Name == "Gemini" {
					triggerGemini = true
				} else {
					triggeredCommonModels = append(triggeredCommonModels, model)
				}
				targetMessage.Msg = strings.ReplaceAll(targetMessage.Msg, model.Trigger, "")
			}
		}

		// Gemini
		if triggerGemini {
			fmt.Printf("[INFO] Get message contain @gemini, trigger Gemini\n")
			var request LlmApiRequest
			if triggerGeminiCode {
				fmt.Printf("[INFO] Run in Gemini Code Execution mode\n")
				tools := "code_execution"
				request = LlmApiRequest{
					Prompt: targetMessage.Msg,
					Tools:  &tools,
				}
			} else if triggerGeminiSearch {
				fmt.Printf("[INFO] Run in Gemini Search Retrieval mode\n")
				tools := "google_search_tool"
				request = LlmApiRequest{
					Prompt: targetMessage.Msg,
					Tools:  &tools,
				}
			} else {
				//request = LlmApiRequest{
				//	Prompt: targetMessage.User.Username + ": " + targetMessage.Msg,
				//}
				request = LlmApiRequest{
					Prompt: targetMessage.Msg,
				}
			}
			client := &http.Client{}
			reqBytes, err := json.Marshal(request)
			req, err := http.NewRequest("POST", "http://localhost:8888/api/v1/gemini/chat", bytes.NewReader(reqBytes))
			if err != nil {
				fmt.Printf("[ERROR] Failed to create request, error: %v\n", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, _ := client.Do(req)
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("[ERROR] io.ReadAll(resp.Body) error: %v\n", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				bodyString := string(body)
				fmt.Printf("[ERROR] not 200 response, status code: %v, body: %v\n", resp.StatusCode, bodyString)
			}
			var geminiResponse LlmApiResponse
			err = json.Unmarshal(body, &geminiResponse)
			if err != nil {
				fmt.Printf("[ERROR] json.NewDecoder(resp.Body).Decode(new(LlmApiResponse)) error: %v\n", err)
			}

			// Optimize markdown
			geminiResponse.Content = strings.ReplaceAll(geminiResponse.Content, ":**", "**:")
			geminiResponse.Content = strings.ReplaceAll(geminiResponse.Content, ":*", "*:")

			// Reply message
			oldBotAvatarURL := bot.avatarUrl
			bot.avatarUrl = "https://i.imgur.com/2Uut5uw.png"
			err = bot.PostMsg(botTarget, geminiResponse.Content, "")
			bot.avatarUrl = oldBotAvatarURL
			if err != nil {
				fmt.Printf("[ERROR] Got error while post message: ")
				fmt.Println(err)
			}
			languageModelTriggered = true
		}

		// Common Models (DeepSeek, Llama, etc.)
		for _, model := range triggeredCommonModels {
			err := bot.InvokeLLM(botTarget, model, targetMessage.Msg)
			if err == nil {
				languageModelTriggered = true
			}
		}

		if languageModelTriggered {
			continue
		}

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

			// Parse time format — returns an absolute target time
			targetTime, err := parseTimeFormat(timeStr)
			if err != nil {
				errMessage := fmt.Sprintf("無法解析時間格式「%s」，請使用「X分後」、「X秒後」、「HH:mm」或「yyyy/MM/dd-HH:mm:ss」格式（秒是可選的），且不可小於當前時間", timeStr)
				err = bot.PostMsg(botTarget, errMessage, "")
				if err != nil {
					fmt.Printf("[ERROR] Got error while post message: ")
					fmt.Println(err)
				}
				continue
			}

			// Schedule reminder using absolute wall-clock target time
			bot.scheduleReminder(targetMessage.User.Username, task, targetTime, botTarget)

			// Confirm message — show human-readable time-left and exact target time
			timeLeft := time.Until(targetTime)
			confirmMessage := fmt.Sprintf(
				"已設定提醒：%s 後（%s）提醒您「%s」",
				formatDuration(timeLeft),
				targetTime.Format("01/02 15:04:05"),
				task,
			)
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

		// Replace all symbols to spaces
		//re, _ := regexp.Compile(`\W`)
		//searchString = re.ReplaceAllString(searchString, " ")

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
				if memesLength == 0 {
					// TODO: add to blacklist
					break
				}
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

// parseTimeFormat parses the time string and returns an absolute target time.Time.
// Supported formats:
//   - X分後  : X minutes from now
//   - X秒後  : X seconds from now
//   - HH:mm  : today at HH:mm (or tomorrow if already past)
//   - yyyy/MM/dd-HH:mm[:ss] : absolute date-time
func parseTimeFormat(timeStr string) (time.Time, error) {
	now := time.Now()

	// Parse X分後 (X minutes later)
	minRegex := regexp.MustCompile(`^(\d+)分後`)
	minMatches := minRegex.FindStringSubmatch(timeStr)
	if len(minMatches) > 1 {
		minutes, err := strconv.Atoi(minMatches[1])
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(time.Duration(minutes) * time.Minute), nil
	}

	// Parse X秒後 (X seconds later)
	secRegex := regexp.MustCompile(`^(\d+)秒後`)
	secMatches := secRegex.FindStringSubmatch(timeStr)
	if len(secMatches) > 1 {
		seconds, err := strconv.Atoi(secMatches[1])
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(time.Duration(seconds) * time.Second), nil
	}

	// Parse yyyy/MM/dd-HH:mm:ss format (seconds optional)
	dateTimeRegex := regexp.MustCompile(`^(\d{4})/(\d{1,2})/(\d{1,2})-(\d{1,2}):(\d{2})(?::(\d{2}))?`)
	dateTimeMatches := dateTimeRegex.FindStringSubmatch(timeStr)
	if len(dateTimeMatches) > 5 {
		year, err := strconv.Atoi(dateTimeMatches[1])
		if err != nil {
			return time.Time{}, err
		}
		month, err := strconv.Atoi(dateTimeMatches[2])
		if err != nil {
			return time.Time{}, err
		}
		day, err := strconv.Atoi(dateTimeMatches[3])
		if err != nil {
			return time.Time{}, err
		}
		hour, err := strconv.Atoi(dateTimeMatches[4])
		if err != nil {
			return time.Time{}, err
		}
		minute, err := strconv.Atoi(dateTimeMatches[5])
		if err != nil {
			return time.Time{}, err
		}

		// Seconds are optional
		seconds := 0
		if len(dateTimeMatches) > 6 && dateTimeMatches[6] != "" {
			seconds, err = strconv.Atoi(dateTimeMatches[6])
			if err != nil {
				return time.Time{}, err
			}
		}

		targetTime := time.Date(year, time.Month(month), day, hour, minute, seconds, 0, now.Location())
		if targetTime.Before(now) {
			return time.Time{}, fmt.Errorf("target time must be in the future")
		}
		return targetTime, nil
	}

	// Parse HH:mm format — today, or tomorrow if the time has already passed today
	timeRegex := regexp.MustCompile(`^(\d{1,2}):(\d{2})`)
	timeMatches := timeRegex.FindStringSubmatch(timeStr)
	if len(timeMatches) > 2 {
		hour, err := strconv.Atoi(timeMatches[1])
		if err != nil {
			return time.Time{}, err
		}
		minute, err := strconv.Atoi(timeMatches[2])
		if err != nil {
			return time.Time{}, err
		}

		targetTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if !targetTime.After(now) {
			// Already past — schedule for the same time tomorrow
			targetTime = targetTime.AddDate(0, 0, 1)
		}
		return targetTime, nil
	}

	return time.Time{}, fmt.Errorf("unsupported time format")
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

// scheduleReminder schedules a reminder and sends a notification when the wall-clock time arrives.
// It uses an absolute target time rather than a sleep duration so that system sleep/wake cycles
// do not delay delivery — on every poll tick the current wall-clock time is compared to the
// fixed target, and the reminder fires as soon as the system is awake past that time.
func (bot *ChatBot) scheduleReminder(username string, task string, targetTime time.Time, channel string) {
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
		timeLeft := time.Until(targetTime)
		fmt.Printf("[INFO] Scheduled reminder for @%s at %s (in %s): %s\n",
			username, targetTime.Format("2006/01/02 15:04:05"), formatDuration(timeLeft), task)

		// removeReminder removes this reminder from the active list.
		removeReminder := func() {
			bot.remindersMutex.Lock()
			for i, r := range bot.reminders {
				if r.Username == username && r.Task == task && r.TargetTime.Equal(targetTime) {
					bot.reminders = append(bot.reminders[:i], bot.reminders[i+1:]...)
					break
				}
			}
			bot.remindersMutex.Unlock()
		}

		// sendReminder removes and fires the reminder, with retry on transient errors.
		sendReminder := func() {
			removeReminder()
			message := fmt.Sprintf("@%s %s", username, task)
			const maxRetries = 3
			for attempt := 1; attempt <= maxRetries; attempt++ {
				err := bot.PostMsg(channel, message, "")
				if err == nil {
					fmt.Printf("[INFO] Sent reminder to @%s: %s\n", username, task)
					return
				}
				fmt.Printf("[ERROR] Failed to send reminder (attempt %d/%d): %v\n", attempt, maxRetries, err)
				if attempt < maxRetries {
					time.Sleep(5 * time.Second)
				}
			}
			fmt.Printf("[ERROR] Gave up sending reminder to @%s after %d attempts: %s\n", username, maxRetries, task)
		}

		// Wall-clock polling loop — fully immune to system sleep drift.
		// Every 5 seconds we read the real wall-clock time and compare it to the
		// fixed targetTime. If the system was asleep, the next tick after wake-up
		// will immediately detect that the deadline has passed and fire.
		const pollInterval = 5 * time.Second
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for range ticker.C {
			if !time.Now().Before(targetTime) {
				sendReminder()
				return
			}
		}
	}()
}
