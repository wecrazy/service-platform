/**
 * @wecrazy
 * @fileoverview Technical Assistance related JavaScript functions
 * @link http://146.196.106.202:3417/
 * @link http://manageserviceai.csna4u.com:22425/
 * @requires jQuery
 * @requires SweetAlert2
 * @copyright 2025 PT. Cyber Smart Network Asia (CSNA)
 * @version 2.0.0
 * @license MIT
 */

/**
 * Generic password visibility toggle function
 * @param {string} displayId - ID of the element displaying the password
 * @param {string} iconId - ID of the eye icon element
 * @param {string} realPassword - The actual password text
 * @param {string} maskedPassword - The masked password (asterisks)
 */
window.togglePasswordVisibility = function (displayId, iconId, realPassword, maskedPassword) {
    const display = document.getElementById(displayId);
    const icon = document.getElementById(iconId);

    if (display && icon) {
        if (display.textContent.includes('*')) {
            display.textContent = realPassword;
            icon.className = 'bx bx-hide';
        } else {
            display.textContent = maskedPassword;
            icon.className = 'bx bx-show';
        }
    }
};

/**
 * Loads and displays an image
 * @param {string} imageURL - The URL of the image to load
 * @param {string} containerId - The ID of the container element to prepend the image to
 * @param {string} altText - The alternative text for the image
 * @param {string} defJPG - The default image URL to use if the main image fails to load
 */
function getImage(imageURL, containerId, altText, defJPG) {
    let imgWidth = "200px";
    let imgHeight = "auto";

    $.ajax({
        url: imageURL,
        method: 'HEAD'
    }).done(function () {
        let img = $('<img>', {
            src: imageURL,
            alt: altText,
            class: 'card-img-top',
            css: { width: imgWidth, height: imgHeight },
            click: function () {
                window.open($(this).attr('src'), '_blank');
            }
        });
        $('#' + containerId).prepend(img);
    }).fail(function () {
        let img = $('<img>', {
            src: defJPG,
            alt: altText,
            class: 'card-img-top',
            css: { width: imgWidth, height: imgHeight }
        });
        $('#' + containerId).prepend(img);
    });
}

/**
 * Handles hash change to determine current tab
 * @returns {Promise<string>} Returns "error" if hash includes "data-error", "pending" if hash includes "data-pending"
 */
async function handleHashChange() {
    const hash = window.location.hash;

    if (hash.includes("data-error")) {
        return "error";
    } else if (hash.includes("data-pending")) {
        return "pending";
    }
}

/**
 * Previews an image file before upload
 * @param {Event} event - The change event from the file input
 * @param {string} id - The ID of the preview element
 */
async function previewImage(event, id) {
    let input = event.target;
    let preview = $(`#preview-${id}`);

    if (input.files && input.files[0]) {
        let reader = new FileReader();

        reader.onload = function (e) {
            preview.attr('src', e.target.result);
            preview.removeClass("d-none");
        };

        reader.readAsDataURL(input.files[0]);
    } else {
        preview.attr('src', "");
        preview.addClass("d-none");
    }
}

/**
 * Shows an image in a modal with zoom controls
 * @param {string} imgSrc - The source URL of the image to display
 * @param {string} title - The title to display in the modal
 */
function showImageModal(imgSrc, title) {
    Swal.fire({
        title: `
				<div class="d-flex justify-content-between align-items-center">
					<span class="fw-semibold">${title}</span>
					<div>
						<button onclick="zoomImage(1.25)" type="button" class="btn btn-sm btn-primary me-1">+</button>
						<button onclick="zoomImage(0.8)" type="button" class="btn btn-sm btn-secondary">-</button>
					</div>
				</div>
			`,
        html: `
				<div class="text-center overflow-auto">
					<img id="zoomable-img" src="${imgSrc}" alt="${title}"
						class="img-fluid"
						style="transition: transform 0.3s ease; max-height: 80vh;" />
				</div>
			`,
        showConfirmButton: false,
        showCloseButton: true,
        allowOutsideClick: false,
        allowEscapeKey: false,
        width: "auto",
        customClass: {
            popup: "border-0 shadow rounded",
        },
    });
    window.currentZoom = 1;
}

/**
 * Zooms the image in the modal
 * @param {number} factor - The zoom factor to apply (e.g., 1.25 to zoom in, 0.8 to zoom out)
 */
function zoomImage(factor) {
    const img = $("#zoomable-img");
    window.currentZoom = (window.currentZoom || 1) * factor;
    window.currentZoom = Math.min(Math.max(window.currentZoom, 0.5), 5);
    img.css('transform', "scale(" + window.currentZoom + ")");
}

/**
 * Opens popup windows for photos and additional data
 * @param {string|number} id - The ID of the task/record
 * @param {string} table - The table name
 */
function openPopupPhotos(id, table) {
    // Photos
    window.open(
        "/photos/" + id + "?table=" + table,
        "popupWindowRight",
        "width=400,height=700,top=20,left=" +
        (window.screen.width - 420) +
        ",scrollbars=yes,resizable=yes"
    );

    // Additional data
    window.open(
        "/ta_additional_data/" + id,
        "popupWindowLeft",
        "width=400,height=700,top=20,left=20,scrollbars=yes,resizable=yes"
    );
}

/**
 * Reload page after a specified timer
 * @param {number} timer - Time in milliseconds
 */
function reloadThePage(timer) {
    setTimeout(() => {
        window.location.reload();
    }, timer);
}

/**
 * Shows a warning dialog for delete operations
 * @param {string} id_task - The task ID to be deleted
 * @param {function} onConfirm - Callback function to execute if user confirms
 */
function showDeleteWarning(id_task, onConfirm) {
    Swal.fire({
        title: "⚠️ Peringatan Penting",
        html: `
            <div style="text-align: left;">
                <p><strong>Dengan melakukan penghapusan pada ID: <code style="color: #dc3545; font-weight: bold;">${id_task}</code></strong></p>
                <div style="background: #fff3cd; border: 1px solid #ffc107; border-radius: 5px; padding: 10px; margin: 10px 0;">
                    <i class="bx bx-error-circle" style="color: #856404;"></i>
                    <strong style="color: #856404;">Data yang dihapus tidak dapat dikembalikan.</strong>
                </div>
                <p style="margin-top: 15px;">Apakah Anda yakin ingin melanjutkan penghapusan data ini?</p>
            </div>
        `,
        icon: "warning",
        showCancelButton: true,
        confirmButtonText: "Ya, Hapus Data",
        cancelButtonText: "Batal",
        allowOutsideClick: false,
        allowEscapeKey: false,
        customClass: {
            confirmButton: "btn btn-danger",
            cancelButton: "btn btn-secondary",
        },
        buttonsStyling: false,
    }).then((result) => {
        if (result.isConfirmed) {
            onConfirm();
        }
    });
}

/**
 * Handles Odoo login failure errors with specific UI
 * @param {string} errorText - The error response text
 * @param {object} formValues - Object containing name and password used in the request
 * @returns {boolean} True if error was handled (Odoo login failure), false otherwise
 */
