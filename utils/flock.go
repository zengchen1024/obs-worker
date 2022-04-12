package utils

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

type FileOp interface {
	Read(b []byte) (n int, err error)
	ReadAt(b []byte, off int64) (n int, err error)
	ReadDir(n int) ([]os.DirEntry, error)
	ReadFrom(r io.Reader) (n int64, err error)
	Readdir(n int) ([]os.FileInfo, error)
	Readdirnames(n int) (names []string, err error)
	Seek(offset int64, whence int) (ret int64, err error)
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	Truncate(size int64) error
	Write(b []byte) (n int, err error)
	WriteAt(b []byte, off int64) (n int, err error)
	WriteString(s string) (n int, err error)
	Close() error
}

func tryLock(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_EX|syscall.LOCK_NB)
}

func lock(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_EX)
}

func unlock(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_UN)
}

type fileOnLock struct {
	*os.File
}

func (fl *fileOnLock) Close() error {
	fl.Sync()

	unlock(fl.Fd())

	return fl.File.Close()
}

func LockOpen(file string, flat int, mode os.FileMode) (op FileOp, err error) {
	f, err := os.OpenFile(file, flat, mode)
	if err != nil {
		return
	}

	if err = lock(f.Fd()); err != nil {
		err = fmt.Errorf("add lock failed, err:%v\n", err)

		f.Close()
		return
	}

	op = &fileOnLock{f}

	return
}
