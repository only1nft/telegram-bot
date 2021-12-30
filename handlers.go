package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gagliardetto/solana-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	only1sdk "github.com/only1nft/solana-sdk-go"
)

func processUpdate(update tgbotapi.Update) {
	defer repo.SetString(updateIdKey, fmt.Sprint(update.UpdateID))

	// On join request
	if update.ChatJoinRequest != nil {
		onJoin(update)

		return
	}

	if !update.Message.Chat.IsPrivate() {
		return
	}

	// Wait until public key input
	if update.Message == nil {
		return
	}

	verify(update)
}

func onJoin(update tgbotapi.Update) {
	all, err := repo.GetAllMints()
	if err != nil {
		SendMessage(update.ChatJoinRequest.From.ID, errMsg)

		return
	}

	approve := false
	for mint, data := range all {
		if data.User != update.ChatJoinRequest.From.ID {
			continue
		}
		if GetCollection(mint).ChatID == update.ChatJoinRequest.Chat.ID {
			approve = true
			break
		}
	}

	if !approve {
		bot.Send(tgbotapi.DeclineChatJoinRequest{
			UserID: update.ChatJoinRequest.From.ID,
			ChatConfig: tgbotapi.ChatConfig{
				ChatID: update.ChatJoinRequest.Chat.ID,
			},
		})

		return
	}

	_, err = bot.Request(tgbotapi.ApproveChatJoinRequestConfig{
		UserID: update.ChatJoinRequest.From.ID,
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: update.ChatJoinRequest.Chat.ID,
		},
	})
	if err != nil {
		log.Printf("failed to approve join request: %v", err)
		SendMessage(update.ChatJoinRequest.From.ID, errMsg)
	}
}

func verify(update tgbotapi.Update) {
	userID := update.Message.From.ID

	if update.Message.IsCommand() && update.Message.Command() == "start" {
		bot.Send(tgbotapi.NewMessage(userID, "Hello! Please send your Solana wallet address so the bot could verify your account."))

		return
	}

	input := update.Message.Text
	publicKey, err := solana.PublicKeyFromBase58(input)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(userID, "Please provide a valid wallet address"))

		return
	}

	// Get owned verified NFTs
	mintsDict := map[string][]string{}
	for _, col := range collections {
		if mintsDict[col.ID], err = only1sdk.GetOwnedVerifiedMints(context.Background(), conn, publicKey, &col.Mints); err != nil {
			SendMessage(userID, errMsg)

			continue
		}
	}

	hasNfts := false
	msgHeader := ""
	for id, mints := range mintsDict {
		if len(mints) > 0 {
			hasNfts = true
			msgHeader += fmt.Sprintf("%s NFT in your wallet: *%d\n*", collections[id].Name, len(mints))
		}
	}

	if !hasNfts {
		bot.Send(tgbotapi.NewMessage(userID, "Unfortunatelly you don't own any of the NFTs that can be verified."))

		return
	}

	// Verify wallet ownership
	amount := only1sdk.RandSmallAmountOfSol()
	amountUi := float64(amount) / float64(solana.LAMPORTS_PER_SOL)
	msg := tgbotapi.NewMessage(userID, fmt.Sprintf("%s\nNext step: Wallet ownership verification\n\nPlease send `%.9f` *SOL* from _%s_ to _%s_ (the same wallet) so we can identify you as the owner of the wallet.\n\nIf this operation is not accomplished within 10 minutes, verification process will fail.", msgHeader, amountUi, input, input))
	msg.ParseMode = tgbotapi.ModeMarkdown
	bot.Send(msg)

	var success bool
	func() {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute*10))
		defer cancel()

		for {
			if deadline, _ := ctx.Deadline(); deadline.Unix() < time.Now().Unix() {
				return
			}
			success, _ = only1sdk.FindOwnershipTransfer(ctx, conn, publicKey, amount)
			if success {
				return
			}
			time.Sleep(time.Second * 3)
		}
	}()

	if err != nil {
		SendMessage(userID, errMsg)

		return
	}

	if !success {
		bot.Send(tgbotapi.NewMessage(userID, "Verification timed out. Try again later."))

		return
	}

	bot.Send(tgbotapi.NewMessage(userID, "Success! Thank you for the effort."))

	if success = func() bool {
		membersToRemove := map[string][]int64{}
		for colID, mints := range mintsDict {
			for _, m := range mints {
				data, err := repo.GetMint(m)
				if err != nil {
					return false
				}
				if data != nil {
					if data.User != userID {
						membersToRemove[colID] = append(membersToRemove[colID], data.User)
					}
				}
				if err = repo.SetMint(m, input, userID); err != nil {
					return false
				}
			}

			all, err := repo.GetAllMints()
			if err != nil {
				return false
			}
			for _, uCol := range membersToRemove {
				for _, u := range uCol {
					remove := true
					for _, d := range all {
						if d.User == u {
							remove = false

							break
						}
					}

					if remove {
						if _, err = bot.Request(tgbotapi.BanChatMemberConfig{ChatMemberConfig: tgbotapi.ChatMemberConfig{
							ChatID: collections[colID].ChatID,
							UserID: u,
						}}); err != nil {
							log.Panicf("failed to ban user: %v", err)

							return false
						}
						bot.Send(tgbotapi.NewMessage(u, fmt.Sprintf("You have been removed from the group %s", collections[colID].Name)))
					}
				}
			}
		}

		return true
	}(); !success {
		SendMessage(userID, errMsg)

		return
	}

	// Create invite links
	for id := range mintsDict {
		col := collections[id]

		if _, err := bot.Request(tgbotapi.UnbanChatMemberConfig{
			ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: col.ChatID, UserID: userID},
			OnlyIfBanned:     true,
		}); err != nil {
			log.Printf("failed to unban user: %v", err)
		}

		msg = tgbotapi.NewMessage(
			update.Message.Chat.ID,
			fmt.Sprintf("Please use the following link to join *%s* private group:\n%s\n", col.Name, col.InviteLink),
		)
		msg.ParseMode = tgbotapi.ModeMarkdown
		if _, err = bot.Send(msg); err != nil {
			log.Printf("failed to send invite: %v", err)
		}
	}
}
