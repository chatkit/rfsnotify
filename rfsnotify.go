// Package rfsnotify implements recursive folder monitoring by wrapping the excellent fsnotify library
package rfsnotify

import (
	"gopkg.in/fsnotify.v1"
	"os"
	"path/filepath"
)

// RWatcher wraps fsnotify.Watcher. When fsnotify adds recursive watches, you should be able to switch your code to use fsnotify.Watcher
type RWatcher struct {
	Events chan fsnotify.Event
	Errors chan error

	done     chan bool
	fsnotify *fsnotify.Watcher
}

// NewWatcher establishes a new watcher with the underlying OS and begins waiting for events.
func NewWatcher() (*RWatcher, error) {
	fsWatch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	m := &RWatcher{}
	m.fsnotify = fsWatch
	m.Events = make(chan fsnotify.Event)
	m.Errors = make(chan error)
	m.done = make(chan bool)

	go m.start()

	return m, nil
}

// Add starts watching the named file or directory (non-recursively).
func (m *RWatcher) Add(name string) error {
	return m.fsnotify.Add(name)
}

// AddRecursive starts watching the named file or directory (recursively).
func (m *RWatcher) AddRecursive(name string) error {
	if err := m.watchRecursive(name); err != nil {
		return err
	}
	return nil
}

// Remove stops watching the the named file or directory (non-recursively).
func (m *RWatcher) Remove(name string) error {
	return m.fsnotify.Remove(name)
}

// Close removes all watches and closes the events channel.
func (m *RWatcher) Close() error {
	m.done <- true
	return nil
}

func (m *RWatcher) start() {
	defer close(m.done)
	for {
		select {

		case e := <-m.fsnotify.Events:
			s, err := os.Stat(e.Name)
			if err == nil && s != nil && s.IsDir() {
				if e.Op&fsnotify.Create != 0 {
					m.watchRecursive(e.Name)
				}
			}
			//Can't stat a deleted directory, so just pretend that it's always a directory and
			//try to remove from the watch list...  we really have no clue if it's a directory or not...
			if e.Op&fsnotify.Remove != 0 {
				m.fsnotify.Remove(e.Name)
			}
			m.Events <- e

		case e := <-m.fsnotify.Errors:
			m.Errors <- e

		case <-m.done:
			m.fsnotify.Close()
			close(m.Events)
			close(m.Errors)
			return
		}
	}
}

// watchRecursive adds all directories under the given one to the watch list.
// this is probably a very racey process. What if a file is added to a folder before we get the watch added?
func (m *RWatcher) watchRecursive(path string) error {
	err := filepath.Walk(path, func(walkPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if fi.IsDir() {
			err = m.fsnotify.Add(walkPath)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}
