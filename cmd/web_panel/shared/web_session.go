package shared

import "sync"

var webSession = &sync.Map{}

func GetWebSession() *sync.Map {
	return webSession
}
