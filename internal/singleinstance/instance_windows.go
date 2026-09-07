//go:build windows

package singleinstance

import "golang.org/x/sys/windows"

type winLock struct {
	handle  windows.Handle
	running bool
}

// Acquire creates a named mutex; ERROR_ALREADY_EXISTS means another instance is running.
func Acquire(name string) (Lock, error) {
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateMutex(nil, false, ptr)
	running := false
	if err == windows.ERROR_ALREADY_EXISTS {
		running = true
	} else if err != nil && h == 0 {
		return nil, err
	}
	return &winLock{handle: h, running: running}, nil
}

func (l *winLock) IsRunning() bool { return l.running }

func (l *winLock) Release() {
	if l.handle != 0 {
		windows.CloseHandle(l.handle)
		l.handle = 0
	}
}
