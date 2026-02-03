package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

/*
Interactive Message Functions for WhatsApp Bot using waE2E

This file contains functions to create interactive messages with buttons and lists
using the new waE2E package instead of the deprecated waProto.

Available Functions:
- SendInteractiveMessageButtons: Creates messages with up to 3 quick reply buttons
- SendInteractiveMessageList: Creates messages with selectable list/menu items
- SendQuickReplyButtons: Alternative button implementation
- SendSimpleTextWithButtons: Fallback for numbered text options

Usage Examples:
1. Simple Buttons: SendFruitSelectionButtons(jid)
2. Service Menu: SendServiceMenu(jid)
3. Language Selection: SendLanguageSelection(jid)
4. Technical Support: SendTechnicalSupportMenu(jid)

Note: Interactive messages may not work on all WhatsApp clients or versions.
Always provide fallback options for older clients.
*/

// InteractiveButton represents a button in an interactive message
type InteractiveButton struct {
	DisplayText string `json:"display_text"`
	ID          string `json:"id"`
}

// InteractiveSection represents a section in an interactive list
type InteractiveSection struct {
	Title string           `json:"title"`
	Rows  []InteractiveRow `json:"rows"`
}

// InteractiveRow represents a row in an interactive list section
type InteractiveRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// SendInteractiveMessageButtons sends an interactive message with buttons using waE2E
// This function creates a message with quick reply buttons that users can tap to respond
// Falls back to regular text if interactive messages aren't supported
func SendInteractiveMessageButtons(jid string, title, content, footer string, buttons []InteractiveButton) error {
	logrus.Debugf("SendInteractiveMessageButtons called with JID: %s, Title: %s", jid, title)

	// Check WhatsApp client connection
	if WhatsappClient == nil {
		return fmt.Errorf("WhatsApp client is nil")
	}
	if !WhatsappClient.IsConnected() {
		return fmt.Errorf("WhatsApp client is not connected")
	}

	parsedJID, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("failed to parse JID: %v", err)
	}

	userJID := types.JID{
		User:   parsedJID.User,
		Server: parsedJID.Server,
	}

	logrus.Debugf("Parsed JID: User=%s, Server=%s", userJID.User, userJID.Server)

	// Create interactive buttons - using ButtonsMessage for simple buttons
	var buttonsList []*waE2E.ButtonsMessage_Button
	for i, btn := range buttons {
		buttonsList = append(buttonsList, &waE2E.ButtonsMessage_Button{
			ButtonID: proto.String(btn.ID),
			ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{
				DisplayText: proto.String(btn.DisplayText),
			},
			Type: waE2E.ButtonsMessage_Button_RESPONSE.Enum(),
		})
		// WhatsApp typically supports max 3 buttons
		if i >= 2 {
			break
		}
	}

	// Create the buttons message
	msg := &waE2E.Message{
		ButtonsMessage: &waE2E.ButtonsMessage{
			ContentText: proto.String(content),
			FooterText:  proto.String(footer),
			Buttons:     buttonsList,
			HeaderType:  waE2E.ButtonsMessage_TEXT.Enum(),
		},
	}

	// If title is provided, prepend it to content
	if title != "" {
		msg.ButtonsMessage.ContentText = proto.String(title + "\n\n" + content)
	}

	logrus.Debugf("Sending interactive buttons message to %s with %d buttons", userJID.String(), len(buttonsList))
	resp, err := WhatsappClient.SendMessage(context.Background(), userJID, msg)
	if err != nil {
		logrus.Errorf("Failed to send interactive message to %s: %v", userJID.String(), err)
		return fmt.Errorf("failed to send interactive message: %v", err)
	}

	logrus.Infof("Successfully sent interactive buttons message to %s, MessageID: %s", userJID.String(), resp.ID)
	return nil
}