async function handleOdooLoginError(errorText, formValues) {
    if (errorText && errorText.toLowerCase().includes('failed login to odoo')) {
        const realPassword = formValues.password;
        const maskedPassword = '*'.repeat(formValues.password.length);

        await Swal.fire({
            icon: "error",
            title: "Error",
            html: `
                <div class="text-start">
                    <p><strong>Autentikasi login ke Odoo gagal!</strong></p>
                    <div class="alert alert-danger" role="alert">
                        <div class="mb-2">
                            <label class="small text-muted">Email:</label><br>
                            <span class="badge bg-light text-dark px-2 py-1">${formValues.name}</span>
                        </div>
                        <div class="mb-2">
                            <label class="small text-muted">Password:</label><br>
                            <div class="d-flex align-items-center">
                                <span id="password-display-error" class="badge bg-light text-dark px-2 py-1 me-2" style="font-family: monospace; min-width: 100px;">${maskedPassword}</span>
                                <button type="button" class="btn btn-sm btn-outline-secondary" onclick="window.togglePasswordVisibility('password-display-error', 'eye-icon-error', '${realPassword.replace(/'/g, "\\'")}', '${maskedPassword}')">
                                    <i id="eye-icon-error" class="bx bx-show"></i>
                                </button>
                            </div>
                        </div>
                    </div>
                    <div class="alert alert-warning" role="alert">
                        <i class="bx bx-info-circle me-1"></i>
                        <strong>Saran:</strong> Periksa kembali email dan password Anda, pastikan tidak ada typo atau caps lock aktif.
                    </div>
                    <p class="text-muted small">
                        <i class="bx bx-bulb me-1"></i>
                        Jika masih bermasalah, coba refresh halaman atau hubungi technical support sistem di
                        <a href="https://wa.me/6287883507445" target="_blank" class="text-primary fw-bold">
                            <i class="bx bxl-whatsapp me-1"></i>WhatsApp
                        </a>.
                    </p>
                </div>
            `,
            confirmButtonText: "Coba Lagi",
            customClass: {
                confirmButton: "btn btn-danger",
            },
            buttonsStyling: false,
            width: '500px'
        });
        return true;
    }
    return false;
}

/* ================================
   🕒 PENDING
   ================================ */

/**
 *
 * @param {HTMLElement} button - The button element that triggered the action for pending data (Reason Code != A00)
 */
function sendCekPending(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card-cek");
    let id_task = card.find(".id_task").val();

    // console.log("ID Task: "+id_task);

    // Buat objek JSON
    let postData = {
        id_task: id_task,
    };

    // Tampilkan SweetAlert dengan loading
    Swal.fire({
        title: "Processing...",
        text: "Data sedang di check...",
        allowOutsideClick: false,
        allowEscapeKey: false,
        didOpen: () => {
            Swal.showLoading();
        },
    });

    $.ajax({
        url: "/here/checkData",
        method: "POST",
        contentType: "application/json",
        data: JSON.stringify(postData),
    }).done(function (result) {
        // console.log("RESULT: " + JSON.stringify(result, null, 2));

        // Jika request berhasil
        Swal.fire({
            title: "Berhasil!",
            text: "Data berhasil dicek.",
            icon: "success",
            timer: 5000, // Auto close dalam 5 detik
            showConfirmButton: false,
        });

        reloadThePage(2000);
    }).fail(function () {
        // Jika request gagal
        Swal.fire({
            title: "Gagal!",
            text: "Terjadi kesalahan saat check data.",
            icon: "error",
            confirmButtonText: "Coba Lagi",
        });
    });
}

function sendDataKonfirmasiPending(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card");
    let email = card.find(".email").val();
    let password = card.find(".password").val();
    let id_task = card.find(".id_task").val();
    let keepData = false;
    if (card.find(".keep-data").length) {
        keepData = card.find(".keep-data").is(":checked");
    }
    let isPaid = false;
    if (card.find(".is-paid").length) {
        isPaid = card.find(".is-paid").is(":checked");
    }

    // Check if ID exists first
    checkExistingIDTaskInTable(id_task, "pending", "error").then(exists => {
        if (!exists) return;

        // Proceed if exists
        // Buat objek JSON
        let postData = {
            email: email,
            password: password,
            id_task: id_task,
            keep_data: keepData,
            is_paid: isPaid,
        };

        // Tampilkan SweetAlert dengan loading
        Swal.fire({
            title: "Processing...",
            text: "Data sedang di proses...",
            allowOutsideClick: false,
            allowEscapeKey: false,
            didOpen: () => {
                Swal.showLoading();
            },
        });

        $.ajax({
            url: "/here/postData",
            method: "POST",
            contentType: "application/json",
            data: JSON.stringify(postData),
        }).done(function (result) {
            // Jika request berhasil
            let successMessage = "Data berhasil dikirim.";

            // Try to extract message from response
            if (typeof result === 'object' && result.message) {
                successMessage = result.message;
            } else if (typeof result === 'string') {
                try {
                    const parsed = JSON.parse(result);
                    successMessage = parsed.message || successMessage;
                } catch (e) {
                    // If parsing fails, use result as message if it's reasonable length
                    if (result.length < 200) {
                        successMessage = result;
                    }
                }
            }

            Swal.fire({
                title: "Berhasil!",
                text: successMessage,
                icon: "success",
                timer: 5000, // Auto close dalam 5 detik
                showConfirmButton: false,
            });

            // Reload halaman setelah 5 detik
            reloadThePage(5000);
        }).fail(function (xhr) {
            // Jika request gagal
            let errorMessage = "Terjadi kesalahan saat mengirim data.";

            try {
                // Try to parse JSON error response
                const errorResponse = JSON.parse(xhr.responseText);
                errorMessage = errorResponse.message || errorMessage;
            } catch (e) {
                // If not JSON, use responseText directly if available
                if (xhr.responseText && xhr.responseText.length < 200) {
                    errorMessage = xhr.responseText;
                } else if (xhr.statusText) {
                    errorMessage += ` (${xhr.status}: ${xhr.statusText})`;
                }
            }

            // Check for Odoo login error
            handleOdooLoginError(errorMessage, { name: email, password: password }).then((handled) => {
                if (!handled) {
                    Swal.fire({
                        title: "Gagal!",
                        text: errorMessage,
                        icon: "error",
                        confirmButtonText: "Coba Lagi",
                        customClass: {
                            confirmButton: "btn btn-danger",
                        },
                        buttonsStyling: false,
                    });
                }
            });
        });
    }).catch(error => {
        console.error("Error checking ID:", error);
        Swal.fire({
            title: "Error",
            text: error.message || "Terjadi kesalahan saat memeriksa ID Task.",
            icon: "error",
            confirmButtonText: "OK"
        });
    });
}

function sendDataHapusPending(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card");
    let id_task = card.find(".id_task").val();

    showDeleteWarning(id_task, () => performPendingDeletion(button));
}

function performPendingDeletion(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card");
    let email = card.find(".email").val();
    let password = card.find(".password").val();
    let id_task = card.find(".id_task").val();
    let ta_remark = card.find(".ta_remark").val();

    // Buat objek JSON
    let postData = {
        email: email,
        password: password,
        id_task: id_task,
        reason: ta_remark,
    };

    // Tampilkan SweetAlert dengan loading
    Swal.fire({
        title: "Processing...",
        text: "Data sedang di proses...",
        allowOutsideClick: false,
        allowEscapeKey: false,
        didOpen: () => {
            Swal.showLoading();
        },
    });

    $.ajax({
        url: "/here/deleteData",
        method: "POST",
        contentType: "application/json",
        data: JSON.stringify(postData),
    }).done(function (result) {
        // Jika request berhasil
        Swal.fire({
            title: "Berhasil!",
            text: "Data berhasil dihapus.",
            icon: "success",
            timer: 5000, // Auto close dalam 5 detik
            showConfirmButton: false,
        });

        // Reload halaman setelah 5 detik
        reloadThePage(5000);
    }).fail(function (xhr, status, error) {
        // Jika request gagal
        let errorMessage = "Terjadi kesalahan saat menghapus data.";

        try {
            // Try to parse JSON error response
            const errorResponse = JSON.parse(xhr.responseText);
            errorMessage = errorResponse.message || errorMessage;
        } catch (e) {
            // If not JSON, use responseText directly if available
            if (xhr.responseText && xhr.responseText.length < 200) {
                errorMessage = xhr.responseText;
            } else if (xhr.statusText) {
                errorMessage += ` (${xhr.status}: ${xhr.statusText})`;
            }
        }

        // Check for Odoo login error
        handleOdooLoginError(errorMessage, { name: email, password: password }).then((handled) => {
            if (!handled) {
                Swal.fire({
                    title: "Gagal!",
                    text: errorMessage,
                    icon: "error",
                    confirmButtonText: "Coba Lagi",
                    customClass: {
                        confirmButton: "btn btn-danger",
                    },
                    buttonsStyling: false,
                });
            }
        });
    });
}

