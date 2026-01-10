package service

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

type User struct {
	Id         string           `json:"id"`
	Name       string           `json:"name,omitempty"`
	Email      string           `json:"email,omitempty"`
	Password   string           `json:"password,omitempty"`
	Containers *[]ContainerInfo `json:"project,omitempty"`
}

type UserStore struct {
	mu    sync.RWMutex
	users map[string]*User
}

func NewUserStore() *UserStore {
	return &UserStore{users: make(map[string]*User)}
}

func ensureProjects(u *User) {
	if u.Containers == nil {
		u.Containers = &[]ContainerInfo{}
	}
}

func (s *UserStore) CreateUser(name, email, password string) string {
	newId := uuid.New().String()

	u := &User{
		Id:         newId,
		Name:       name,
		Email:      email,
		Password:   password,
		Containers: &[]ContainerInfo{},
	}

	s.mu.Lock()
	s.users[newId] = u
	s.mu.Unlock()

	return newId
}

func (s *UserStore) GetUser(id string) (*User, bool) {
	s.mu.RLock()
	u, ok := s.users[id]
	s.mu.RUnlock()

	if ok {
		ensureProjects(u)
	}
	return u, ok
}

func (s *UserStore) ListUsers() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		ensureProjects(u)
		out = append(out, u)
	}
	return out
}

type UserUpdate struct {
	Name     *string
	Email    *string
	Password *string
}

func (s *UserStore) UpdateUser(id string, upd UserUpdate) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	ensureProjects(u)

	if upd.Name != nil {
		u.Name = *upd.Name
	}
	if upd.Email != nil {
		u.Email = *upd.Email
	}
	if upd.Password != nil {
		u.Password = *upd.Password
	}

	return u, nil
}

func (s *UserStore) DeleteUser(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return false
	}
	delete(s.users, id)
	return true
}
