package tests

import (
	"fmt"
	"strings"
	"testing"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
)

// go test -v -timeout 60m ./tests/sendEmail_test.go
func TestSendToYahooMail(t *testing.T) {
	err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	dataToSend := []struct {
		Name  string
		Email string
	}{
		{Name: "Wegil", Email: "wegirandol@smartwebindonesia.com"},
	}

	if len(dataToSend) == 0 {
		t.Log("No data found to send")
	}

	for i, data := range dataToSend {
		fmt.Printf("\nProcessing index-%d [%s - %s]\n", i, data.Name, data.Email)

		// purpose := "Test data to Inbox"
		// url := "google"

		var sb strings.Builder
		sb.WriteString("<mjml>")
		sb.WriteString(`
		<mj-head>
			<mj-preview>Test auto inbox via email serv.....</mj-preview>
			<mj-style inline="inline">
			.body-section {
				background-color: #ffffff;
				padding: 30px;
				border-radius: 12px;
				box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
			}
			.footer-text {
				color: #6b7280;
				font-size: 12px;
				text-align: center;
				padding-top: 10px;
				border-top: 1px solid #e5e7eb;
			}
			.header-title {
				font-size: 66px;
				font-weight: bold;
				color: #1E293B;
				text-align: left;
			}
			.cta-button {
				background-color: #6D28D9;
				color: #ffffff;
				padding: 12px 24px;
				border-radius: 8px;
				font-size: 16px;
				font-weight: bold;
				text-align: center;
				display: inline-block;
			}
			.email-info {
				color: #374151;
				font-size: 16px;
			}
			</mj-style>
		</mj-head>`)

		sb.WriteString(fmt.Sprintf(`
		<mj-body background-color="#f8fafc">
			<!-- Main Content -->
			<mj-section css-class="body-section" padding="20px">
			<mj-column>
				<mj-text font-size="20px" color="#1E293B" font-weight="bold">Yth. Sdr(i) %v</mj-text>
				<mj-text font-size="16px" color="#4B5563" line-height="1.6">
								Halo saya wegil ingin bertanya tentang edc saya yang mengalami problem sebagai berikut:
				- kendala: tidak bisa melakukan transaksi <br>
				- keterangan : ingin segera diganti saja gess <br>

				- no hp: 085173207755555 <br>
				</mj-text>

				<mj-divider border-color="#e5e7eb"></mj-divider>

				<mj-text font-size="16px" color="#374151">
				Best Regards,<br>
				<b><i>%v</i></b>
				</mj-text>
			</mj-column>
			</mj-section>

			<!-- Footer -->
			<mj-section>
			<mj-column>
				<mj-text css-class="footer-text">
				⚠ This is an automated email. Please do not reply directly.
				</mj-text>
				<mj-text font-size="12px" color="#6b7280">
				<b>IT Team.</b><br>
				<!--
				<br>
				<a href="wa.me/%v">
				📞 Support
				</a>
				-->
				</mj-text>
			</mj-column>
			</mj-section>

		</mj-body>
		`,
			strings.ToUpper(data.Name),
			config.GetConfig().Default.PT,
			"085123456789",
		))
		sb.WriteString("</mjml>")

		mjmlTemplate := sb.String()

		var emailTo []string
		emailTo = append(emailTo, strings.ToLower(data.Email))

		err := fun.TrySendEmail(emailTo, nil, nil, "TEST MASUK INBOX "+fun.GenerateRandomString(10), mjmlTemplate, nil)
		if err != nil {
			t.Log(err)
		}
	}

}

// func TestYahooMail(t *testing.T) {
// 	email := "wegirandol@smartwebindonesia.com"
// 	password := "bnelwrlowhmnjrxd"

// 	if email == "" || password == "" {
// 		t.Fatal("Missing YAHOO_EMAIL or YAHOO_APP_PASSWORD environment variable")
// 	}

// 	// Connect to Yahoo IMAP server
// 	c, err := client.DialTLS("imap.mail.yahoo.com:993", nil)
// 	if err != nil {
// 		t.Fatalf("Failed to connect to Yahoo IMAP: %v", err)
// 	}
// 	defer c.Logout()

// 	// Login
// 	if err := c.Login(email, password); err != nil {
// 		t.Fatalf("Failed to login: %v", err)
// 	}