// SendButtonsAsText sends button options as regular text message (fallback)
func SendButtonsAsText(jid string, title, content, footer string, buttons []InteractiveButton) {
	var message strings.Builder

	if title != "" {
		message.WriteString("*" + title + "*\n\n")
	}

	message.WriteString(content + "\n\n")

	for i, btn := range buttons {
		message.WriteString(fmt.Sprintf("%d. %s\n", i+1, btn.DisplayText))
	}

	if footer != "" {
		message.WriteString("\n" + footer)
	}

	message.WriteString("\n\n_Reply with the number of your choice_")

	sendTextMessageViaBot(jid, message.String())
	logrus.Infof("Sent buttons as text message to %s", jid)
}

// SendInteractiveMessageList sends an interactive message with a list/menu using waE2E
// Note: For list messages, we'll use ListMessage which might be available in newer versions
// If not available, we'll fall back to regular buttons
func SendInteractiveMessageList(jid string, title, content, footer, buttonText string, sections []InteractiveSection) error {
	logrus.Debugf("SendInteractiveMessageList called with JID: %s, Title: %s", jid, title)

	// Check WhatsApp client connection
	if WhatsappClient == nil {
		return fmt.Errorf("WhatsApp client is nil")
	}
	if !WhatsappClient.IsConnected() {
		return fmt.Errorf("WhatsApp client is not connected")
	}

	parsedJID, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("failed to parse JID: %v", err)
	}

	userJID := types.JID{
		User:   parsedJID.User,
		Server: parsedJID.Server,
	}

	logrus.Debugf("Parsed JID for list: User=%s, Server=%s", userJID.User, userJID.Server)

	// Create list sections using ListMessage
	var listSections []*waE2E.ListMessage_Section
	for _, section := range sections {
		var rows []*waE2E.ListMessage_Row
		for _, row := range section.Rows {
			rows = append(rows, &waE2E.ListMessage_Row{
				RowID:       proto.String(row.ID),
				Title:       proto.String(row.Title),
				Description: proto.String(row.Description),
			})
		}
		listSections = append(listSections, &waE2E.ListMessage_Section{
			Title: proto.String(section.Title),
			Rows:  rows,
		})
	}

	// Create the list message
	msg := &waE2E.Message{
		ListMessage: &waE2E.ListMessage{
			Title:       proto.String(title),
			Description: proto.String(content),
			ButtonText:  proto.String(buttonText),
			FooterText:  proto.String(footer),
			Sections:    listSections,
			ListType:    waE2E.ListMessage_SINGLE_SELECT.Enum(),
		},
	}

	logrus.Debugf("Sending interactive list message to %s with %d sections", userJID.String(), len(listSections))
	resp, err := WhatsappClient.SendMessage(context.Background(), userJID, msg)
	if err != nil {
		logrus.Errorf("Failed to send interactive list message to %s: %v", userJID.String(), err)
		return fmt.Errorf("failed to send interactive list message: %v", err)
	}

	logrus.Infof("Successfully sent interactive list message to %s, MessageID: %s", userJID.String(), resp.ID)
	return nil
}

// SendListAsText sends list options as regular text message (fallback)
func SendListAsText(jid string, title, content, footer string, sections []InteractiveSection) {
	var message strings.Builder

	if title != "" {
		message.WriteString("*" + title + "*\n\n")
	}

	message.WriteString(content + "\n\n")

	counter := 1
	for _, section := range sections {
		if section.Title != "" {
			message.WriteString("📋 *" + section.Title + "*\n")
		}

		for _, row := range section.Rows {
			message.WriteString(fmt.Sprintf("%d. %s", counter, row.Title))
			if row.Description != "" {
				message.WriteString(" - " + row.Description)
			}
			message.WriteString("\n")
			counter++
		}
		message.WriteString("\n")
	}

	if footer != "" {
		message.WriteString(footer + "\n")
	}

	message.WriteString("_Reply with the number of your choice_")

	sendTextMessageViaBot(jid, message.String())
	logrus.Infof("Sent list as text message to %s", jid)
}