/* ================================
   📷 ERROR
   ================================ */

/**
 *
 * @param {HTMLElement} button - The button element that triggered the action for photo error data
 */
function sendCekError(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card-cek");
    let id_task = card.find(".id_task").val();

    console.log("ID Task: " + id_task);

    // Buat objek JSON
    let postData = {
        id_task: id_task,
    };

    // Tampilkan SweetAlert dengan loading
    Swal.fire({
        title: "Processing...",
        text: "Data sedang di check...",
        allowOutsideClick: false,
        allowEscapeKey: false,
        didOpen: () => {
            Swal.showLoading();
        },
    });

    $.ajax({
        url: "/here/checkData",
        method: "POST",
        contentType: "application/json",
        data: JSON.stringify(postData),
    }).done(function (result) {
        // Jika request berhasil
        Swal.fire({
            title: "Berhasil!",
            text: "Data berhasil dicek.",
            icon: "success",
            timer: 5000, // Auto close dalam 5 detik
            showConfirmButton: false,
        });

        reloadThePage(2000);
    }).fail(function () {
        // Jika request gagal
        Swal.fire({
            title: "Gagal!",
            text: "Terjadi kesalahan saat check data.",
            icon: "error",
            confirmButtonText: "Coba Lagi",
        });
    });
}

function sendDataKonfirmasiError(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card");
    let email = card.find(".email").val();
    let password = card.find(".password").val();
    let id_task = card.find(".id_task").val();
    let keepData = false;
    if (card.find(".keep-data").length) {
        keepData = card.find(".keep-data").is(":checked");
    }
    let isPaid = false;
    if (card.find(".is-paid").length) {
        isPaid = card.find(".is-paid").is(":checked");
    }

    // Check if ID exists first
    checkExistingIDTaskInTable(id_task, "error", "pending").then(exists => {
        if (!exists) return;

        // Proceed if exists
        // Buat objek JSON
        let postData = {
            email: email,
            password: password,
            id_task: id_task,
            keep_data: keepData,
            is_paid: isPaid,
        };

        // Tampilkan SweetAlert dengan loading
        Swal.fire({
            title: "Processing...",
            text: "Data sedang di proses...",
            allowOutsideClick: false,
            allowEscapeKey: false,
            didOpen: () => {
                Swal.showLoading();
            },
        });

        $.ajax({
            url: "/here/postData",
            method: "POST",
            contentType: "application/json",
            data: JSON.stringify(postData),
        }).done(function (result) {
            // Jika request berhasil
            let successMessage = "Data berhasil dikirim.";

            // Try to extract message from response
            if (typeof result === 'object' && result.message) {
                successMessage = result.message;
            } else if (typeof result === 'string') {
                try {
                    const parsed = JSON.parse(result);
                    successMessage = parsed.message || successMessage;
                } catch (e) {
                    // If parsing fails, use result as message if it's reasonable length
                    if (result.length < 200) {
                        successMessage = result;
                    }
                }
            }

            Swal.fire({
                title: "Berhasil!",
                text: successMessage,
                icon: "success",
                timer: 5000, // Auto close dalam 5 detik
                showConfirmButton: false,
            });

            // Reload halaman setelah 5 detik
            reloadThePage(5000);
        }).fail(function (xhr) {
            // Jika request gagal
            let errorMessage = "Terjadi kesalahan saat mengirim data.";

            try {
                // Try to parse JSON error response
                const errorResponse = JSON.parse(xhr.responseText);
                errorMessage = errorResponse.message || errorMessage;
            } catch (e) {
                // If not JSON, use responseText directly if available
                if (xhr.responseText && xhr.responseText.length < 200) {
                    errorMessage = xhr.responseText;
                } else if (xhr.statusText) {
                    errorMessage += ` (${xhr.status}: ${xhr.statusText})`;
                }
            }

            // Check for Odoo login error
            handleOdooLoginError(errorMessage, { name: email, password: password }).then((handled) => {
                if (!handled) {
                    Swal.fire({
                        title: "Gagal!",
                        text: errorMessage,
                        icon: "error",
                        confirmButtonText: "Coba Lagi",
                        customClass: {
                            confirmButton: "btn btn-danger",
                        },
                        buttonsStyling: false,
                    });
                }
            });
        });
    }).catch(error => {
        console.error("Error checking ID:", error);
        Swal.fire({
            title: "Error",
            text: error.message || "Terjadi kesalahan saat memeriksa ID Task.",
            icon: "error",
            confirmButtonText: "OK"
        });
    });
}

function sendDataHapusError(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card");
    let id_task = card.find(".id_task").val();

    showDeleteWarning(id_task, () => performErrorDeletion(button));
}

function performErrorDeletion(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card");
    let email = card.find(".email").val();
    let password = card.find(".password").val();
    let id_task = card.find(".id_task").val();
    let ta_remark = card.find(".ta_remark").val();

    // Buat objek JSON
    let postData = {
        email: email,
        password: password,
        id_task: id_task,
        reason: ta_remark,
    };

    // Tampilkan SweetAlert dengan loading
    Swal.fire({
        title: "Processing...",
        text: "Data sedang di proses...",
        allowOutsideClick: false,
        allowEscapeKey: false,
        didOpen: () => {
            Swal.showLoading();
        },
    });

    $.ajax({
        url: "/here/deleteData",
        method: "POST",
        contentType: "application/json",
        data: JSON.stringify(postData),
    }).done(function (result) {
        // Jika request berhasil
        Swal.fire({
            title: "Berhasil!",
            text: "Data berhasil dihapus.",
            icon: "success",
            timer: 5000, // Auto close dalam 5 detik
            showConfirmButton: false,
        });

        // Reload halaman setelah 5 detik
        reloadThePage(5000);
    }).fail(function (xhr, status, error) {
        // Jika request gagal
        let errorMessage = "Terjadi kesalahan saat menghapus data.";

        try {
            // Try to parse JSON error response
            const errorResponse = JSON.parse(xhr.responseText);
            errorMessage = errorResponse.message || errorMessage;
        } catch (e) {
            // If not JSON, use responseText directly if available
            if (xhr.responseText && xhr.responseText.length < 200) {
                errorMessage = xhr.responseText;
            } else if (xhr.statusText) {
                errorMessage += ` (${xhr.status}: ${xhr.statusText})`;
            }
        }

        // Check for Odoo login error
        handleOdooLoginError(errorMessage, { name: email, password: password }).then((handled) => {
            if (!handled) {
                Swal.fire({
                    title: "Gagal!",
                    text: errorMessage,
                    icon: "error",
                    confirmButtonText: "Coba Lagi",
                    customClass: {
                        confirmButton: "btn btn-danger",
                    },
                    buttonsStyling: false,
                });
            }
        });
    });
}

