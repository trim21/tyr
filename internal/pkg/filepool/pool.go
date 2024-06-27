package filepool

import (
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

func onEvict(key string, value *File) {
	_ = value.Close()
}

var pool = expirable.NewLRU[string, *File](128, onEvict, time.Minute*10)

// Open creates and returns a file item with given file path, flag and opening permission.
// It automatically creates an associated file pointer pool internally when it's called first time.
// It retrieves a file item from the file pointer pool after then.
func Open(path string, flag int, perm os.FileMode, ttl time.Duration) (file *File, err error) {
	key := fmt.Sprintf("%s&%d&%d&%d", path, flag, ttl, perm)
	item, ok := pool.Get(key)
	if ok {
		return item, nil
	}

	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}

	return &File{
		File: f,
		flag: flag,
		perm: perm,
		path: path,
		key:  key,
	}, nil
}

// File is an item in the pool.
type File struct {
	File *os.File
	key  string
	path string      // Absolute path of the file.
	perm os.FileMode // Permission for opening file.
	flag int         // Flash for opening file.
}

func (f *File) Close() error {
	return f.File.Close()
}

func (f *File) Release() {
	pool.Add(f.key, f)
}
