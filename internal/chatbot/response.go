package chatbot

type LoginData struct {
	AuthToken string `json:"authToken" default:""`
	UserId    string `json:"userId" default:""`
}

type User struct {
	Id       string `json:"_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type Message struct {
	Id    string `json:"_id"`
	Msg   string `json:"msg"`
	User  User   `json:"u"`
	Alias string `json:"alias"`
}

// See: https://rocket.chat/docs/developer-guides/rest-api/authentication/login/
type LoginResult struct {
	Status string    `json:"status"`
	Data   LoginData `json:"data"`
}

// See: https://rocket.chat/docs/developer-guides/rest-api/channels/messages/
type ChannelsMsgResult struct {
	Success  bool      `json:"success"`
	Messages []Message `json:"messages"`
	Total    int       `json:"total"`
}

// See: https://rocket.chat/docs/developer-guides/rest-api/chat/postmessage/
type PostMsgResult struct {
	Success bool   `json:"success"`
	Channel string `json:"channel"`
}

// See: https://developers.google.com/custom-search/v1/reference/rest/v1/Search
type SearchItem struct {
	Title string `json:"title"`
	Link  string `json:"link"`
}

type SearchResult struct {
	Items []SearchItem `json:"items"`
}
