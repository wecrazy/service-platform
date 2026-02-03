package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"service-platform/cmd/technical_assistance/model/op_model"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// ShowPhotoByID handles GET /photos/:id requests
func ShowPhotoByID(redisDB *redis.Client, db_pengerjaan *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id_task := ctx.Param("id")
		if id_task == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": "photo not found"})
			return
		}

		table := ctx.Query("table")
		if table == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": "unknown table to search the data"})
			return
		}

		var joData interface{}
		switch table {
		case "error":
			var errorData op_model.Error
			result := db_pengerjaan.Model(&op_model.Error{}).Where(&op_model.Error{IDTask: id_task}).First(&errorData)
			if result.Error != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"message": "failed to fetch data", "detail": result.Error.Error()})
				return
			}
			joData = errorData
		case "pending":
			var pendingData op_model.Pending
			result := db_pengerjaan.Model(&op_model.Pending{}).Where(&op_model.Pending{IDTask: id_task}).First(&pendingData)
			if result.Error != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"message": "failed to fetch data", "detail": result.Error.Error()})
				return
			}
			joData = pendingData
		case "temp_submission":
			var tempSubmissionData op_model.TempSubmission
			result := db_pengerjaan.Model(&op_model.TempSubmission{}).Where(&op_model.TempSubmission{IDTask: id_task}).First(&tempSubmissionData)
			if result.Error != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"message": "failed to fetch data", "detail": result.Error.Error()})
				return
			}
			joData = tempSubmissionData
		default:
			ctx.JSON(http.StatusBadRequest, gin.H{"message": "unknown table"})
			return
		}

		var teknisi, woNumber, ticketSubject, merchant, mid, tid string
		switch v := joData.(type) {
		case op_model.Error:
			teknisi = v.Teknisi
			woNumber = v.WoNumber
			ticketSubject = v.SpkNumber
			merchant = *v.Merchant
			mid = v.MID
			tid = v.TID
		case op_model.Pending:
			teknisi = v.Teknisi
			woNumber = v.WoNumber
			ticketSubject = v.SpkNumber
			merchant = *v.Merchant
			mid = v.MID
			tid = v.TID
		case op_model.TempSubmission:
			teknisi = v.Teknisi
			woNumber = v.WONumber
			ticketSubject = v.SPKNumber
			merchant = v.Merchant
			mid = v.MID
			tid = v.TID
		}

		id_foto := []string{
			"x_foto_bast", "x_foto_ceklis", "x_foto_edc", "x_foto_pic", "x_foto_setting",
			"x_foto_thermal", "x_foto_toko", "x_foto_training", "x_foto_transaksi",
			"x_tanda_tangan_pic", "x_tanda_tangan_teknisi",
			"x_foto_sticker_edc", "x_foto_screen_guard", "x_foto_all_transaction",
			"x_foto_transaksi_bmri", "x_foto_transaksi_bni", "x_foto_transaksi_bri",
			"x_foto_transaksi_btn", "x_foto_transaksi_patch", "x_foto_screen_p2g",
			"x_foto_kontak_stiker_pic",
			"x_foto_selfie_video_call", "x_foto_selfie_teknisi_merchant",
		}

		judul_foto := []string{
			"Foto BAST", "Foto Media Promo", "Foto SN EDC", "Foto PIC Merchant", "Foto Pengaturan",
			"Foto Thermal", "Foto Merchant", "Foto Surat Training", "Foto Transaksi",
			"Tanda Tangan PIC", "Tanda Tangan Teknisi",
			"Foto Stiker EDC", "Foto Screen Gard", "Foto Sales Draft All Memberbank",
			"Foto Sales Draft BMRI", "Foto Sales Draft BNI", "Foto Sales Draft BRI",
			"Foto Sales Draft BTN", "Foto Sales Draft Patch L", "Foto Screen P2G",
			"Foto Kontak Stiker PIC",
			"Foto Selfie Video Call", "Foto Selfie Teknisi dan Merchant",
		}

		var html strings.Builder
		html.WriteString(`
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Photo Gallery</title>
	<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
	<link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.5/font/bootstrap-icons.css" rel="stylesheet">
	<style>
		body {
			background-color: #f8f9fa;
		}
		.photo-card {
			transition: transform 0.2s ease-in-out;
		}
		.photo-card:hover {
			transform: scale(1.05);
			box-shadow: 0 4px 20px rgba(0,0,0,0.2);
		}
		.data-label {
			font-weight: 600;
			color: #343a40;
		}
	</style>
</head>
<body>
	<script>
		function openImagePopup(url) {
			const width = 400;
			const height = 700;
			const left = 0;
			const top = 20;

			window.open(
				url,
				'imagePopup',
				'width=' + width + ',height=' + height + ',left=' + left + ',top=' + top + ',resizable=yes,scrollbars=yes'
			);
		}
	</script>

	<div class="container mt-4">
		<h2 class="text-center mb-4">Photo Gallery for ID Task: ` + id_task + `</h2>

		<div class="card mb-4 shadow-sm border-0">
	<div class="card-body">
		<div class="row g-3">
			<div class="col-md-6 d-flex">
				<div class="me-3">
					<i class="bi bi-person-lines-fill fs-3 text-secondary"></i>
				</div>
				<div>
				<div><span class="fw-semibold text-muted">Ticket Subject:</span> ` + ticketSubject + `</div>
				<div><span class="fw-semibold text-muted">WO Number:</span> ` + woNumber + `</div>
				<div><span class="fw-semibold text-muted">Teknisi:</span> ` + teknisi + `</div>
				</div>
			</div>
			<div class="col-md-6 d-flex">
				<div class="me-3">
					<i class="bi bi-shop-window fs-3 text-secondary"></i>
				</div>
				<div>
					<div><span class="fw-semibold text-muted">Merchant:</span> ` + merchant + `</div>
					<div><span class="fw-semibold text-muted">MID:</span> ` + mid + `</div>
					<div><span class="fw-semibold text-muted">TID:</span> ` + tid + `</div>
				</div>
			</div>
		</div>
	</div>
</div>

		<div class="row row-cols-1 row-cols-sm-2 row-cols-md-3 g-4">
`)

		for i, id := range id_foto {
			html.WriteString(fmt.Sprintf(`
			<div class="col">
				<div class="card photo-card h-100 text-center">
					<img src="/here/file/%s@%s" 
						class="card-img-top" 
						alt="%s"
						style="height:250px; object-fit:contain; cursor:pointer;"
						onclick="openImagePopup(this.src);"
						onerror="this.onerror=null; this.src='/assets/self/img/no-img.jpg';">
					<div class="card-body">
						<h5 class="card-title">%s</h5>
					</div>
				</div>
			</div>
	`, id_task, id, judul_foto[i], judul_foto[i]))
		}

		html.WriteString(`
		</div>
		<div class="text-center mt-4">
		<!-- 
			<a href="/" class="btn btn-secondary">
				<i class="bi bi-arrow-left-circle me-1"></i> Back to Home
			</a>
			-->
		</div>
	</div>
	<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></>
</body>
</html>
`)

		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, html.String())

	}
}

