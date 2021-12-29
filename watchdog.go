package main

import (
	"context"
	"time"

	"github.com/gagliardetto/solana-go"
	only1sdk "github.com/only1nft/solana-sdk-go"
)

func watchdog(interval time.Duration) {
	for {
		mints, err := repo.GetAllMints()
		if err != nil {
			time.Sleep(time.Minute * 10)

			continue
		}

		membersToRemove := map[string][]int64{}
		for mint, data := range mints {
			pk, err := only1sdk.GetCurrentNFTOwner(context.Background(), conn, solana.MustPublicKeyFromBase58(mint))
			if err != nil {
				continue
			}
			col := GetCollection(mint)
			if pk.String() != data.PublicKey {
				membersToRemove[col.ID] = unique(append(membersToRemove[col.ID], data.User))
				repo.DeleteMint(mint)
			}
		}

		revokeAccess(membersToRemove)

		time.Sleep(interval)
	}
}
