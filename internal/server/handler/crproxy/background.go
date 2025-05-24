package crproxy

import (
	log "github.com/sirupsen/logrus"
	"time"
)

func (h *proxyHandler) backgroundReloadThread() {
	interval := h.settings.Auth.UsersFileReloadInterval

	needsReloadThread := false
	needsReloadThread = needsReloadThread || (h.settings.Auth.Enabled && interval != nil)
	if !needsReloadThread {
		return
	}

	reloadAuthUserList := func() {
		newAuthUserList, err := buildAuthUserList(h.settings)
		if err != nil {
			log.Errorf("Failed to build auth user list: %v", err)
			return
		}

		h.authUsers.Store(newAuthUserList)
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			reloadAuthUserList()

		case <-h.shutdownChannel:
			break
		}
	}
}