// SendInteractiveButtonsWithFallback tries to send interactive buttons, falls back to text
func SendInteractiveButtonsWithFallback(jid string, title, content, footer string, buttons []InteractiveButton) error {
	logrus.Infof("Attempting to send interactive buttons to %s", jid)

	// Try interactive message first
	err := SendInteractiveMessageButtons(jid, title, content, footer, buttons)
	if err != nil {
		logrus.Warnf("Interactive buttons failed, using text fallback: %v", err)
		SendButtonsAsText(jid, title, content, footer, buttons)
		return nil // Don't return error since we sent text version
	}

	return nil
}

// SendInteractiveListWithFallback tries to send interactive list, falls back to text
func SendInteractiveListWithFallback(jid string, title, content, footer, buttonText string, sections []InteractiveSection) error {
	logrus.Infof("Attempting to send interactive list to %s", jid)

	// Try interactive message first
	err := SendInteractiveMessageList(jid, title, content, footer, buttonText, sections)
	if err != nil {
		logrus.Warnf("Interactive list failed, using text fallback: %v", err)
		SendListAsText(jid, title, content, footer, sections)
		return nil // Don't return error since we sent text version
	}

	return nil
}

// SendSimpleTextWithButtons sends a simple text message followed by button options
// This is a fallback for cases where interactive messages might not work
func SendSimpleTextWithButtons(jid string, message string, options []string) error {
	parsedJID, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("failed to parse JID: %v", err)
	}

	userJID := types.JID{
		User:   parsedJID.User,
		Server: parsedJID.Server,
	}

	// Build message with numbered options
	var sb strings.Builder
	sb.WriteString(message)
	sb.WriteString("\n\n")

	for i, option := range options {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, option))
	}

	sb.WriteString("\nPlease reply with the number of your choice.")

	// Send as regular text message
	msg := &waE2E.Message{
		Conversation: proto.String(sb.String()),
	}

	_, err = WhatsappClient.SendMessage(context.Background(), userJID, msg)
	if err != nil {
		return fmt.Errorf("failed to send text message with options: %v", err)
	}

	return nil
}

// SendQuickReplyButtons sends a message with quick reply buttons (simpler than interactive messages)
func SendQuickReplyButtons(jid string, message string, buttons []InteractiveButton) error {
	parsedJID, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("failed to parse JID: %v", err)
	}

	userJID := types.JID{
		User:   parsedJID.User,
		Server: parsedJID.Server,
	}

	// Create quick reply buttons
	var quickReplyButtons []*waE2E.ButtonsMessage_Button
	for i, btn := range buttons {
		quickReplyButtons = append(quickReplyButtons, &waE2E.ButtonsMessage_Button{
			ButtonID: proto.String(btn.ID),
			ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{
				DisplayText: proto.String(btn.DisplayText),
			},
			Type: waE2E.ButtonsMessage_Button_RESPONSE.Enum(),
		})
		// WhatsApp supports max 3 buttons
		if i >= 2 {
			break
		}
	}

	msg := &waE2E.Message{
		ButtonsMessage: &waE2E.ButtonsMessage{
			ContentText: proto.String(message),
			Buttons:     quickReplyButtons,
			HeaderType:  waE2E.ButtonsMessage_TEXT.Enum(),
		},
	}

	_, err = WhatsappClient.SendMessage(context.Background(), userJID, msg)
	if err != nil {
		return fmt.Errorf("failed to send quick reply buttons: %v", err)
	}

	return nil
}

// Example usage functions for your WhatsApp bot

// SendFruitSelectionButtons sends an example interactive message with fruit selection buttons
func SendFruitSelectionButtons(jid string) error {
	buttons := []InteractiveButton{
		{DisplayText: "Apple 🍎", ID: "fruit_apple"},
		{DisplayText: "Banana 🍌", ID: "fruit_banana"},
		{DisplayText: "Orange 🍊", ID: "fruit_orange"},
	}

	return SendInteractiveButtonsWithFallback(
		jid,
		"🍉 Choose a fruit",
		"Select one of the fruits below:",
		"Only one choice allowed.",
		buttons,
	)
}