/* ================================
   ✏️ E D I T - Data
   ================================ */

/**
 * Universal constant for photo fields
 * Used in modal forms and change tracking
 */
const PHOTO_FIELDS = [
    { id: "x_foto_bast", label: "Foto BAST" },
    { id: "x_foto_ceklis", label: "Foto Media Promo" },
    { id: "x_foto_edc", label: "Foto SN EDC" },
    { id: "x_foto_pic", label: "Foto PIC Merchant" },
    { id: "x_foto_setting", label: "Foto Pengaturan" },
    { id: "x_foto_thermal", label: "Foto Thermal" },
    { id: "x_foto_toko", label: "Foto Merchant" },
    { id: "x_foto_training", label: "Foto Surat Training" },
    { id: "x_foto_transaksi", label: "Foto Transaksi" },
    { id: "x_tanda_tangan_pic", label: "Tanda Tangan PIC" },
    { id: "x_tanda_tangan_teknisi", label: "Tanda Tangan Teknisi" },
    { id: "x_foto_sticker_edc", label: "Foto Stiker EDC" },
    { id: "x_foto_screen_guard", label: "Foto Screen Gard" },
    { id: "x_foto_all_transaction", label: "Foto Sales Draft All Memberbank" },
    { id: "x_foto_transaksi_bmri", label: "Foto Sales Draft BMRI" },
    { id: "x_foto_transaksi_bni", label: "Foto Sales Draft BNI" },
    { id: "x_foto_transaksi_bri", label: "Foto Sales Draft BRI" },
    { id: "x_foto_transaksi_btn", label: "Foto Sales Draft BTN" },
    { id: "x_foto_transaksi_patch", label: "Foto Sales Draft Patch L" },
    { id: "x_foto_screen_p2g", label: "Foto Screen P2G" },
    { id: "x_foto_kontak_stiker_pic", label: "Foto Kontak Stiker PIC" },
    { id: "x_foto_selfie_video_call", label: "Foto Selfie Video Call" },
    { id: "x_foto_selfie_teknisi_merchant", label: "Foto Selfie Teknisi dan Merchant" },
];

/**
 * Sends edit data for a task
 * @param {HTMLElement} button - The button element that triggered the edit action to open the modal confirmation
 */
async function sendEditData(button) {
    let card = $(button).closest(".card-edit-data");
    let id_task = card.find(".id_task").val();
    let woNumber = card.find(".wo_number").val();
    let company = card.find(".company").val();
    let reasonCode = card.find(".reason_code").val();
    let woRemark = card.find(".wo_remark").val();
    let taFeedback = card.find(".editable-feedback").val();

    Swal.fire({
        icon: "warning",
        // title: `<span class="text-danger">Confirm Edit</span>`,
        html: `Apakah Anda yakin ingin mengubah data dengan WO Number: <b>${woNumber}</b>?`,
        showCancelButton: true,
        confirmButtonText: "Ya, Edit",
        cancelButtonText: "Batal",
        allowOutsideClick: false,
        allowEscapeKey: false,
        customClass: {
            confirmButton: "btn btn-primary",
            cancelButton: "btn btn-danger",
        },
    }).then((result) => {
        if (result.isConfirmed) {
            modalFormEditDataWO(
                id_task,
                woNumber,
                company,
                reasonCode,
                woRemark,
                taFeedback
            );
        }
    });
}

/**
 * Opens a modal form for editing WO data
 * @param {string} id_task - The task ID
 * @param {string} woNumber - The work order number like MTI-123456, etc.
 * @param {string} company - The company name like ARTAJASA, MTI, MANDIRI, etc.
 * @param {string} reasonCode - The reason code like A00, B02, M10, etc.
 * @param {string} woRemark - The work order remark
 * @param {string} taFeedback - The TA feedback
 */
