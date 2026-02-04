package tests

import (
	"errors"
	"fmt"
	"service-platform/cmd/web_panel/database"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

type userAccount struct {
	Fullname    string
	Username    string
	PhoneNumber string
	Email       string
	Password    string
}

// go test -v -timeout 60m ./tests/createUserAndSendAccountInEmail_test.go
func TestCreateUserAndSendAccountInEmail(t *testing.T) {
	err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	dkiUsers := getDKIDashboardUsers()
	if len(dkiUsers) == 0 {
		t.Fatal("No DKI users found")
	}

	dbWeb, err := database.InitAndCheckDB(
		config.WebPanel.Get().Database.Username,
		config.WebPanel.Get().Database.Password,
		config.WebPanel.Get().Database.Host,
		config.WebPanel.Get().Database.Port,
		"db_web_panel_gl", // config.WebPanel.Get().Database.Name,
	)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	err = createUserDashboardDKI(dbWeb, dkiUsers)
	if err != nil {
		t.Fatalf("Failed to create DKI dashboard users: %v", err)
	}
}

func createUserDashboardDKI(db *gorm.DB, users []userAccount) error {
	if db == nil {
		return errors.New("database connection is nil")
	}

	if len(users) == 0 {
		return errors.New("no users provided")
	}

	var dkiClientRole model.Role
	if err := db.Where("role_name = ?", "DKI - Client").First(&dkiClientRole).Error; err != nil {
		return err
	}

	for _, user := range users {
		var existingUser model.Admin
		if err := db.Where("email = ?", user.Email).First(&existingUser).Error; err == nil {
			// User already exists, skip creation
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var profileImage string
		if strings.Contains(user.Fullname, "CSNA") {
			profileImage = "uploads/admin/csna_small_jpg.jpg"
		} else {
			profileImage = "uploads/admin/dki.png"

		}

		newUser := model.Admin{
			Fullname:     user.Fullname,
			Username:     user.Username,
			Phone:        user.PhoneNumber,
			Email:        user.Email,
			Password:     fun.GenerateSaltedPassword(user.Password),
			Type:         0,
			Role:         int(dkiClientRole.ID),
			Status:       2,
			CreateBy:     1,
			UpdateBy:     1,
			LastLogin:    time.Now(),
			ProfileImage: profileImage,
		}

		if err := db.Create(&newUser).Error; err != nil {
			return err
		}
		fmt.Printf("Created user: %s with email: %s and password: %s\n", newUser.Fullname, newUser.Email, user.Password)

		if err := sendAccountDKIToEmail(
			newUser.Fullname,
			newUser.Username,
			newUser.Email,
			user.Password,
		); err != nil {
			fmt.Printf("Failed to send account details to %s: %v\n", newUser.Email, err)
		}
	}

	return nil
}

func sendAccountDKIToEmail(fullname, username, email, password string) error {
	subject := "[noreply] DKI Dashboard User Account Information"
	dashboardUrl := config.WebPanel.Get().App.WebPublicURL
	dashboardUrl = "https://dashboardms.csna4u.com:14427"

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
	        <mj-image src="%s/assets/self/img/logo_web.png" alt="Logo" css-class="brand-logo" />
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
		config.WebPanel.Get().Default.PT,
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

func getDKIDashboardUsers() []userAccount {
	return []userAccount{
		{
			Fullname:    "CSNA - Arip Pradana",
			Username:    "arip_pradana",
			PhoneNumber: "62822222222222222",
			Email:       "arippradana@csna4u.com",
			Password:    "DKIWebCSNAUser@2025#Sept123!!",
		},
		{
			Fullname:    "CSNA - Nuri",
			Username:    "nuriyanti_silaban",
			PhoneNumber: "6281010101010101010",
			Email:       "nuriyantisilaban@csna4u.com",
			Password:    "DKIWebCSNAUser@2025#Sept123!!",
		},
		{
			Fullname:    "DKI - Nugraha",
			Username:    "anu_nugraha",
			PhoneNumber: "628444444444444444",
			Email:       "anu.nugraha@gmail.com",
			Password:    "DKIDashboardUser@2025#123!!",
		},
		{
			Fullname:    "DKI - Hari",
			Username:    "hari",
			PhoneNumber: "628555555555555555",
			Email:       "hari201015@gmail.com",
			Password:    "DKIDashboardUser@2025#123!!",
		},
		{
			Fullname:    "DKI - Irfan",
			Username:    "irfan_dwi_setyawan",
			PhoneNumber: "6286666666666666666",
			Email:       "irfandwisetyawan08@gmail.com",
			Password:    "DKIDashboardUser@2025#123!!",
		},
		{
			Fullname:    "DKI - Fiqa",
			Username:    "fiqa_aji",
			PhoneNumber: "6287777777777777777",
			Email:       "fiqaaji20@gmail.com",
			Password:    "DKIDashboardUser@2025#123!!",
		},
		{
			Fullname:    "DKI - Wahyu",
			Username:    "wahyu_agung_rahmanto",
			PhoneNumber: "6288888888888888888",
			Email:       "wahyuagungrahmanto@gmail.com",
			Password:    "DKIDashboardUser@2025#123!!",
		},
		{
			Fullname:    "DKI - Astrid",
			Username:    "astrid_indah_herdianti",
			PhoneNumber: "6210111011011010110",
			Email:       "astridindahh.herdianti@gmail.com",
			Password:    "DKIDashboardUser@2025#123!!",
		},
		//
		//
		//
	}
}