// SendMenuList sends an example interactive message with a menu list
func SendMenuList(jid string) error {
	sections := []InteractiveSection{
		{
			Title: "Main Courses",
			Rows: []InteractiveRow{
				{ID: "pasta", Title: "Pasta", Description: "Delicious Italian pasta"},
				{ID: "pizza", Title: "Pizza", Description: "Fresh baked pizza"},
			},
		},
		{
			Title: "Desserts",
			Rows: []InteractiveRow{
				{ID: "cake", Title: "Cake", Description: "Chocolate cake"},
				{ID: "ice_cream", Title: "Ice Cream", Description: "Vanilla ice cream"},
			},
		},
	}

	return SendInteractiveListWithFallback(
		jid,
		"🍽️ Restaurant Menu",
		"Choose from our delicious menu:",
		"Powered by WhatsApp Bot",
		"View Menu",
		sections,
	)
}

// SendTechnicalSupportMenu sends a technical support menu with buttons
func SendTechnicalSupportMenu(jid string) error {
	buttons := []InteractiveButton{
		{DisplayText: "🔧 Troubleshoot", ID: "tech_troubleshoot"},
		{DisplayText: "📞 Call Support", ID: "tech_call"},
		{DisplayText: "📧 Email Support", ID: "tech_email"},
	}

	return SendInteractiveButtonsWithFallback(
		jid,
		"🛠️ Technical Support",
		"How can we help you today?",
		"Choose an option below:",
		buttons,
	)
}

// SendLanguageSelection sends language selection buttons
func SendLanguageSelection(jid string) error {
	buttons := []InteractiveButton{
		{DisplayText: "🇺🇸 English", ID: "lang_en"},
		{DisplayText: "🇮🇩 Bahasa Indonesia", ID: "lang_id"},
	}

	return SendInteractiveButtonsWithFallback(
		jid,
		"🌐 Language Selection",
		"Please select your preferred language:",
		"Pilih bahasa yang Anda inginkan:",
		buttons,
	)
}

// SendServiceMenu sends a service menu using list format
func SendServiceMenu(jid string) error {
	sections := []InteractiveSection{
		{
			Title: "Customer Service",
			Rows: []InteractiveRow{
				{ID: "cs_billing", Title: "Billing Support", Description: "Questions about your bill"},
				{ID: "cs_technical", Title: "Technical Support", Description: "Technical issues and troubleshooting"},
				{ID: "cs_general", Title: "General Inquiry", Description: "General questions and information"},
			},
		},
		{
			Title: "Self Service",
			Rows: []InteractiveRow{
				{ID: "self_status", Title: "Check Status", Description: "Check your service status"},
				{ID: "self_payment", Title: "Make Payment", Description: "Pay your bills online"},
				{ID: "self_report", Title: "Report Issue", Description: "Report service issues"},
			},
		},
	}

	return SendInteractiveListWithFallback(
		jid,
		"📋 Service Menu",
		"Select the service you need:",
		"Customer Support Team",
		"🔽 Select Service",
		sections,
	)
}

// SendBusinessHoursMenu sends business hours and contact menu
func SendBusinessHoursMenu(jid string) error {
	sections := []InteractiveSection{
		{
			Title: "Business Hours",
			Rows: []InteractiveRow{
				{ID: "hours_weekday", Title: "Weekday Hours", Description: "Monday - Friday: 9:00 AM - 6:00 PM"},
				{ID: "hours_weekend", Title: "Weekend Hours", Description: "Saturday: 10:00 AM - 4:00 PM"},
				{ID: "hours_holiday", Title: "Holiday Schedule", Description: "Check our holiday operating hours"},
			},
		},
		{
			Title: "Contact Information",
			Rows: []InteractiveRow{
				{ID: "contact_phone", Title: "Phone Support", Description: "Call us for immediate assistance"},
				{ID: "contact_email", Title: "Email Support", Description: "Send us your questions via email"},
				{ID: "contact_office", Title: "Office Location", Description: "Visit our office location"},
			},
		},
	}

	return SendInteractiveListWithFallback(
		jid,
		"🏢 Business Information",
		"Get information about our business hours and contact details:",
		"Business Support Team",
		"📋 View Information",
		sections,
	)
}