async function modalFormEditDataWO(
    id_task,
    woNumber,
    company,
    reasonCode,
    woRemark,
    taFeedback
) {
    let modal = $(".modal.show")[0];
    if (modal) {
        $(modal).attr("inert", ""); // Prevent interactions, but keep it visible
    }

    let userEmail = $(".userData").data("user-email");
    let currentTab = await handleHashChange();

    if (!currentTab) {
        currentTab = curTabTA;
    }

    // Run on hash change (when user switches tab)
    $(window).on("hashchange", function () {
        currentTab = handleHashChange();
    });

    console.log("Current Tab TA: " + curTabTA);

    let paidCheckbox = "";
    let isFinal = false;
    let usingKeepDataCheckbox = true;
    switch (currentTab) {
        case "error":
            usingKeepDataCheckbox = true;
            paidCheckbox = "";
            break;
        case "pending":
            usingKeepDataCheckbox = true;
            paidCheckbox = `
        <div class="col-lg-2 col-md-6 col-sm-6 d-flex align-items-center">
          <div class="form-check d-flex align-items-center mt-4 mx-3">
            <input class="form-check-input me-2" type="checkbox" id="edit-data-status">
            <label for="edit-data-status" class="form-check-label">Paid?</label>
          </div>
        </div>
      `;
            break;
        case "submission":
            isFinal = true;
            usingKeepDataCheckbox = false;
            break;
        // default: // unknown || undefined
        //     usingKeepDataCheckbox = false;
        //     break;
    }

    console.log("#==# Current Tab: " + currentTab);

    // Determine suggestion table
    let suggestionTable = "pending";
    if (currentTab === "pending") {
        suggestionTable = "error";
    } else if (currentTab === "error") {
        suggestionTable = "pending";
    } // for submission, default to pending

    // Check if ID exists first
    try {
        const exists = await checkExistingIDTaskInTable(id_task, currentTab, suggestionTable);
        if (!exists) {
            if (modal) {
                $(modal).removeAttr("inert");
            }
            return;
        }
    } catch (error) {
        console.error("Error checking ID:", error);
        if (modal) {
            $(modal).removeAttr("inert");
        }
        Swal.fire({
            title: "Error",
            text: error.message || "Terjadi kesalahan saat memeriksa ID Task.",
            icon: "error",
            confirmButtonText: "OK"
        });
        return;
    }

    const { value: formValues } = await Swal.fire({
        title: "",
        html: `
    <form id="taskEdit" class="container-fluid">
      <h1 class="text-warning mb-5 display-3">
        EDIT DATA
      </h1>

      <div class="row align-items-center gx-3 gy-2">
        <!-- Paid Checkbox -->
        
        ${paidCheckbox ||
            '<div class="col-lg-2 col-md-6 col-sm-6 d-flex align-items-center"></div>'
            }

        <!-- Task ID -->
        <div class="col-lg-2 col-md-6 col-sm-6">
          <label for="edit-data-task" class="form-label small mb-1"><b>${woNumber}</b></label>
          <input type="text" class="form-control bg-label-secondary" id="edit-data-task" value="${id_task}" readonly>
        </div>

        <!-- Email -->
        <div class="col-lg-4 col-md-6 col-sm-12">
          <label for="edit-data-name" class="form-label small mb-1">Email</label>
          <input type="text" class="form-control" id="edit-data-name" value="${userEmail}" placeholder="Masukkan email ODOO">
        </div>

        <!-- Password -->
        <div class="col-lg-4 col-md-6 col-sm-12">
          <label for="edit-data-password" class="form-label small mb-1">Password</label>
          <div class="input-group">
            <input type="password" class="form-control" id="edit-data-password" placeholder="Masukkan password Anda">
            <button type="button" class="btn btn-outline-secondary" onclick="togglePasswordInput('edit-data-password', 'eye-icon-password')">
              <i id="eye-icon-password" class="bx bx-show"></i>
            </button>
          </div>
        </div>
      </div>

      <div class="row gx-3 gy-3 mb-4">
        <!-- TA Feedback -->
        <div class="col-12 mt-3">
          <label for="" class="form-label">TA Feedback</label>
          <textarea class="form-control" id="edit-feedback">
            ${taFeedback.trim()}
          </textarea>
        </div>

        <!-- Reason -->
        <div class="col-12">
          <label for="edit-data-reason" class="form-label">Reason Code</label>
          <select class="form-select" id="edit-data-reason">
            <!-- Dynamic option -->
          </select>
        </div>

        <!-- Keterangan / WO Remark -->
        <div class="col-12">
          <label for="edit-data-keterangan" class="form-label">Keterangan / WO Remark</label>
          <textarea id="edit-data-keterangan" class="form-control" placeholder="Input keterangan JO / WO Remark" rows="3">${woRemark}</textarea>
        </div>

        <!-- Supply Thermal -->
        <div class="col-12">
          <label for="edit-data-supply-thermal" class="form-label">Supply Thermal</label>
          <input 
            type="number" 
            id="edit-data-supply-thermal" 
            class="form-control" 
            placeholder="Masukkan jumlah supply thermal" 
            min="0" 
            max="3000">
        </div>

        <!-- File Uploads with Preview -->
        <div class="col-12">
          <label class="form-label fw-bold">Upload Foto</label>
          <div class="row gx-2 gy-2">
            ${PHOTO_FIELDS.map(
                ({ id, label }) => `<div class="col-md-4">
                <label for="edit-data-${id}" class="form-label">${label}</label>
                <input type="file" class="form-control" id="edit-data-${id}" accept="image/*" onchange="previewImage(event, '${id}')">
                <img id="preview-${id}" src="" alt="Preview" class="img-thumbnail mt-2 d-none" style="max-width: 100px;">
              </div>`)
                .join("")}
          </div>
        </div>

        ${usingKeepDataCheckbox ? `
        <!-- Keep Data Checkbox -->
        <div class="form-check d-flex align-items-center mt-4">
            <input 
                class="form-check-input" 
                type="checkbox"
                id="edit-data-keep"
                data-bs-toggle="tooltip" 
                data-bs-placement="right" 
                title="Jika dicentang, data akan tetap muncul di Dashboard TA 😃"
            >
            <label for="edit-data-keep" class="form-check-label ms-2">Tetap Simpan Data?</label>
        </div>
        ` : ''}

      </div>
    </form>
  `,
        customClass: {
            confirmButton: "btn btn-primary",
            cancelButton: "btn btn-danger",
        },
        width: "90vw",
        showCancelButton: true,
        confirmButtonText: "Preview Changes",
        cancelButtonText: "Cancel",
        allowOutsideClick: false,
        allowEscapeKey: false,
        focusConfirm: false,
        didOpen: () => {
            setTimeout(() => {
                $("#edit-data-name").focus();
            }, 200);

            $(".swal2-container").css("z-index", "1100");

            $.ajax({
                url: "/here/listReason",
                type: "POST",
                contentType: "application/json",
                data: JSON.stringify({ company: company }), // Send JSON body
                success: function (data) {
                    let reasonSelect = $("#edit-data-reason");
                    reasonSelect.empty(); // Clear existing options

                    // Default option
                    reasonSelect.append(
                        '<option value="" disabled selected>Pilih reason code yang sesuai</option>'
                    );

                    // Convert reasonCode to lowercase for case-insensitive comparison
                    let reasonCodeLower = reasonCode.toLowerCase();

                    // Append options from the response
                    $.each(data, function (id, label) {
                        let labelLower = label.toLowerCase(); // Convert API response to lowercase

                        let option = $("<option>", {
                            value: id,
                            text: label,
                            selected: reasonCodeLower === labelLower, // Set selected if it matches
                        });

                        reasonSelect.append(option);
                    });
                },
                error: function (xhr, status, error) {
                    // Show an error alert using SweetAlert2
                    let errorMessage = "Gagal mengambil Reason Codes! Coba lagi.";

                    try {
                        // Try to parse JSON error response
                        const errorResponse = JSON.parse(xhr.responseText);
                        errorMessage = errorResponse.message || errorMessage;
                    } catch (e) {
                        // If not JSON, use responseText directly if available
                        if (xhr.responseText && xhr.responseText.length < 200) {
                            errorMessage = xhr.responseText;
                        } else if (xhr.statusText) {
                            errorMessage += ` (${xhr.status}: ${xhr.statusText})`;
                        }
                    }

                    Swal.fire({
                        icon: "error",
                        title: "Oops...",
                        html: `<h2 class="m-0 p-0">${errorMessage}</h2><small>Error: ${xhr.status} - ${error}</small>`,
                        footer: `<small>Silahkan hubungi Technical Support di +6287883507445</small>`,
                    });
                },
            });
        },
        preConfirm: () => {
            let name = $("#edit-data-name").val();
            let password = $("#edit-data-password").val();
            let task = $("#edit-data-task").val();
            let reason = $("#edit-data-reason").val();
            let keterangan = $("#edit-data-keterangan").val();
            let feedback = $("#edit-feedback").val();
            let supplyThermal = $("#edit-data-supply-thermal").val();
            feedback = feedback.trim();
            let isPaid = false;
            if ($("#edit-data-status").length) {
                isPaid = $("#edit-data-status").is(":checked");
            }
            let keepData = false;
            if (usingKeepDataCheckbox && $("#edit-data-keep").length) {
                keepData = $("#edit-data-keep").is(":checked");
            }

            console.log("Paid Status: " + isPaid);
            console.log("Keep Data Status: " + keepData);

            // Get all file inputs - dynamically generated from PHOTO_FIELDS
            let files = {};
            PHOTO_FIELDS.forEach((field) => {
                let fileInput = $("#edit-data-" + field.id)[0];
                if (fileInput && fileInput.files.length > 0) {
                    files[field.id] = fileInput.files[0];
                }
            });

            if (!name || !password || !task || !reason || !keterangan) {
                Swal.showValidationMessage(
                    "Alamat email, password, ID task, reason code dan keterangan / wo remark harus diisi!"
                );
                return false;
            }

            // Validate thermal value INSIDE preConfirm to keep modal open
            const thermalValue = Number(supplyThermal);
            const maxThermal = 3000;
            if (supplyThermal && thermalValue > maxThermal) {
                Swal.showValidationMessage(
                    `${thermalValue} Supply Thermal terlalu besar! Maksimum adalah ${maxThermal}.`
                );
                return false;
            }

            // Store data temporarily
            const selectedReasonText = $("#edit-data-reason option:selected").text();

            return {
                name,
                password,
                task,
                reason,
                keterangan,
                files,
                isPaid,
                supplyThermal,
                feedback,
                selectedReasonText,
                keepData,
            };
        },
    });

    if (modal) {
        $(modal).removeAttr("inert");
    }

    if (formValues) {
        // Track changes and show confirmation
        let changes = [];
        const selectedReasonText = formValues.selectedReasonText;

        if (reasonCode.toLowerCase() !== selectedReasonText.toLowerCase()) {
            changes.push(`Reason Code: ${reasonCode} → ${selectedReasonText}`);
        }
        if (formValues.keterangan !== woRemark) {
            changes.push(`WO Remark: ${woRemark} → ${formValues.keterangan}`);
        }
        if (formValues.feedback !== taFeedback.trim()) {
            changes.push(`TA Feedback: ${taFeedback.trim()} → ${formValues.feedback}`);
        }
        if (formValues.supplyThermal && formValues.supplyThermal > 0) {
            changes.push(`Supply Thermal: ${formValues.supplyThermal}`);
        }
        if (formValues.isPaid) {
            changes.push(`Paid Status: true`);
        }
        if (formValues.keepData) {
            changes.push(`Keep Data: true`);
        }
        Object.keys(formValues.files).forEach((fileKey) => {
            const photoField = PHOTO_FIELDS.find(field => fileKey === field.id);
            if (photoField) {
                changes.push(`Mengubah ${photoField.label}`);
            }
        });

        // No changes detected
        if (changes.length === 0) {
            Swal.fire({
                icon: "info",
                title: "No Changes Detected",
                text: "Anda tidak melakukan perubahan apapun ☺️.",
                confirmButtonText: "OK",
            });
            return;
        }

        // Show confirmation dialog with clear messaging
        const confirmResult = await Swal.fire({
            icon: "question",
            title: "📋 Preview Perubahan Data",
            html: `<div style="text-align: left; max-height: 500px; overflow-y: auto; padding: 15px; background: #f8f9fa; border-radius: 8px; margin-bottom: 15px;">
                <strong style="color: #696cff; font-size: 1.1em;">Data yang akan diubah:</strong><br><br>
                ${changes.map((change, index) => `<div style="margin-bottom: 10px; padding: 8px; background: white; border-radius: 4px;">
                    <span style="color: #696cff; font-weight: 700; margin-right: 8px;">${index + 1}.</span>${change}
                </div>`).join('')}
            </div>
            <div style="text-align: center; padding: 15px; background: #fff3cd; border-radius: 8px; border-left: 4px solid #ffc107;">
                <strong style="color: #856404;">⚠️ PERHATIAN</strong><br>
                <small style="color: #856404;">Jika Anda klik "Batal", Anda harus mengisi form edit dari awal lagi.<br>
                Pastikan data sudah benar sebelum melanjutkan!</small>
            </div>`,
            width: '85%',
            showCancelButton: true,
            confirmButtonText: '<i class="bx bx-check-circle"></i> Simpan Perubahan',
            cancelButtonText: '<i class="bx bx-x-circle"></i> Batal (Form akan tertutup)',
            allowOutsideClick: false,
            allowEscapeKey: false,
            customClass: {
                confirmButton: "btn btn-success btn-lg",
                cancelButton: "btn btn-danger btn-lg",
            },
            buttonsStyling: false,
        });

        // If user cancels, show clear message that form is closed
        if (!confirmResult.isConfirmed) {
            Swal.fire({
                icon: "warning",
                title: "❌ Dibatalkan",
                html: `<p style="font-size: 1.1em;">Update data dibatalkan.</p>
                       <p style="color: #6c757d;">Tidak ada perubahan yang disimpan.<br>
                       Silakan klik tombol <strong>Edit</strong> lagi jika ingin mengubah data.</p>`,
                confirmButtonText: "OK, Mengerti",
                customClass: {
                    confirmButton: "btn btn-primary",
                },
                buttonsStyling: false,
            });
            return;
        }

        /**
         * Update feedback after user confirms changes
         */
        const updatedFeedback = formValues.feedback;
        if (updatedFeedback !== taFeedback.trim()) {
            $.ajax({
                url: "/ta_feedback",
                method: "POST",
                contentType: "application/json",
                data: JSON.stringify({
                    tabel: currentTab,
                    id_task: id_task,
                    wo_number: woNumber,
                    feedback: updatedFeedback,
                }),
                success: function (res) {
                    console.log("Feedback updated:", res);
                },
                error: function (xhr) {
                    let msg = "Terjadi kesalahan saat update feedback: " + xhr.responseText;
                    try {
                        const res = JSON.parse(xhr.responseText);
                        msg = res.message || msg;
                    } catch (e) { }
                    console.error("Feedback update failed:", msg);
                },
            });
        }
        /**
         * .end of update feedback
         */

        // Build formData once
        let formData = new FormData();
        formData.append("name", formValues.name);
        formData.append("password", formValues.password);
        formData.append("task", formValues.task);
        formData.append("reason", formValues.reason);
        formData.append("keterangan", formValues.keterangan);
        formData.append("isPaid", formValues.isPaid);

        // Convert thermal value to number for validation
        const thermalValue = Number(formValues.supplyThermal);
        if (formValues.supplyThermal !== "" && thermalValue > 0) {
            formData.append("thermal", formValues.supplyThermal);
        }

        // Create logEdit string from changes array (separated by |) only if changes exist
        if (changes && changes.length > 0) {
            const logEditString = changes.join(" | ");
            formData.append("logEdit", logEditString);
        }

        formData.append("keepData", formValues.keepData ? "true" : "false");

        // Append is_final flag for submission tab
        if (isFinal) {
            formData.append("is_final", "true");
        }

        // Append all selected files
        Object.keys(formValues.files).forEach((key) => {
            formData.append(key, formValues.files[key]);
        });

        // Show processing alert
        Swal.fire({
            title: "Processing...",
            text: "Mohon bersabar, system sedang mengolah data yang telah Anda update",
            allowOutsideClick: false,
            allowEscapeKey: false,
            didOpen: () => {
                Swal.showLoading();
            },
        });

        try {
            const controller = new AbortController();
            const timeout = setTimeout(() => controller.abort(), 10 * 60 * 1000); // 10 minutes

            // console.log("Submitting form data...");
            // console.log("Form Data Entries:");
            // for (let pair of formData.entries()) {
            //     console.log(`${pair[0]}: ${pair[1]}`);
            // }
            // // .end of TODO

            let response = await fetch("/here/editData", {
                method: "POST",
                body: formData,
                signal: controller.signal,
            });

            clearTimeout(timeout);

            let resultText = await response.text();

            let result;
            try {
                result = JSON.parse(resultText);
            } catch (e) {
                // Check if the response text contains login failure
                const handled = await handleOdooLoginError(resultText, formValues);
                if (!handled) {
                    await Swal.fire({
                        title: "Response",
                        text: resultText,
                        icon: "info",
                        confirmButtonText: "OK",
                    });
                }
                return;
            }

            let responseMessage = result.message
                ? result.message
                : Object.values(result).join("\n");

            Swal.fire({
                title: "Success!",
                text: responseMessage,
                icon: "success",
                timer: 5000,
                showConfirmButton: false,
            });

            reloadThePage(4000);
        } catch (error) {
            console.error("Error:", error);
            let errorMessage =
                error.name === "AbortError"
                    ? "Request timed out! Please try again."
                    : error.message || "Failed to save data. Please try again.";

            Swal.fire({
                title: "Error!",
                text: errorMessage,
                icon: "error",
                confirmButtonText: "Try Again",
            });
        }
    }
}

