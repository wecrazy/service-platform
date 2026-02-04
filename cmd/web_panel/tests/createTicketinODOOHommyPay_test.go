package tests

import (
	"service-platform/cmd/web_panel/controllers"
	"service-platform/internal/config"
	"testing"
)

func TestCreateTicketDatainODOOHommyPay(t *testing.T) {
	config.LoadConfig()
	config.WebPanel.Get()

	controllers.InsertDataTicketInODOOHommyPay()
}