// SendProductCatalog sends a product catalog menu
func SendProductCatalog(jid string) error {
	sections := []InteractiveSection{
		{
			Title: "Technology Services",
			Rows: []InteractiveRow{
				{ID: "prod_web", Title: "Web Development", Description: "Custom websites and web applications"},
				{ID: "prod_mobile", Title: "Mobile Apps", Description: "iOS and Android mobile applications"},
				{ID: "prod_cloud", Title: "Cloud Solutions", Description: "Cloud hosting and infrastructure"},
			},
		},
		{
			Title: "Support Services",
			Rows: []InteractiveRow{
				{ID: "prod_maintenance", Title: "Maintenance", Description: "Ongoing system maintenance"},
				{ID: "prod_consulting", Title: "Consulting", Description: "Technical consulting services"},
				{ID: "prod_training", Title: "Training", Description: "Staff training and workshops"},
			},
		},
	}

	return SendInteractiveListWithFallback(
		jid,
		"🛍️ Product Catalog",
		"Explore our range of products and services:",
		"Sales Team",
		"🔍 Browse Catalog",
		sections,
	)
}

// HandleInteractiveButtonResponse handles responses from interactive buttons
// You can call this function from your message processing logic
func HandleInteractiveButtonResponse(buttonID string, senderJID string) {
	switch buttonID {
	// Fruit selection responses
	case "fruit_apple":
		sendTextMessageViaBot(senderJID, "🍎 Great choice! You selected Apple. Apples are rich in vitamins!")
	case "fruit_banana":
		sendTextMessageViaBot(senderJID, "🍌 Excellent! You chose Banana. Bananas are packed with potassium!")
	case "fruit_orange":
		sendTextMessageViaBot(senderJID, "🍊 Nice pick! Orange is full of Vitamin C!")

	// Language selection responses
	case "lang_en":
		handleLanguageChange(senderJID, "en")
	case "lang_id":
		handleLanguageChange(senderJID, "id")

	// Technical support responses
	case "tech_troubleshoot":
		sendTextMessageViaBot(senderJID, "🔧 Let me help you troubleshoot. Please describe your issue in detail.")
	case "tech_call":
		sendTextMessageViaBot(senderJID, "📞 Please call our support hotline: +62-XXX-XXXX-XXXX")
	case "tech_email":
		sendTextMessageViaBot(senderJID, "📧 Send your inquiry to: support@yourcompany.com")

	// Customer service responses
	case "cs_billing":
		sendTextMessageViaBot(senderJID, "💰 Billing Support: Please provide your account number for billing assistance.")
	case "cs_technical":
		sendTextMessageViaBot(senderJID, "🔧 Technical Support: Describe your technical issue and we'll help you resolve it.")
	case "cs_general":
		sendTextMessageViaBot(senderJID, "❓ General Inquiry: How can we assist you today?")

	// Self service responses
	case "self_status":
		sendTextMessageViaBot(senderJID, "📊 Your service status: Active ✅\nNext billing date: 15th November 2025")
	case "self_payment":
		sendTextMessageViaBot(senderJID, "💳 Payment options:\n1. Bank Transfer\n2. Credit Card\n3. Mobile Payment")
	case "self_report":
		sendTextMessageViaBot(senderJID, "⚠️ Report Issue: Please describe the issue you're experiencing.")

	// Restaurant menu responses
	case "pasta":
		sendTextMessageViaBot(senderJID, "🍝 You selected Pasta! Our pasta is made fresh daily with authentic Italian recipes.")
	case "pizza":
		sendTextMessageViaBot(senderJID, "🍕 You chose Pizza! Our pizzas are wood-fired and made with the finest ingredients.")
	case "cake":
		sendTextMessageViaBot(senderJID, "🍰 Delicious Cake! Our chocolate cake is made with premium Belgian chocolate.")
	case "ice_cream":
		sendTextMessageViaBot(senderJID, "🍦 Ice Cream! Our vanilla ice cream is made with real vanilla beans.")

	// Business hours responses
	case "hours_weekday":
		sendTextMessageViaBot(senderJID, "🕘 Weekday Hours:\nMonday - Friday: 9:00 AM - 6:00 PM\nWe're here to serve you during business days!")
	case "hours_weekend":
		sendTextMessageViaBot(senderJID, "🕙 Weekend Hours:\nSaturday: 10:00 AM - 4:00 PM\nSunday: Closed\nLimited weekend support available.")
	case "hours_holiday":
		sendTextMessageViaBot(senderJID, "🎄 Holiday Schedule:\nPlease check our website for current holiday hours or call our office.")

	// Contact information responses
	case "contact_phone":
		sendTextMessageViaBot(senderJID, "📞 Phone Support:\nMain Line: +62-XXX-XXXX-XXXX\nSupport Hours: Mon-Fri 9AM-6PM")
	case "contact_email":
		sendTextMessageViaBot(senderJID, "📧 Email Support:\nGeneral: info@yourcompany.com\nSupport: support@yourcompany.com\nSales: sales@yourcompany.com")
	case "contact_office":
		sendTextMessageViaBot(senderJID, "🏢 Office Location:\nJl. Example Street No. 123\nJakarta, Indonesia\nVisiting hours: Mon-Fri 9AM-5PM")

	// Product catalog responses
	case "prod_web":
		sendTextMessageViaBot(senderJID, "💻 Web Development:\nCustom websites, e-commerce platforms, and web applications.\nContact our sales team for a quote!")
	case "prod_mobile":
		sendTextMessageViaBot(senderJID, "📱 Mobile Apps:\niOS and Android development with modern frameworks.\nLet's build your mobile solution!")
	case "prod_cloud":
		sendTextMessageViaBot(senderJID, "☁️ Cloud Solutions:\nAWS, Google Cloud, and Azure implementations.\nScale your business with cloud technology!")
	case "prod_maintenance":
		sendTextMessageViaBot(senderJID, "🔧 Maintenance Services:\nOngoing system maintenance and monitoring.\nKeep your systems running smoothly!")
	case "prod_consulting":
		sendTextMessageViaBot(senderJID, "👥 Consulting Services:\nTechnical consulting and architecture design.\nLet our experts guide your project!")
	case "prod_training":
		sendTextMessageViaBot(senderJID, "📚 Training Services:\nStaff training and technical workshops.\nEmpower your team with knowledge!")

	// Default response
	default:
		sendTextMessageViaBot(senderJID, "❓ Unknown button response. Please try again or contact support.")
	}
}