// 	// Select INBOX
// 	mbox, err := c.Select("INBOX", false)
// 	if err != nil {
// 		t.Fatalf("Failed to select INBOX: %v", err)
// 	}

// 	// Fetch last 5 messages
// 	from := uint32(1)
// 	to := mbox.Messages
// 	if mbox.Messages > 5 {
// 		from = mbox.Messages - 4
// 	}
// 	seqset := new(imap.SeqSet)
// 	seqset.AddRange(from, to)

// 	messages := make(chan *imap.Message, 5)

// 	fmt.Println(len(messages))

// 	done := make(chan error, 1)
// 	go func() {
// 		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
// 	}()

// 	for msg := range messages {
// 		fmt.Printf("From: %v | Subject: %v", msg.Envelope.From, msg.Envelope.Subject)
// 	}

// 	if err := <-done; err != nil {
// 		t.Fatalf("Error fetching messages: %v", err)
// 	}
// }

// func TestSendEmail(t *testing.T) {
// 	// Test to send email for Dashboard TA View Data account
// 	config.LoadConfig()

// 	dataSPL := []struct {
// 		Name  string
// 		Email string
// 	}{
// 		// {Name: "Wegil", Email: "wegirandol@smartwebindonesia.com"},
// 		// {Name: "Lanita", Email: "lanitahinari570@gmail.com"},
// 		// {Name: "Maman Hidayat", Email: "manbose53@gmail.com"},
// 		// {Name: "Ridha Kurniawan", Email: "ridhakurniawan79@gmail.com"},
// 		// {Name: "Ahmad Soni", Email: "ahmadsoni.jktc6760@gmail.com"},
// 		// {Name: "Andi setiawan", Email: "setiawan122173@gmail.com"},
// 		// {Name: "Andry", Email: "anggraini.pgcd@gmail.com"},
// 		// {Name: "Putu oka semara yasa", Email: "ta.denpasar@gmail.com"},
// 		// {Name: "Juan Rizky Romeiro", Email: "juanrizkyromeiro@gmail.com"},
// 		// {Name: "Fuad Dwi Cahyanto", Email: "fuaddwic@gmail.com"},
// 		// {Name: "Gilar Darmawan", Email: "gilar1979@gmail.com"},
// 		// {Name: "Randy irwan", Email: "randyirwan20@gmail.com"},
// 		// {Name: "Bryan Sisco Pantas", Email: "pantassisco03@gmail.com"},
// 		// {Name: "Zefrian", Email: "zeffry39@gmail.com"},
// 		// {Name: "Wawan Setiawan", Email: "wawan081075@gmail.com"},
// 		// {Name: "Muhammad ikhsan", Email: "ghaisan296@gmail.com"},
// 		// {Name: "Bintang Mahardika", Email: "dhika@csnams.com"},
// 		// {Name: "Muflih Isnain", Email: "muflih.isnain1@gmail.com"},
// 		// {Name: "Abdul Muiz", Email: "A.muiz1201@gmail.com"},
// 		// {Name: "Roni Koswara", Email: "Aron.hideung4@gmail.com"},
// 		// {Name: "Syahrul Rohim", Email: "syahrulrohim24@gmail.com"},
// 		// {Name: "Rizal Wilti Samitrapura", Email: "Samitrapura86@gmail.com"},
// 		// {Name: "M. Zaenun Fathurozi", Email: "Zaldy1401@gmail.com"},
// 		// {Name: "Hafiz Rahman Hidayat", Email: "harizsyakib@gmail.com"},
// 		// {Name: "Agus Sasmito", Email: "agus.sasmit99@gmail.com"},
// 		// {Name: "Bagas Yusa", Email: "bagasyusa33@gmail.com"},
// 		// {Name: "Fedro Auzzreal Nur Hidayat", Email: "fedroauzzrealnurhidayat@gmail.com"},
// 		// {Name: "Hizbul Wathon", Email: "Hizbul86@gmail.com"},
// 		// {Name: "Upu Mahbuby", Email: "upuabyasa1802@gmail.com"},
// 		// {Name: "Rizal Feviadi", Email: "rfeviadi@gmail.com"},
// 		// {Name: "Iqbal Haris", Email: "iqbalharissyahfitri@gmail.com"},
// 		// {Name: "Rahmaddianto", Email: "muhrahmaddianto@gmail.com"},
// 		// {Name: "Buleleng Idham", Email: "idhamk857@gmail.com"},
// 		// {Name: "Agung Gede Teguh Adi Swayadnya", Email: "ariantoputra1971@gmail.com"},
// 		// {Name: "Polinus Gea", Email: "polinusgea0904@gmail.com"},
// 		// {Name: "Tirta Kunawan", Email: "tierthagangga@gmail.com"},
// 	}

