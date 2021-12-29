package main

import (
	"encoding/json"
	"log"
	"strings"

	badger "github.com/dgraph-io/badger/v3"
)

type Repository struct {
	Db *badger.DB
}

type Data struct {
	PublicKey string
	User      int64
}

const mintPrefix = "mint:"

func (r *Repository) SetString(key, value string) (err error) {
	if err = r.Db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), []byte(value))
		return txn.SetEntry(e)
	}); err != nil {
		log.Printf("failed to save data: %v", err)
	}

	return err
}

func (r *Repository) GetString(key string) (out string, err error) {
	r.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			out = string(val[:])

			return nil
		})
	})

	return out, err
}

func (r *Repository) GetAllMints() (out map[string]Data, err error) {
	out = map[string]Data{}
	if err := r.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())
			if !strings.HasPrefix(key, mintPrefix) {
				continue
			}
			if item.IsDeletedOrExpired() {
				continue
			}
			if err := item.Value(func(val []byte) error {
				var data Data
				err = json.Unmarshal(val, &data)
				if err != nil {
					log.Printf("failed to unmarshall data: %v", err)

					return err
				}
				mint := strings.TrimPrefix(key, mintPrefix)
				out[mint] = data
				return nil
			}); err != nil {
				log.Printf("failed to retrieve data: %v", err)

				return err
			}
		}

		return nil
	}); err != nil {
		log.Printf("failed to retrieve data for all mints: %v", err)
	}

	return out, err
}

func (r *Repository) GetMint(mint string) (data *Data, err error) {
	if err = r.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(mintPrefix + mint))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			log.Printf("failed to retrieve data: %v", err)

			return err
		}

		return item.Value(func(val []byte) error {
			err = json.Unmarshal(val, &data)
			if err != nil {
				log.Printf("failed to unmarshall data: %v", err)
			}

			return err
		})
	}); err != nil {
		log.Printf("failed to retrieve data: %v", err)
	}

	return data, err
}

func (r *Repository) SetMint(mint, publicKey string, user int64) (err error) {
	if err = r.Db.Update(func(txn *badger.Txn) error {
		o := Data{PublicKey: publicKey, User: user}
		val, err := json.Marshal(o)
		if err != nil {
			log.Printf("failed to marshall data to json: %v", err)

			return err
		}
		e := badger.NewEntry([]byte(mintPrefix+mint), val)
		return txn.SetEntry(e)
	}); err != nil {
		log.Printf("failed to save data: %v", err)
	}

	return err
}

func (r *Repository) DeleteMint(mint string) (err error) {
	if err = r.Db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(mintPrefix + mint))
	}); err != nil {
		log.Printf("failed to delete data: %v", err)
	}

	return err
}
