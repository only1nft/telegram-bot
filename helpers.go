package main

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func GetCollection(mint string) *Collection {
	for _, col := range collections {
		for _, m := range col.Mints {
			if m == mint {
				return col
			}
		}
	}

	return nil
}

func SendMessage(chatID int64, text string) (tgbotapi.Message, error) {
	return bot.Send(tgbotapi.NewMessage(chatID, text))
}

func unique(slice []int64) []int64 {
	keys := make(map[int64]bool)
	list := []int64{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}

	return list
}

func revokeAccess(members map[string][]int64) bool {
	all, err := repo.GetAllMints()
	if err != nil {
		return false
	}
	for colID, c := range members {
		for _, u := range c {
			remove := true
			for _, d := range all {
				if d.User == u {
					remove = false

					break
				}
			}

			collection := collections[colID]

			if remove {
				if _, err = bot.Request(tgbotapi.BanChatMemberConfig{ChatMemberConfig: tgbotapi.ChatMemberConfig{
					ChatID: collection.ChatID,
					UserID: u,
				}}); err != nil {
					log.Printf("failed to ban user: %v", err)

					return false
				}
				bot.Send(tgbotapi.NewMessage(u, fmt.Sprintf("Your wallet doesn't have %s anymore, you have been removed from the private chat.", collection.Name)))
			}
		}
	}

	return true
}
