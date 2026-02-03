package tests

import (
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/controllers"
	"testing"
)

func TestCreateTicketDatainODOOHommyPay(t *testing.T) {
	config.LoadConfig()
	config.GetConfig()

	controllers.InsertDataTicketInODOOHommyPay()
}
