package fs

import (
	"io"
	"os"
)

// File is the interface compatible with os.File.
type File interface {
	io.Closer
	io.ReaderAt
	io.WriterAt

	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(size int64) error
}

// MmapFile represents a memory-mapped file.
type FileManager interface {
	File
	Type() string
	Slice(start int64, end int64) ([]byte, error)
}

// LockFile represents a lock file.
type LockFile interface {
	Unlock() error
}

// FileSystem represents a virtual file system.
type FileSystem interface {
	OpenFile(name string, flag int, perm os.FileMode) (FileManager, error)
	CreateLockFile(name string, perm os.FileMode) (LockFile, bool, error)
	Stat(name string) (os.FileInfo, error)
	Remove(name string) error
}