// ProcessInteractiveMessage processes incoming interactive messages
// Call this function from your main message handler
func ProcessInteractiveMessage(messageText string, senderJID string) bool {
	messageTextLower := strings.ToLower(strings.TrimSpace(messageText))

	switch messageTextLower {
	case "/menu", "menu":
		SendServiceMenu(senderJID)
		return true
	case "/support", "support":
		SendTechnicalSupportMenu(senderJID)
		return true
	case "/language", "language", "/lang", "lang":
		SendLanguageSelection(senderJID)
		return true
	case "/fruit", "fruit":
		SendFruitSelectionButtons(senderJID)
		return true
	case "/restaurant", "restaurant", "/food", "food":
		SendMenuList(senderJID)
		return true
	case "/business", "business", "/hours", "hours":
		SendBusinessHoursMenu(senderJID)
		return true
	case "/products", "products", "/catalog", "catalog":
		SendProductCatalog(senderJID)
		return true
	case "/help", "help":
		sendHelpMessage(senderJID)
		return true
	}

	return false
}

// sendHelpMessage sends a help message with available commands
func sendHelpMessage(senderJID string) {
	helpText := `🤖 *WhatsApp Bot Commands*

*Interactive Menus:*
• /menu - Service menu
• /support - Technical support
• /language - Language selection
• /business - Business hours & contact
• /products - Product catalog

*Examples:*
• /fruit - Fruit selection demo
• /restaurant - Restaurant menu demo

*General:*
• /help - Show this help

Simply type any of these commands to get started! 🚀`

	sendTextMessageViaBot(senderJID, helpText)
}