/**
 * Shows edit logs in a modal with formatted information
 * @param {string} idTask - The task ID
 * @param {string} logs - The raw edit logs string with format: "Change1 | Change2 | Change3 ~email@domain.com"
 */
function showEditLogs(idTask, wo, logs) {
    if (!logs || logs.trim() === "") {
        Swal.fire({
            title: "No Edit History",
            text: "No edit logs available for this task.",
            icon: "info",
            confirmButtonText: "OK"
        });
        return;
    }

    // Extract editor email from the end (after ~)
    let editorEmail = "Unknown";
    let logContent = logs;

    const emailMatch = logs.match(/~([^~]+@[^~]+)$/);
    if (emailMatch) {
        editorEmail = emailMatch[1].trim();
        // Remove the email part from logs to get clean content
        logContent = logs.replace(/\s*~[^~]+@[^~]+$/, '').trim();
    }

    // Split by | to get individual changes
    const editEntries = logContent.split('|').map(entry => entry.trim()).filter(entry => entry !== '');

    // Format the logs for display with different styling for different types of changes
    let formattedLogs = '';
    if (editEntries.length > 0) {
        formattedLogs = editEntries.map((entry, index) => {
            let badgeClass = 'bg-primary';
            let iconClass = 'bx-edit';

            // Assign different colors and icons based on change type
            if (entry.includes('Reason Code:')) {
                badgeClass = 'bg-warning';
                iconClass = 'bx-code-alt';
            } else if (entry.includes('WO Remark:')) {
                badgeClass = 'bg-info';
                iconClass = 'bx-note';
            } else if (entry.includes('TA Feedback:')) {
                badgeClass = 'bg-success';
                iconClass = 'bx-message-dots';
            } else if (entry.includes('Supply Thermal:')) {
                badgeClass = 'bg-secondary';
                iconClass = 'bx-receipt';
            } else if (entry.includes('Paid Status:')) {
                badgeClass = 'bg-success';
                iconClass = 'bx-money';
            } else if (entry.includes('Keep Data:')) {
                badgeClass = 'bg-info';
                iconClass = 'bx-save';
            } else if (entry.includes('Mengubah Foto') || entry.includes('Foto')) {
                badgeClass = 'bg-danger';
                iconClass = 'bx-image';
            }

            return `<div class="mb-2 p-3 bg-light rounded border-start border-3 border-${badgeClass.replace('bg-', '')}">
                <div class="d-flex align-items-start">
                    <span class="badge ${badgeClass} me-2 d-flex align-items-center">
                        <i class="bx ${iconClass} me-1"></i>
                        ${index + 1}
                    </span>
                    <div class="flex-grow-1">
                        <span class="fw-medium">${entry}</span>
                    </div>
                </div>
            </div>`;
        }).join('');
    }

    // If no formatted logs, show raw logs as fallback
    if (!formattedLogs) {
        formattedLogs = `<div class="p-3 bg-light rounded border">
            <pre class="mb-0" style="white-space: pre-wrap; font-family: inherit;">${logContent}</pre>
        </div>`;
    }

    // Format timestamp or show generic edit time
    const currentTime = new Date().toLocaleString('id-ID', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
    });

    Swal.fire({
        title: `<div class="d-flex align-items-center justify-content-center">
            <i class="bx bx-history text-info me-2 fs-1"></i>
            <span class="text-info">Edit History</span>
        </div>`,
        html: `
            <div class="text-start">
                <div class="row mb-4">
                    <div class="col-md-6">
                        <div class="d-flex align-items-center mb-2">
                            <strong class="text-primary me-2">Task ID:</strong> 
                            <code class="bg-label-danger px-2 py-1 rounded">${idTask}</code>
                        </div>
                        <div class="d-flex align-items-center mb-2">
                            <strong class="text-primary me-2">WO Number:</strong> 
                            <code class="bg-label-warning px-2 py-1 rounded">${wo}</code>
                        </div>
                    </div>
                    <div class="col-md-6">
                        <div class="d-flex align-items-center mb-2">
                            <strong class="text-primary me-2">Edited by:</strong> 
                            <span class="badge bg-success fs-6">
                                <i class="bx bx-user me-1"></i>${editorEmail}
                            </span>
                        </div>
                    </div>
                </div>
                
                <div class="mb-3">
                    <strong class="text-primary d-flex align-items-center">
                        <i class="bx bx-list-ul me-2"></i>Changes Made:
                        <span class="badge bg-light text-dark ms-2">${editEntries.length} change${editEntries.length !== 1 ? 's' : ''}</span>
                    </strong>
                </div>
                
                <div style="max-height: 450px; overflow-y: auto; padding-right: 8px;">
                    ${formattedLogs}
                </div>
                
                <div class="mt-3 pt-3 border-top">
                    <small class="text-muted d-flex align-items-center justify-content-center">
                        <i class="bx bx-time me-1"></i>
                        Last viewed: ${currentTime}
                    </small>
                </div>
            </div>
        `,
        width: '80%',
        showConfirmButton: true,
        confirmButtonText: '<i class="bx bx-check-circle me-1"></i> CLOSE',
        customClass: {
            confirmButton: "btn btn-danger",
            popup: "border-0 shadow-lg"
        },
        buttonsStyling: false,
    });
}

