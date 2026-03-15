package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var totals = struct {
	sync.RWMutex
	Data map[string]int
}{Data: make(map[string]int)}

const dataFile = "totals.json"

func loadTotals() {
	file, err := os.Open(dataFile)
	if err != nil {
		return
	}
	defer file.Close()

	json.NewDecoder(file).Decode(&totals.Data)
}

func saveTotals() {
	file, err := os.Create(dataFile)
	if err != nil {
		return
	}
	defer file.Close()

	json.NewEncoder(file).Encode(totals.Data)
}

func main() {

	loadTotals()

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	r := gin.Default()

	r.POST("/webhook", func(c *gin.Context) {

		var update tgbotapi.Update
		c.BindJSON(&update)

		if update.Message != nil {

			text := update.Message.Text
			chatID := update.Message.Chat.ID

			parts := strings.Fields(text)

			// show all totals
			if text == "all" {

				msg := "All Totals:\n"

				for k, v := range totals.Data {
					msg += k + " : " + strconv.Itoa(v) + "\n"
				}

				bot.Send(tgbotapi.NewMessage(chatID, msg))
				return
			}

			// show one total
			if len(parts) == 1 {

				tag := parts[0]

				total := totals.Data[tag]

				bot.Send(tgbotapi.NewMessage(chatID,
					tag+" total : "+strconv.Itoa(total)))
				return
			}

			// add or subtract
			tag := parts[1]
			amount, _ := strconv.Atoi(parts[2])

			totals.Data[tag] += amount

			saveTotals()

			bot.Send(tgbotapi.NewMessage(chatID,
				"Updated "+tag+" total : "+strconv.Itoa(totals.Data[tag])))

		}

		c.JSON(http.StatusOK, gin.H{})
	})

	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
}