// Note: handleLanguageChange is implemented in whatsmeow_controllers.go
// It handles language preference changes and saves to database

// TestInteractiveMessages is a helper function to test all interactive message types
// You can call this from your admin panel or for testing purposes
func TestInteractiveMessages(jid string) {
	logrus.Info("Testing all interactive message types...")

	// Test all the different interactive menus
	functions := []struct {
		name string
		fn   func(string) error
	}{
		{"Service Menu", SendServiceMenu},
		{"Technical Support Menu", SendTechnicalSupportMenu},
		{"Language Selection", SendLanguageSelection},
		{"Business Hours Menu", SendBusinessHoursMenu},
		{"Product Catalog", SendProductCatalog},
		{"Fruit Selection (Demo)", SendFruitSelectionButtons},
		{"Restaurant Menu (Demo)", SendMenuList},
	}

	for i, test := range functions {
		logrus.Infof("Testing %d: %s", i+1, test.name)

		// Check WhatsApp client connection before sending
		if WhatsappClient == nil {
			logrus.Errorf("WhatsApp client is nil for %s", test.name)
			continue
		}
		if !WhatsappClient.IsConnected() {
			logrus.Errorf("WhatsApp client is not connected for %s", test.name)
			continue
		}

		if err := test.fn(jid); err != nil {
			logrus.Errorf("Failed to send %s: %v", test.name, err)
		} else {
			logrus.Infof("Successfully sent %s", test.name)
		}
		// Small delay to avoid rate limiting
		time.Sleep(2 * time.Second)
	}

	// Send help message at the end
	sendHelpMessage(jid)
}

// TestBasicMessage sends a simple text message to verify WhatsApp connection
func TestBasicMessage(jid string) error {
	logrus.Infof("Testing basic text message to %s", jid)

	if WhatsappClient == nil {
		return fmt.Errorf("WhatsApp client is nil")
	}
	if !WhatsappClient.IsConnected() {
		return fmt.Errorf("WhatsApp client is not connected")
	}

	// Try sending a simple text message first
	sendTextMessageViaBot(jid, "🧪 Test message: WhatsApp connection is working!")
	logrus.Infof("Basic test message sent successfully to %s", jid)
	return nil
}

// TestInteractiveMessagesFallback tests interactive messages with fallback to text
func TestInteractiveMessagesFallback(jid string) {
	logrus.Info("Testing interactive messages with fallback...")

	// First test basic connectivity
	if err := TestBasicMessage(jid); err != nil {
		logrus.Errorf("Basic message test failed: %v", err)
		return
	}

	// Wait a moment
	time.Sleep(3 * time.Second)

	// Try a simple button message first
	logrus.Info("Testing simple button message...")
	if err := SendLanguageSelection(jid); err != nil {
		logrus.Errorf("Button message failed, falling back to text: %v", err)
		sendTextMessageViaBot(jid, "🌐 Language Selection\n\nPlease select your preferred language:\n1. 🇺🇸 English\n2. 🇮🇩 Bahasa Indonesia\n\nReply with 1 or 2")
	} else {
		logrus.Info("Button message sent successfully")
	}

	time.Sleep(3 * time.Second)

	// Try a list message
	logrus.Info("Testing list message...")
	if err := SendServiceMenu(jid); err != nil {
		logrus.Errorf("List message failed, falling back to text: %v", err)
		sendTextMessageViaBot(jid, "📋 Service Menu\n\nCustomer Service:\n1. Billing Support\n2. Technical Support\n3. General Inquiry\n\nSelf Service:\n4. Check Status\n5. Make Payment\n6. Report Issue\n\nReply with the number of your choice")
	} else {
		logrus.Info("List message sent successfully")
	}
}