/**
 * Edits feedback for a task
 * @param {HTMLElement} el - The element that triggered the edit
 * @param {string} tabel - The table name
 * @param {string} idTask - The task ID
 * @param {string} woNumber - The work order number
 * @param {string} endpoint - The API endpoint for updating feedback
 */
function editFeedback(el, tabel, idTask, woNumber, endpoint) {
    // Close any open Bootstrap modals
    $(".modal.show").each(function () {
        const modalEl = this;
        const modalInstance = bootstrap.Modal.getInstance(modalEl);
        if (modalInstance) {
            modalInstance.hide();
        } else {
            $(modalEl).removeClass("show").css("display", "none");
            $("body").removeClass("modal-open");
            $(".modal-backdrop").remove();
        }
    });

    Swal.close();

    const currentVal = $(el).data("value") || "";

    setTimeout(() => {
        Swal.fire({
            title: `<span class="badge bg-label-secondary" style="font-size: 0.75rem; vertical-align: middle;">${idTask}</span> Feedback untuk WO Number: ${woNumber}`,
            input: "textarea",
            inputLabel: "Masukkan feedback",
            inputValue: currentVal,
            showCancelButton: true,
            confirmButtonText: "Simpan",
            cancelButtonText: "Batal",
            allowOutsideClick: false,
            allowEscapeKey: false,
            customClass: {
                popup: "rounded",
                confirmButton: "btn btn-primary",
                cancelButton: "btn btn-secondary ms-2",
            },
            buttonsStyling: false,
            preConfirm: (newVal) => {
                newVal = newVal.trim();

                if (!newVal) {
                    Swal.showValidationMessage("Feedback tidak boleh kosong.");
                    return false;
                }

                Swal.showLoading();

                return new Promise((resolve, reject) => {
                    $.ajax({
                        url: endpoint,
                        method: "POST",
                        contentType: "application/json",
                        data: JSON.stringify({
                            tabel: tabel,
                            id_task: idTask,
                            wo_number: woNumber,
                            feedback: newVal,
                        }),
                        success: function (res) {
                            // Update textarea value and data attribute
                            $(el).val(newVal).data("value", newVal);

                            // Optional: dynamically resize height
                            $(el).css("height", "auto").css("height", el.scrollHeight + "px");

                            resolve(res);
                        },
                        error: function (xhr) {
                            let msg = "Terjadi kesalahan: " + xhr.responseText;
                            try {
                                const res = JSON.parse(xhr.responseText);
                                msg = res.message || msg;
                            } catch (e) { }

                            Swal.hideLoading();
                            Swal.showValidationMessage(msg);
                            reject();
                        },
                    });
                });
            },
        }).then((result) => {
            if (result.isConfirmed) {
                const message = result.value?.message || "Feedback berhasil disimpan";

                // Reload the DataTable that contains el
                const $table = $("table.dt_teknisi_pengerjaan_" + tabel);
                if ($table.length) {
                    const dtInstance = $table.DataTable();
                    if (dtInstance) {
                        dtInstance.ajax.reload(null, false);
                    }
                }

                Swal.fire({
                    icon: "success",
                    title: message,
                    showConfirmButton: false,
                    timer: 1500,
                });
            }
        });
    }, 200);
}

