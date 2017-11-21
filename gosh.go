package gosh

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrAlreadyExists  = errors.New("session with that iden already exists")
	ErrDoesntExist    = errors.New("no session with that iden exists")
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

type Room struct {
	mutex sync.Mutex

	sessions   map[string]map[string]string
	watchers   map[string]chan struct{}
	dispatcher *dispatcher
	killer     chan string
}

func NewRoom(live time.Duration) *Room {
	room := &Room{
		sessions:   make(map[string]map[string]string, 0),
		watchers:   make(map[string]chan struct{}),
		dispatcher: &dispatcher{live},
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

func (r *Room) Get(iden, key string) (string, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if err := r.accessCheck(iden, key); err != nil {
		return "", err
	}

	r.watchers[iden] <- struct{}{}

	return r.sessions[iden][key], nil
}

func (r *Room) Set(iden, key, value string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if err := r.accessCheck(iden, ""); err != nil {
		return err
	}

	r.sessions[iden][key] = value
	r.watchers[iden] <- struct{}{}

	return nil
}

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
