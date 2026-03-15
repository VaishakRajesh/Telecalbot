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

// ------------------------------
// Thread-safe totals map
// ------------------------------
var totals = struct {
	sync.RWMutex
	Data map[string]int
}{Data: make(map[string]int)}

// JSON file path on Render persistent disk
const dataFile = "/mnt/data/totals.json"

// ------------------------------
// Load totals from JSON file
// ------------------------------
func loadTotals() {
	// Check if file exists, create if not
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		os.WriteFile(dataFile, []byte("{}"), 0644)
	}

	file, err := os.Open(dataFile)
	if err != nil {
		log.Println("Cannot open totals.json:", err)
		return
	}
	defer file.Close()

	// Decode JSON into totals.Data map
	err = json.NewDecoder(file).Decode(&totals.Data)
	if err != nil {
		log.Println("Cannot decode JSON:", err)
	}
}

// ------------------------------
// Save totals to JSON file
// ------------------------------
func saveTotals() {
	totals.RLock()
	defer totals.RUnlock()

	file, err := os.Create(dataFile)
	if err != nil {
		log.Println("Cannot create totals.json:", err)
		return
	}
	defer file.Close()

	err = json.NewEncoder(file).Encode(totals.Data)
	if err != nil {
		log.Println("Cannot encode JSON:", err)
	}
}

func main() {
	// ------------------------------
	// Load existing totals on startup
	// ------------------------------
	loadTotals()

	// ------------------------------
	// Telegram Bot setup
	// ------------------------------
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("Please set TELEGRAM_BOT_TOKEN environment variable")
	}
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	// ------------------------------
	// Gin HTTP server
	// ------------------------------
	r := gin.Default()

	// Root route (optional, test if server is running)
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "Bot server running"})
	})

	// ------------------------------
	// Webhook route: Telegram POST updates
	// ------------------------------
	r.POST("/webhook", func(c *gin.Context) {
		var update tgbotapi.Update
		c.BindJSON(&update)

		if update.Message != nil {
			text := update.Message.Text
			chatID := update.Message.Chat.ID
			parts := strings.Fields(text)

			// ------------------------------
			// Command: "all" -> show all totals
			// ------------------------------
			if strings.ToLower(text) == "all" {
				totals.RLock()
				if len(totals.Data) == 0 {
					bot.Send(tgbotapi.NewMessage(chatID, "No totals recorded yet."))
					totals.RUnlock()
					return
				}

				msg := "All Totals:\n"
				for tag, amount := range totals.Data {
					msg += tag + " : " + strconv.Itoa(amount) + "\n"
				}
				totals.RUnlock()
				bot.Send(tgbotapi.NewMessage(chatID, msg))
				return
			}

			// ------------------------------
			// Command: "<tag>" -> show total for one tag
			// ------------------------------
			if len(parts) == 1 {
				tag := parts[0]
				totals.RLock()
				total := totals.Data[tag]
				totals.RUnlock()
				bot.Send(tgbotapi.NewMessage(chatID, tag+" total : "+strconv.Itoa(total)))
				return
			}

			// ------------------------------
			// Transaction: <date> <tag> <amount>
			// Example: 10/3/26 ajin +200 or 10/3/26 ajin -50
			// ------------------------------
			tag := parts[1]
			amountStr := parts[2]

			// Remove + if present
			amountStr = strings.Replace(amountStr, "+", "", 1)
			amount, err := strconv.Atoi(amountStr)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, "Invalid amount: "+parts[2]))
				return
			}

			// Update total safely
			totals.Lock()
			totals.Data[tag] += amount
			newTotal := totals.Data[tag]
			totals.Unlock()

			// Save updated totals to JSON
			saveTotals()

			bot.Send(tgbotapi.NewMessage(chatID, "Updated "+tag+" total : "+strconv.Itoa(newTotal)))
		}

		c.JSON(http.StatusOK, gin.H{})
	})

	// ------------------------------
	// Start server with port
	// ------------------------------
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}
