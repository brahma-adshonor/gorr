package gorr

import (
	"errors"
	"flag"
	"fmt"
	"github.com/boltdb/bolt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	bolt_db_big_value_thresh    = flag.Int("bolt_db_big_value_thresh", 1024*1024, "value size than threshold will be store to a single file")
	gorr_bolt_bucket_name = flag.String("gorr_bolt_bucket_name", "global_bucket", "bucket name used by gorr in bolt db")
)

type MapStorage struct {
	mu sync.Mutex
	m  map[string][]byte
}

func NewMapStorage(capacity int) *MapStorage {
	s := MapStorage{m: make(map[string][]byte, 10000)}
	return &s
}

func (s *MapStorage) Put(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.m[key] = value
	return nil
}

func (s *MapStorage) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, exist := s.m[key]
	if !exist {
		return nil, errors.New("key not exists")
	}

	return v, nil
}

func (s *MapStorage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = make(map[string][]byte)
}

func (s *MapStorage) Close() {
}

func (s *MapStorage) AllFiles() []string {
	return nil
}

type BoltStorage struct {
	db   *bolt.DB
	mu   sync.Mutex
	file map[string]int
}

// bolt key/value db
func NewBoltStorage(path string) (*BoltStorage, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}

	_ = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(*gorr_bolt_bucket_name))
		if err == bolt.ErrBucketExists {
			return nil
		}
		return err
	})

	s := &BoltStorage{db: db, file: make(map[string]int)}
	return s, nil
}

func genBigValueFileName() string {
	prefix := "gorr.file.db." + time.Now().Format("20060102150405")

	path := prefix
	for i := 0; i < 32; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}

		path = fmt.Sprintf("%s.%d", prefix, i)
	}

	return ""
}

func (s *BoltStorage) Put(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*gorr_bolt_bucket_name))
		value = append(value, byte('m'))

		if len(value) > *bolt_db_big_value_thresh {
			file, err := s.GetBigValueFile(key)
			if err != nil {
				file = genBigValueFileName()
			}

			path := filepath.Dir(s.db.Path()) + "/" + file
			err = ioutil.WriteFile(path, value, 0644)
			if err == nil {
				err = b.Put([]byte(key), []byte(file+"p"))
			}

			s.file[path] = 1
			return err
		} else {
			err := b.Put([]byte(key), value)
			return err
		}
	})
	return err
}

func (s *BoltStorage) GetBigValueFile(key string) (string, error) {
	var ret []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*gorr_bolt_bucket_name))
		ret = b.Get([]byte(key))
		return nil
	})

	if err != nil {
		return "", err
	}

	sz := len(ret)
	if sz == 0 {
		return "", fmt.Errorf("invalid value from db")
	}

	if ret[sz-1] == 'p' {
		file := string(ret[:sz-1])
		return file, nil
	}

	return "", fmt.Errorf("big value not exist")
}

func (s *BoltStorage) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var ret []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*gorr_bolt_bucket_name))
		ret = b.Get([]byte(key))
		return nil
	})

	if err != nil {
		return nil, err
	}

	sz := len(ret)
	if sz == 0 {
		return nil, fmt.Errorf("invalid value from db")
	}

	if ret[sz-1] == 'p' {
		file := string(ret[:sz-1])
		path := filepath.Dir(s.db.Path()) + "/" + file
		ret, err = ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read file by path from db failed, path:%s, err:%s", path, err.Error())
		}
	}

	return ret[:len(ret)-1], nil
}

func (s *BoltStorage) Clear() {
}

func (s *BoltStorage) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.db.Close()
	s.db = nil
}

func (s *BoltStorage) AllFiles() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	all := []string{s.db.Path()}
	for k := range s.file {
		all = append(all, k)
	}
	return all
}
