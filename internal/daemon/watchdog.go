package daemon

import (
	"annoying-doge-bot/internal/chatbot"
	"fmt"
	"github.com/spf13/viper"
	"time"
)

type WatchDog struct {
	TimeInterval time.Duration `default:"600"`
	TimeLimit    time.Duration `default:"86400"`
}

func NewWatchDog() WatchDog {
	dog := WatchDog{
		TimeInterval: time.Duration(
			viper.GetInt("watch_dog.time_interval_sec")),
		TimeLimit: time.Duration(
			viper.GetInt("watch_dog.time_limit_sec")),
	}
	return dog
}

func (dog WatchDog) Run() {
	// Init chat bot
	bot := chatbot.New()
	loginErr := bot.Login()
	if loginErr != nil {
		panic(fmt.Errorf("Fatal error login by http post: %s \n", loginErr))
	}
	fmt.Println("[INFO] WatchDog: Init done")

	// Set ticker and time limit
	ticker := time.NewTicker(dog.TimeInterval * time.Second)
	defer ticker.Stop()
	done := make(chan bool)
	go func() {
		time.Sleep(dog.TimeLimit * time.Second)
		done <- true
	}()
	for {
		select {
		case <-done:
			fmt.Println(
				"[INFO] WatchDog: Time limit reached, stop running")
			return
		case now := <-ticker.C:
			fmt.Printf(
				"[INFO] WatchDog: Start a job at time: %v\n", now)
			replyErr := bot.ReplyMeme()
			if replyErr != nil {
				panic(fmt.Errorf("Fatal error chatbot reply meme: %s \n", replyErr))
			}
		}
	}
}
