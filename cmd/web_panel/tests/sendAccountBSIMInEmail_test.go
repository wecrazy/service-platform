package tests

import (
	"fmt"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"strings"
	"testing"
)

// go test -v -timeout 60m ./tests/sendAccountBSIMInEmail_test.go
func TestCreateUserAndSendAccountBSIMInEmail(t *testing.T) {
	err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// email := "lanitahinari570@gmail.com"
	// email := "acquiring.business@banksinarmas.com"
	// email := "acquiring.product@banksinarmas.com"
	// email := "maintenance.pinpad@banksinarmas.com"
	// email := "merchantcare@banksinarmas.com"
	// email := "merchantprocessing@banksinarmas.com"
	email := ""

	if err := sendAccountBSIMToEmail(
		"BSIM User 7",
		"dashboard_user_bank_sinarmas",
		email,
		"UserDashboardBSIM1234#@!!",
	); err != nil {
		t.Errorf("Failed to send account email to %s: %v", email, err)
	}
}

func sendAccountBSIMToEmail(fullname, username, email, password string) error {
	subject := "[noreply] BSIM Dashboard User Account Information"
	dashboardUrl := "https://dashboardms.csna4u.com:14426"

	var sb strings.Builder
	sb.WriteString("<mjml>")
	sb.WriteString(`
	  <mj-head>
	    <mj-preview>Account created ...........</mj-preview>
	    <mj-style inline="inline">
				.main-card {
					background: #fff;
					border-radius: 18px;
					box-shadow: 0 4px 24px rgba(0,0,0,0.10);
					padding: 40px 32px 32px 32px;
					margin: 0 auto;
				}
	      .header-title {
	        font-size: 28px;
	        font-weight: 700;
	        color: #4F46E5;
	        margin-bottom: 8px;
	        text-align: center;
	      }
	      .subtitle {
	        font-size: 16px;
	        color: #64748B;
	        text-align: center;
	        margin-bottom: 24px;
	      }
		.account-table {
			width: 100%;
			border-collapse: separate;
			border-spacing: 0;
			margin: 24px 0 32px 0;
			background: #F8FAFC;
			border-radius: 12px;
			overflow: hidden;
		}
	      .account-table th, .account-table td {
	        padding: 14px 18px;
	        font-size: 16px;
	        text-align: left;
	      }
	      .account-table th {
	        background: #F1F5F9;
	        color: #334155;
	        font-weight: 600;
	        border-bottom: 1px solid #E2E8F0;
	      }
	      .account-table tr:nth-child(even) td {
	        background: #F1F5F9;
	      }
		.cta-button {
			background-color: #b9b7d3;
			color: #fff !important;
			padding: 16px 32px;
			border-radius: 12px;
			font-size: 18px;
			font-weight: 700;
			text-align: center;
			display: block;
			margin: 0 auto 24px auto;
			text-decoration: none;
			box-shadow: 0 2px 8px rgba(79,70,229,0.10);
		}
	      .footer-text {
	        color: #6b7280;
	        font-size: 13px;
	        text-align: center;
	        padding-top: 18px;
	        border-top: 1px solid #e5e7eb;
	      }
	      .brand-logo {
	        display: block;
	        margin: 0 auto 24px auto;
	        width: 80px;
	      }
	    </mj-style>
	  </mj-head>
	`)

	sb.WriteString(fmt.Sprintf(`
	  <mj-body background-color="#F3F4F6">
	    <mj-section>
	      <mj-column>
	        <mj-image src="%s/assets/self/img/csna.png" alt="Logo" css-class="brand-logo" />
	        <mj-text css-class="header-title main-card">Hello, %s!</mj-text>
	        <mj-text css-class="subtitle main-card">
	          Your dashboard account has been created.<br/>Please find your login details below:
	        </mj-text>
	        <mj-table css-class="account-table main-card">
	          <tr>
	            <th>Dashboard URL</th>
	            <td><a href="%s/login">%s/login</a></td>
	          </tr>
	          <tr>
	            <th>Email</th>
	            <td>%s</td>
	          </tr>
	          <tr>
	            <th>Username</th>
	            <td>%s</td>
	          </tr>
	          <tr>
	            <th>Password</th>
	            <td>%s</td>
	          </tr>
	        </mj-table>
	        <mj-button css-class="cta-button main-card" href="%s/login">Go to Dashboard</mj-button>
	        <mj-text font-size="15px" color="#64748B" align="center" padding-bottom="0" css-class="main-card">
	          Please change your password by clicking the <b>'Lupa Kata Sandi'</b> button on the login page.<br/>
	          Or use this link to reset your password:<br/>
	          <a href="%s/forgot-password">%s/forgot-password</a>
	        </mj-text>
	        <mj-divider border-color="#e5e7eb" css-class="main-card" />
	        <mj-text font-size="16px" color="#374151" align="center" css-class="main-card">
	          Best Regards,<br>
	          <b><i>%s IT Team</i></b>
	        </mj-text>
	        <mj-text css-class="footer-text">
	          ⚠ This is an automated email. Please do not reply directly.<br>
	          For support, contact: <a href="mailto:support@csna4u.com">support@csna4u.com</a>
	        </mj-text>
	      </mj-column>
	    </mj-section>
	  </mj-body>
	  </mjml>
	`,
		dashboardUrl,
		fullname,
		dashboardUrl, dashboardUrl,
		email,
		username,
		password,
		dashboardUrl,
		dashboardUrl, dashboardUrl,
		config.GetConfig().Default.PT,
	))

	mjmlTemplate := sb.String()
	var emailTo []string
	emailTo = append(emailTo, email)

	err := fun.TrySendEmail(emailTo, nil, nil, subject, mjmlTemplate, nil)
	if err != nil {
		return err
	}

	return nil
}
