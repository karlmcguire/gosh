// Package gosh provides session handling with automatic timeouts and key-value
// storage.
package gosh

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrAlreadyExists is thrown when attempting to create/add a new session
	// and a session with that identifier already exists in the Room.
	ErrAlreadyExists = errors.New("session with that iden already exists")

	// ErrDoesntExist is thrown when attempting to access a session, but a
	// session with that identifier doesn't exist in the Room.
	ErrDoesntExist = errors.New("no session with that iden exists")

	// ErrKeyDoesntExist is thrown when attempting to access a key-value pair
	// inside of a session, but the key doesn't exist inside that session.
	ErrKeyDoesntExist = errors.New("that key wasn't found in the session")
)

type dispatcher struct {
	lifetime time.Duration
}

func (d *dispatcher) watch(iden string, ping chan struct{}, kill chan string) {
	left := d.lifetime

	for {
		select {
		case <-ping:
			left = d.lifetime
		case <-time.After(left):
			kill <- iden
			return
		}
	}
}

// Room holds multiple sessions.
type Room struct {
	mutex sync.Mutex

	sessions   map[string]map[string]string
	watchers   map[string]chan struct{}
	dispatcher *dispatcher
	killer     chan string
}

// NewRoom returns an empty Room. The lifetime param specifies how long each
// session inside the Room will live without activity. After the lifetime has
// expired, the session is automatically deleted from the Room.
func NewRoom(lifetime time.Duration) *Room {
	room := &Room{
		sessions:   make(map[string]map[string]string, 0),
		watchers:   make(map[string]chan struct{}),
		dispatcher: &dispatcher{lifetime},
		killer:     make(chan string, 0),
	}

	go room.killWatch()

	return room
}

func (r *Room) killWatch() {
	for {
		select {
		case iden := <-r.killer:
			err := r.Del(iden)
			if err != nil {
				// handle err
			}
		}
	}
}

func (r *Room) accessCheck(iden, key string) error {
	if _, ok := r.sessions[iden]; !ok {
		return ErrDoesntExist
	}
	if _, ok := r.watchers[iden]; !ok {
		return ErrDoesntExist
	}
	if key != "" {
		if _, ok := r.sessions[iden][key]; !ok {
			return ErrKeyDoesntExist
		}
	}
	return nil
}

// Add creates a new session identified by the iden param.
//
// Add returns an error if a session with that iden already exists.
func (r *Room) Add(iden string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, ok := r.sessions[iden]; ok {
		return ErrAlreadyExists
	}
	if _, ok := r.watchers[iden]; ok {
		return ErrAlreadyExists
	}

	r.sessions[iden] = make(map[string]string, 0)
	r.watchers[iden] = make(chan struct{}, 0)

	go r.dispatcher.watch(iden, r.watchers[iden], r.killer)

	return nil
}

// Get is for getting session values. The session is identified by the iden
// parameter. The key parameter is used to find the key-value pair, with the
// value being returned (if found).
//
// Get returns an error if the session doesn't exist or a value doesn't exist
// for the specified key.
func (r *Room) Get(iden, key string) (string, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if err := r.accessCheck(iden, key); err != nil {
		return "", err
	}

	r.watchers[iden] <- struct{}{}

	return r.sessions[iden][key], nil
}

// GetBatch is for getting multiple session values. The session is identified
// by the iden parameter. The key parameters are used to find their key-value
// pairs, with the values being returned in a string slice.
//
// GetBatch returns an error if the session doesn't exist or one of the values
// don't exist for the specified key.
func (r *Room) GetBatch(iden string, keys ...string) ([]string, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if err := r.accessCheck(iden, ""); err != nil {
		return nil, err
	}

	r.watchers[iden] <- struct{}{}

	var (
		values = make([]string, len(keys), len(keys))
		ok     bool
	)

	for i, k := range keys {
		if _, ok = r.sessions[iden][k]; !ok {
			return nil, ErrKeyDoesntExist
		}
		values[i] = r.sessions[iden][k]
	}

	return values, nil
}

// Set is for setting session key-value pairs. The session is identified by the
// iden parameter. The key-value pair is specified by the key and value
// parameters.
//
// Set returns an error if the session doesn't exist.
func (r *Room) Set(iden, key, value string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if err := r.accessCheck(iden, ""); err != nil {
		return err
	}

	r.watchers[iden] <- struct{}{}
	r.sessions[iden][key] = value

	return nil
}

// Del deletes the session specified by the iden parameter. It returns an error
// if the session doesn't exist.
func (r *Room) Del(iden string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if err := r.accessCheck(iden, ""); err != nil {
		return err
	}

	delete(r.sessions, iden)
	delete(r.watchers, iden)

	return nil
}
