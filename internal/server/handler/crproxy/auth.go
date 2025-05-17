package crproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"net/http"
	"os"
	"strings"
)

type authUser struct {
	Name     string
	Password string
}

type authUserList []authUser

func buildAuthUserList(settings *config.ContainerRegistrySettings) (authUserList, error) {
	var authUserList []authUser
	if settings.Auth.Enabled {
		for _, user := range settings.Auth.Users {
			authUserList = append(authUserList, authUser{user.Name, user.Password})
		}
		if settings.Auth.UsersFile != "" {
			configBuf, err := os.ReadFile(settings.Auth.UsersFile)
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
				authUserList = append(authUserList, authUser{user.Name, user.Password})
			}
			log.Debugf("loaded %d users from file %+q", len(usersFile.Users), settings.Auth.UsersFile)
		}
	}
	return authUserList, nil
}

func parseBasicAuth(r *http.Request) (selfUser, selfPassword string, upstreamUser, upstreamPassword *string, ok bool) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return
	}

	splitString := func(s string) (string, *string) {
		parts := strings.SplitN(s, "$", 2)
		if len(parts) != 2 {
			return s, nil
		} else {
			return parts[0], &parts[1]
		}
	}

	selfUser, upstreamUser = splitString(username)
	selfPassword, upstreamPassword = splitString(password)
	return
}