// 	if len(dataSPL) == 0 {
// 		t.Log("No data found for send its email")
// 	}

// 	for i, data := range dataSPL {
// 		fmt.Printf("\nProcessing index-%d [%s]\n", i, data.Name)

// 		purpose := "Dashboard View TA (Technical Assistance) Data"
// 		url := "http://manageserviceai.csna4u.com:22425/forgot-password"

// 		var sb strings.Builder
// 		sb.WriteString("<mjml>")
// 		sb.WriteString(`
// 		<mj-head>
// 			<mj-preview>Link for Dashboard</mj-preview>
// 			<mj-style inline="inline">
// 			.body-section {
// 				background-color: #ffffff;
// 				padding: 30px;
// 				border-radius: 12px;
// 				box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
// 			}
// 			.footer-text {
// 				color: #6b7280;
// 				font-size: 12px;
// 				text-align: center;
// 				padding-top: 10px;
// 				border-top: 1px solid #e5e7eb;
// 			}
// 			.header-title {
// 				font-size: 66px;
// 				font-weight: bold;
// 				color: #1E293B;
// 				text-align: left;
// 			}
// 			.cta-button {
// 				background-color: #6D28D9;
// 				color: #ffffff;
// 				padding: 12px 24px;
// 				border-radius: 8px;
// 				font-size: 16px;
// 				font-weight: bold;
// 				text-align: center;
// 				display: inline-block;
// 			}
// 			.email-info {
// 				color: #374151;
// 				font-size: 16px;
// 			}
// 			</mj-style>
// 		</mj-head>`)

// 		sb.WriteString(fmt.Sprintf(`
// 		<mj-body background-color="#f8fafc">
// 			<!-- Main Content -->
// 			<mj-section css-class="body-section" padding="20px">
// 			<mj-column>
// 				<mj-text font-size="20px" color="#1E293B" font-weight="bold">Yth. Sdr(i) %v</mj-text>
// 				<mj-text font-size="16px" color="#4B5563" line-height="1.6">
// 				Berikut kami lampirkan link dan akun untuk <i><b>%v</b></i>.
// 				<br> <br>
// 				URL: <a href="%s">Link URL</a>
// 				<br>
// 				Email: %v
// 				<br> <br>
// 				<em>Dari URL tersebut, nantinya akan diminta untuk melakukan setup password yang baru agar dapat login kedalam dashboard.</em>
// 				</mj-text>

// 				<mj-divider border-color="#e5e7eb"></mj-divider>

// 				<mj-text font-size="16px" color="#374151">
// 				Best Regards,<br>
// 				<b><i>%v</i></b>
// 				</mj-text>
// 			</mj-column>
// 			</mj-section>

// 			<!-- Footer -->
// 			<mj-section>
// 			<mj-column>
// 				<mj-text css-class="footer-text">
// 				⚠ This is an automated email. Please do not reply directly.
// 				</mj-text>
// 				<mj-text font-size="12px" color="#6b7280">
// 				<b>IT Team.</b><br>
// 				<!--
// 				<br>
// 				<a href="wa.me/%v">
// 				📞 Support
// 				</a>
// 				-->
// 				</mj-text>
// 			</mj-column>
// 			</mj-section>

// 		</mj-body>
// 		`, strings.ToUpper(data.Name), purpose, url, strings.ToLower(data.Email), config.GetConfig().Default.PT, "085123456789"))
// 		sb.WriteString("</mjml>")

// 		mjmlTemplate := sb.String()

// 		var emailTo []string
// 		emailTo = append(emailTo, strings.ToLower(data.Email))

// 		err := fun.TrySendEmail(emailTo, nil, nil, "Link Dashboard TA", mjmlTemplate, nil)
// 		if err != nil {
// 			t.Log(err)
// 		}
// 	}
// }