// Get additional data from endpoint get data kukuh
type RequestPayloadToKukuh struct {
	IdTask string `json:"id_task"`
	Data   []struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Index int    `json:"index,omitempty"`
	} `json:"data"`
}

type AdditionalDataResponseFromKukuh struct {
	IdTask string `json:"id_task"`
	Result []struct {
		Name string      `json:"name"`
		Data interface{} `json:"data"`
	} `json:"result"`
}

func getAdditionalDatafromEndpointKukuh(idTask string) (map[string]interface{}, error) {
	// Prepare request payload
	payload := RequestPayloadToKukuh{
		IdTask: idTask,
		Data: []struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			Index int    `json:"index,omitempty"`
		}{
			// TID
			{Name: "x_cimb_master_tid", Type: "string"},
			{Name: "x_cimb_tid2", Type: "string"},
			{Name: "x_cimb_tid3", Type: "string"},
			{Name: "x_cimb_tid4", Type: "string"},
			{Name: "x_cimb_tid5", Type: "string"},
			{Name: "x_cimb_tid6", Type: "string"},
			{Name: "x_cimb_tid7", Type: "string"},
			{Name: "x_cimb_tid8", Type: "string"},
			{Name: "x_cimb_tiqr", Type: "string"},
			// MID
			{Name: "x_cimb_master_mid", Type: "string"},
			{Name: "x_cimb_mid2", Type: "string"},
			{Name: "x_cimb_mid3", Type: "string"},
			{Name: "x_cimb_mid4", Type: "string"},
			{Name: "x_cimb_mid5", Type: "string"},
			{Name: "x_cimb_mid6", Type: "string"},
			{Name: "x_cimb_mid7", Type: "string"},
			{Name: "x_cimb_mid8", Type: "string"},
			{Name: "x_cimb_midqr", Type: "string"},

			{Name: "partner_street", Type: "string"},
			{Name: "x_street2", Type: "string"},
			{Name: "x_street3", Type: "string"},
			{Name: "x_supply_thermal", Type: "integer"},
			{Name: "x_kanwil", Type: "string"},

			{Name: "x_history", Type: "string"},
			{Name: "x_software_version", Type: "string"},
			// {Name: "x_studio_edc", Type: "array", Index: 1},
			// {Name: "active", Type: "boolean"},
			// {Name: "fsm_task_count", Type: "integer"},
		},
	}

	// Encode request to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	// Make HTTP POST request
	resp, err := http.Post(os.Getenv("ENDPOINT_KUKUH_GET_DATA"), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var dataResp AdditionalDataResponseFromKukuh
	if err := json.Unmarshal(body, &dataResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert to map
	dataMap := make(map[string]interface{})
	for _, item := range dataResp.Result {
		dataMap[item.Name] = item.Data
	}

	return dataMap, nil
}

func ShowAdditionalDataByID(redisDB *redis.Client, db_pengerjaan *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id_task := ctx.Param("id")
		if id_task == "" {
			ctx.String(http.StatusBadRequest, "ID not found")
			return
		}

		dataMap, err := getAdditionalDatafromEndpointKukuh(id_task)
		if err != nil {
			ctx.String(http.StatusInternalServerError, fmt.Sprintf("Error: %v", err))
			return
		}

		var fieldLabels = map[string]string{
			// TID
			"x_cimb_master_tid": "Master TID",
			"x_cimb_tid2":       "TID 2",
			"x_cimb_tid3":       "TID 3",
			"x_cimb_tid4":       "TID 4",
			"x_cimb_tid5":       "TID 5",
			"x_cimb_tid6":       "TID 6",
			"x_cimb_tid7":       "TID 7",
			"x_cimb_tid8":       "TID 8",
			"x_cimb_tiqr":       "TID QR",

			// MID
			"x_cimb_master_mid": "Master MID",
			"x_cimb_mid2":       "MID 2",
			"x_cimb_mid3":       "MID 3",
			"x_cimb_mid4":       "MID 4",
			"x_cimb_mid5":       "MID 5",
			"x_cimb_mid6":       "MID 6",
			"x_cimb_mid7":       "MID 7",
			"x_cimb_mid8":       "MID 8",
			"x_cimb_midqr":      "MID QR",

			// Others
			"partner_street":   "Merchant Address",
			"x_street2":        "Merchant Address 2",
			"x_street3":        "Merchant Address 3",
			"x_supply_thermal": "Thermal Paper Supply",
			"x_kanwil":         "Wajib Supply Thermal",

			"x_history":          "Versi APK FS",
			"x_software_version": "Tanggal Visit Re-Schedule",
		}

		var orderedKeys = []string{
			// MID fields first
			"x_cimb_master_mid", "x_cimb_mid2", "x_cimb_mid3", "x_cimb_mid4",
			"x_cimb_mid5", "x_cimb_mid6", "x_cimb_mid7", "x_cimb_mid8", "x_cimb_midqr",

			// TID fields next
			"x_cimb_master_tid", "x_cimb_tid2", "x_cimb_tid3", "x_cimb_tid4",
			"x_cimb_tid5", "x_cimb_tid6", "x_cimb_tid7", "x_cimb_tid8", "x_cimb_tiqr",

			// others
			"partner_street",
			"x_street2",
			"x_street3",

			"x_supply_thermal",
			"x_kanwil",

			"x_history",
			"x_software_version",
		}

		// Build HTML
		htmlContent := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<title>Additional Data</title>
	<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
</head>
<body>
	<div class="container my-5">
		<h1 class="text-center mb-4">Additional Data for Task ID: ` + id_task + `</h1>
		<div class="card shadow-sm">
			<div class="card-body">
				<ul class="list-group">`

		// Loop through dataMap
		for _, key := range orderedKeys {
			value, exists := dataMap[key]
			if !exists {
				continue // skip if not returned by API
			}

			label, ok := fieldLabels[key]
			if !ok {
				label = key // fallback to raw key
			}

			htmlContent += `
		<li class="list-group-item d-flex justify-content-between align-items-center">
			<strong>` + label + `</strong>
			<span>` + fmt.Sprintf("%v", value) + `</span>
		</li>`
		}

		htmlContent += `
				</ul>
			</div>
		</div>
	</div>
</body>
</html>`

		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, htmlContent)
	}
}
