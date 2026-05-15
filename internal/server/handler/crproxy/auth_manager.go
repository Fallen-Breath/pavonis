package crproxy

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type authUser struct {
	Name     string
	Password string
}

type authUserList []authUser

type authManager struct {
	siteId   string
	auth     *config.ContainerRegistryAuthConfig
	users    atomic.Value // type: authUserList
	running  bool
	shutdown chan bool
}

func newAuthManager(siteId string, auth *config.ContainerRegistryAuthConfig) (*authManager, error) {
	m := &authManager{
		siteId:   siteId,
		auth:     auth,
		shutdown: make(chan bool, 1),
	}
	users, err := m.buildList()
	if err != nil {
		return nil, err
	}
	m.users.Store(users)
	return m, nil
}

func (m *authManager) buildList() (authUserList, error) {
	var list authUserList
	if !m.auth.Enabled {
		return list, nil
	}
	for _, user := range m.auth.Users {
		list = append(list, authUser{user.Name, user.Password})
	}
	if m.auth.UsersFile != "" {
		configBuf, err := os.ReadFile(m.auth.UsersFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read users file: %v", err)
		}
		usersFile := config.UsersFile{}
		if err := yaml.Unmarshal(configBuf, &usersFile); err != nil {
			return nil, fmt.Errorf("failed to parse users file: %v", err)
		}
		for userIdx, user := range usersFile.Users {
			if err := config.ValidateUser(user); err != nil {
				return nil, fmt.Errorf("failed to validate user[%d]: %v", userIdx, err)
			}
			list = append(list, authUser{user.Name, user.Password})
		}
		log.Debugf("(%s) loaded %d users from file %+q", m.siteId, len(usersFile.Users), m.auth.UsersFile)
	}
	return list, nil
}

func (m *authManager) CheckForAuthorization(username, password string) bool {
	users := m.users.Load().(authUserList)
	for _, u := range users {
		if u.Name == username && u.Password == password {
			return true
		}
	}
	return false
}

// StartBackgroundReload starts the periodic users-file reload goroutine (if configured).
func (m *authManager) StartBackgroundReload() {
	interval := m.auth.UsersFileReloadInterval
	if !m.auth.Enabled || interval == nil {
		return
	}
	m.running = true
	go func() {
		ticker := time.NewTicker(*interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				newList, err := m.buildList()
				if err != nil {
					log.Errorf("(%s) Failed to reload auth user list: %v", m.siteId, err)
					continue
				}
				m.users.Store(newList)
			case <-m.shutdown:
				return
			}
		}
	}()
}

func (m *authManager) Shutdown() {
	if m.running {
		m.shutdown <- true
	}
}