/**
 * Generic password visibility toggle function for input fields
 * @param {string} inputId - ID of the password input element
 * @param {string} iconId - ID of the eye icon element
 */
function togglePasswordInput(inputId, iconId) {
    const input = $('#' + inputId);
    const icon = $('#' + iconId);
    if (input.attr('type') === 'password') {
        input.attr('type', 'text');
        icon.removeClass('bx-show').addClass('bx-hide');
    } else {
        input.attr('type', 'password');
        icon.removeClass('bx-hide').addClass('bx-show');
    }
}

/**
 * Toggles password visibility for DataTable inputs using the button element
 * @param {HTMLElement} button - The button element that was clicked
 */
function togglePasswordInputFromButton(button) {
    const input = button.previousElementSibling;
    const icon = button.querySelector('i');
    if (input.type === 'password') {
        input.type = 'text';
        icon.classList.remove('bx-show');
        icon.classList.add('bx-hide');
    } else {
        input.type = 'password';
        icon.classList.remove('bx-hide');
        icon.classList.add('bx-show');
    }
}

/**
 * Check if a specific ID task exists in a given table
 * @param {string} idTask 
 * @param {string} tableToFind 
 * @param {string} suggestionTableToGo 
 */
function checkExistingIDTaskInTable(idTask, tableToFind, suggestionTableToGo) {
    return new Promise((resolve, reject) => {
        $.ajax({
            url: "/check_existing_status_id_in_table_ta",
            method: "POST",
            contentType: "application/json",
            data: JSON.stringify({
                id_task: idTask,
                table: tableToFind,
            }),
            success: function (res) {
                if (!res.exists) {
                    Swal.fire({
                        title: "Peringatan",
                        text: `Maaf, ID Task ${idTask} tidak ditemukan di tabel ${tableToFind}. Data mungkin telah dihapus. Silakan refresh halaman dan periksa di ${suggestionTableToGo}.`,
                        icon: "warning",
                        confirmButtonText: "OK"
                    });
                }
                resolve(res.exists);
            },
            error: function (xhr) {
                let msg = "Terjadi kesalahan saat memeriksa ID Task: " + xhr.responseText;
                try {
                    const res = JSON.parse(xhr.responseText);
                    msg = res.message || msg;
                } catch (e) { }
                console.error(msg);
                reject(new Error(msg));
            },
        });
    });
}

/* ================================
   ⏳ Awaiting Final Submission
   ================================ */

/**
 * @param {HTMLElement} button - The button element that triggered the action for photo awaiting submission data
 */
function sendCekSubmission(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card-cek");
    let id_task = card.find(".id_task").val();

    console.log("ID Task: " + id_task);

    // Show warning confirmation before proceeding
    Swal.fire({
        title: "⚠️ Peringatan Penting",
        html: `
            <div style="text-align: left;">
                <p><strong>Dengan melakukan pengecekan pada ID: <code style="color: #dc3545; font-weight: bold;">${id_task}</code></strong></p>
                <div style="background: #fff3cd; border: 1px solid #ffc107; border-radius: 5px; padding: 10px; margin: 10px 0;">
                    <i class="bx bx-error-circle" style="color: #856404;"></i>
                    <strong style="color: #856404;">Kemungkinan data yang ada di tabel akan hilang atau berubah.</strong>
                </div>
                <p style="margin-top: 15px;">Apakah Anda yakin ingin melanjutkan pengecekan data ini?</p>
            </div>
        `,
        icon: "warning",
        showCancelButton: true,
        confirmButtonText: "Ya, Lanjutkan Pengecekan",
        cancelButtonText: "Batal",
        allowOutsideClick: false,
        allowEscapeKey: false,
        customClass: {
            confirmButton: "btn btn-warning",
            cancelButton: "btn btn-secondary",
        },
        buttonsStyling: false,
    }).then((result) => {
        if (result.isConfirmed) {
            // Proceed with the check operation
            performSubmissionCheck(id_task);
        }
    });
}

function performSubmissionCheck(id_task) {
    // Buat objek JSON
    let postData = {
        id_task: id_task,
    };

    // Tampilkan SweetAlert dengan loading
    Swal.fire({
        title: "Processing...",
        text: "Data sedang di check...",
        allowOutsideClick: false,
        allowEscapeKey: false,
        didOpen: () => {
            Swal.showLoading();
        },
    });

    $.ajax({
        url: "/here/checkData",
        method: "POST",
        contentType: "application/json",
        data: JSON.stringify(postData),
    }).done(function (result) {
        // Jika request berhasil
        Swal.fire({
            title: "Berhasil!",
            text: "Data berhasil dicek.",
            icon: "success",
            timer: 5000, // Auto close dalam 5 detik
            showConfirmButton: false,
        });

        reloadThePage(2000);
    }).fail(function () {
        // Jika request gagal
        Swal.fire({
            title: "Gagal!",
            text: "Terjadi kesalahan saat check data.",
            icon: "error",
            confirmButtonText: "Coba Lagi",
        });
    });
}

function sendDataHapusSubmission(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card");
    let id_task = card.find(".id_task").val();

    showDeleteWarning(id_task, () => performSubmissionDeletion(button));
}

function performSubmissionDeletion(button) {
    // Cari elemen input yang terdekat di dalam card yang sama
    let card = $(button).closest(".card");
    let email = card.find(".email").val();
    let password = card.find(".password").val();
    let id_task = card.find(".id_task").val();
    let ta_remark = card.find(".ta_remark").val();

    // Buat objek JSON
    let postData = {
        email: email,
        password: password,
        id_task: id_task,
        reason: ta_remark,
    };

    // Tampilkan SweetAlert dengan loading
    Swal.fire({
        title: "Processing...",
        text: "Data sedang di proses...",
        allowOutsideClick: false,
        allowEscapeKey: false,
        didOpen: () => {
            Swal.showLoading();
        },
    });

    $.ajax({
        url: "/here/deleteData",
        method: "POST",
        contentType: "application/json",
        data: JSON.stringify(postData),
    }).done(function (result) {
        // Jika request berhasil
        Swal.fire({
            title: "Berhasil!",
            text: "Data berhasil dihapus.",
            icon: "success",
            timer: 5000, // Auto close dalam 5 detik
            showConfirmButton: false,
        });

        // Reload halaman setelah 5 detik
        reloadThePage(5000);
    }).fail(function (xhr, status, error) {
        // Jika request gagal
        let errorMessage = "Terjadi kesalahan saat menghapus data.";

        try {
            // Try to parse JSON error response
            const errorResponse = JSON.parse(xhr.responseText);
            errorMessage = errorResponse.message || errorMessage;
        } catch (e) {
            // If not JSON, use responseText directly if available
            if (xhr.responseText && xhr.responseText.length < 200) {
                errorMessage = xhr.responseText;
            } else if (xhr.statusText) {
                errorMessage += ` (${xhr.status}: ${xhr.statusText})`;
            }
        }

        // Check for Odoo login error
        handleOdooLoginError(errorMessage, { name: email, password: password }).then((handled) => {
            if (!handled) {
                Swal.fire({
                    title: "Gagal!",
                    text: errorMessage,
                    icon: "error",
                    confirmButtonText: "Coba Lagi",
                    customClass: {
                        confirmButton: "btn btn-danger",
                    },
                    buttonsStyling: false,
                });
            }
        });
    });
}