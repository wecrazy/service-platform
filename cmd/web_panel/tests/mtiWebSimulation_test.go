package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
)

// go test -v -timeout 60m ./tests/mtiWebSimulation_test.go
func TestMTIWebSimulation(t *testing.T) {
	WebSimulationTest()
}

func WebSimulationTest() {
	// Start browser
	// url := launcher.New().Headless(true).MustLaunch()
	url := launcher.New().
		Bin("/snap/bin/chromium").
		Headless(true).
		MustLaunch()

	browser := rod.New().ControlURL(url).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("http://localhost:2220/login")

	// Fill inputs
	emailEl := page.MustElement("#email")
	emailEl.MustInput("admin@webpanel.com")

	// Get value back
	valObj, _ := emailEl.Eval("() => this.value")
	valStr := fmt.Sprintf("%v", valObj.Value)
	fmt.Println("Email field value:", valStr)

	passwordEl := page.MustElement("#password")
	passwordEl.MustInput("Ro224171222#")

	valObj2, _ := passwordEl.Eval("() => this.value")
	valStr2 := fmt.Sprintf("%v", valObj2.Value)
	fmt.Println("Password field value:", valStr2)

	// Submit by pressing Enter
	page.KeyActions().Press(input.Enter).MustDo()

	// Wait a bit for page to update
	time.Sleep(3 * time.Second)

	// in here get the swal2

	// ✅ Example 1: print page title
	title := page.MustInfo().Title
	fmt.Println("📄 Page Title:", title)

	// ✅ Example 2: get text of an element that should appear
	// Adjust selector according to what appears on success (like #welcome-message)
	el, err := page.Timeout(5 * time.Second).Element("#welcome-message")
	if err == nil {
		text, _ := el.Text()
		fmt.Println("✅ Login Success Message:", text)
	} else {
		fmt.Println("⚠️ Login success message not found")
	}

	// ✅ Example 3: print current URL
	currentURL := page.MustInfo().URL
	fmt.Println("🌐 Current URL:", currentURL)
}
