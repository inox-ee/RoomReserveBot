package api

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v3"
)

// key: {YYYY-MM-DD}_{room-name}_{timestamp}
// val: {HH:mm}_{HH:mm}_{user-name}

type Reserve struct {
	Room      string
	StartTime string
	EndTime   string
	User      string
}

func ParseReserveKV(key, val string) Reserve {
	k := strings.Split(key, "_")
	v := strings.Split(val, "_")
	return Reserve{
		Room:      k[1],
		StartTime: v[0],
		EndTime:   v[1],
		User:      v[2],
	}
}

func (reserve *Reserve) FormatKV() (string, string) {
	key := fmt.Sprintf("%s_%s_%d", time.Now().Format("2006-01-02"), reserve.Room, time.Now().Unix())
	val := fmt.Sprintf("%s_%s_%s", reserve.StartTime, reserve.EndTime, reserve.User)
	return key, val
}

func FormatViewByRoom(rsvs []Reserve) string {
	groupRsv := map[string][]string{}
	sort.Slice(rsvs, func(i, j int) bool { return rsvs[i].StartTime < rsvs[j].EndTime })
	for _, rsv := range rsvs {
		groupRsv[rsv.Room] = append(groupRsv[rsv.Room], fmt.Sprintf("\t`%s` ~ `%s` (by %s)", rsv.StartTime, rsv.EndTime, rsv.User))
	}
	var res string
	for _, room := range rooms {
		reserve := groupRsv[room.Name]
		res += fmt.Sprintf("%s[%s] :\n%s\n", room.Name, room.Description, strings.Join(reserve, "\n"))
	}
	return res
}

func (reserve *Reserve) IsConflict(srv *Server) (string, error) {
	reserved_user := ""

	srv.store.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02"), reserve.Room))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			// k := item.Key()
			err := item.Value(func(v []byte) error {
				t := strings.Split(string(v), "_")
				start_t := t[0]
				end_t := t[1]
				log.Println(reserve.StartTime, reserve.EndTime)
				log.Println(start_t, end_t)
				if start_t < reserve.EndTime && end_t > reserve.StartTime {
					reserved_user = t[2]
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	var err error
	if reserved_user != "" {
		err = errors.New("予約が重複しています")
	}

	return reserved_user, err
}
