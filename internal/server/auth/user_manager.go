package auth

import (
	"sync/atomic"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	log "github.com/sirupsen/logrus"
)

type user struct {
	name     string
	password string
}

type UserManager struct {
	siteId   string
	cfg      *config.ClientAuthConfig
	users    atomic.Pointer[[]user]
	shutdown chan struct{} // nil if not reloading
}

// NewUserManager accepts a nil or disabled cfg, in which case no user is accepted
func NewUserManager(siteId string, cfg *config.ClientAuthConfig) (*UserManager, error) {
	m := &UserManager{
		siteId: siteId,
		cfg:    cfg,
	}
	users, err := m.loadUsers()
	if err != nil {
		return nil, err
	}
	m.users.Store(&users)

	if cfg != nil && cfg.Enabled && cfg.UsersFileReloadInterval != nil {
		m.shutdown = make(chan struct{})
		go m.reloadLoop(*cfg.UsersFileReloadInterval)
	}
	return m, nil
}

func (m *UserManager) loadUsers() ([]user, error) {
	var users []user
	if m.cfg == nil || !m.cfg.Enabled {
		return users, nil
	}
	for _, u := range m.cfg.Users {
		users = append(users, user{u.Name, u.Password})
	}
	if m.cfg.UsersFile != "" {
		fileUsers, err := config.LoadUsersFile(m.cfg.UsersFile)
		if err != nil {
			return nil, err
		}
		for _, u := range fileUsers {
			users = append(users, user{u.Name, u.Password})
		}
		log.Debugf("(%s) loaded %d users from file %+q", m.siteId, len(fileUsers), m.cfg.UsersFile)
	}
	return users, nil
}

func (m *UserManager) reloadLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			users, err := m.loadUsers()
			if err != nil {
				log.Errorf("(%s) Failed to reload auth user list: %v", m.siteId, err)
				continue
			}
			m.users.Store(&users)
		case <-m.shutdown:
			return
		}
	}
}

func (m *UserManager) Check(name, password string) bool {
	for _, u := range *m.users.Load() {
		if u.name == name && u.password == password {
			return true
		}
	}
	return false
}

func (m *UserManager) Shutdown() {
	if m.shutdown != nil {
		close(m.shutdown)
	}
}
