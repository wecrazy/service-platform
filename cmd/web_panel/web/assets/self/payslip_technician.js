/**
 * Fetch the last update timestamp for the payslip of a technician.
 * @param {*} endpoint - The API endpoint to fetch the last update.
 * @param {*} lastUpdateClass - The CSS class to update with the last update timestamp.
 */
function GetLastUpdatePayslipTechnicianEDC(endpoint, lastUpdateClass) {
    if (!endpoint || !lastUpdateClass) {
        swal.fire({
            icon: "error",
            title: "Error!",
            text: "Invalid parameters for fetching last update.",
            timer: 2500,
        });
        return;
    }

    $.ajax({
        url: endpoint,
        type: "GET",
        dataType: "json",
        success: function (response) {
            if (response) {
                let lastUpdateStr = response.last_update_edc;
                let htmlLastUpdate = "";
                
                if (lastUpdateStr === "N/A") {
                    htmlLastUpdate = `
                        <div class="text-center">
                            <small class="text-muted">
                                <i class="fas fa-info-circle me-1"></i>
                                No updates available
                            </small>
                        </div>
                    `;
                } else {
                    let userPart = response.last_update_edc_by || "Unknown";
                    let timePart = lastUpdateStr;
                    let monthYear = response.last_update_edc_month_year || "";
                    
                    htmlLastUpdate = `
                        <div class="d-flex flex-column align-items-start">
                            <h1 class="mb-2 text-primary fw-bold">
                                Slip Gaji <i class="fas fa-file-invoice-dollar me-2 ms-2"></i> Teknisi Manage Service EDC ${monthYear}
                            </h1>
                            <div class="d-flex flex-column gap-1">
                                <small class="text-muted d-flex align-items-center">
                                    <i class="fas fa-clock me-2 text-info"></i>
                                    <span>${timePart}</span>
                                    <span class="mx-2 text-muted">|</span>
                                    <i class="fas fa-user me-2 text-success"></i>
                                    <span>by: ${userPart}</span>
                                </small>
                            </div>
                        </div>
                    `;
                }
                
                // Disable/enable send button based on all_sent status
                if (response.all_sent_edc !== undefined) {
                    const $btn = $(".btn-sent-all-payslip-technician-edc");
                    if (response.all_sent_edc) {
                        $btn.prop("disabled", true).addClass("btn-secondary").removeClass("btn-success");
                        $btn.html('<i class="far fa-check-circle me-2"></i>All Sent');
                    } else {
                        $btn.prop("disabled", false).addClass("btn-success").removeClass("btn-secondary");
                        $btn.html('<i class="far fa-paper-plane me-2"></i>Sent All Payslip');
                    }
                }
                
                $("." + lastUpdateClass).html(htmlLastUpdate);
            } else {
                $("." + lastUpdateClass).text("No data available");
            }
        },
        error: function (xhr, status, error) {
            let errorMsg = "Error fetching data";
            if (xhr.responseJSON) {
                if (xhr.responseJSON.error) {
                    errorMsg = xhr.responseJSON.error;
                }
            }
            Swal.fire({
                icon: "error",
                title: "Error!",
                text: errorMsg,
                timer: 2500,
            });
        },
    });
}

function GetLastUpdatePayslipTechnicianATM(endpoint, lastUpdateClass) {
    if (!endpoint || !lastUpdateClass) {
        swal.fire({
            icon: "error",
            title: "Error!",
            text: "Invalid parameters for fetching last update.",
            timer: 2500,
        });
        return;
    }

    $.ajax({
        url: endpoint,
        type: "GET",
        dataType: "json",
        success: function (response) {
            if (response) {
                let lastUpdateStr = response.last_update_atm;
                let htmlLastUpdate = "";
                
                if (lastUpdateStr === "N/A") {
                    htmlLastUpdate = `
                        <div class="text-center">
                            <small class="text-muted">
                                <i class="fas fa-info-circle me-1"></i>
                                No updates available
                            </small>
                        </div>
                    `;
                } else {
                    let userPart = response.last_update_atm_by || "Unknown";
                    let timePart = lastUpdateStr;
                    let monthYear = response.last_update_atm_month_year || "";
                    
                    htmlLastUpdate = `
                        <div class="d-flex flex-column align-items-start">
                            <h1 class="mb-2 text-primary fw-bold">
                                Slip Gaji <i class="fas fa-file-invoice-dollar me-2 ms-2"></i> Teknisi Dedicated ATM ${monthYear}
                            </h1>
                            <div class="d-flex flex-column gap-1">
                                <small class="text-muted d-flex align-items-center">
                                    <i class="fas fa-clock me-2 text-info"></i>
                                    <span>${timePart}</span>
                                    <span class="mx-2 text-muted">|</span>
                                    <i class="fas fa-user me-2 text-success"></i>
                                    <span>by: ${userPart}</span>
                                </small>
                            </div>
                        </div>
                    `;
                }
                
                // Disable/enable send button based on all_sent status
                if (response.all_sent_atm !== undefined) {
                    const $btn = $(".btn-sent-all-payslip-technician-atm");
                    if (response.all_sent_atm) {
                        $btn.prop("disabled", true).addClass("btn-secondary").removeClass("btn-success");
                        $btn.html('<i class="far fa-check-circle me-2"></i>All Sent');
                    } else {
                        $btn.prop("disabled", false).addClass("btn-danger").removeClass("btn-secondary");
                        $btn.html('<i class="far fa-paper-plane me-2"></i>Sent All Payslip');
                    }
                }
                
                $("." + lastUpdateClass).html(htmlLastUpdate);
            } else {
                $("." + lastUpdateClass).text("No data available");
            }
        },
        error: function (xhr, status, error) {
            let errorMsg = "Error fetching data";
            if (xhr.responseJSON) {
                if (xhr.responseJSON.error) {
                    errorMsg = xhr.responseJSON.error;
                }
            }
            Swal.fire({
                icon: "error",
                title: "Error!",
                text: errorMsg,
                timer: 2500,
            });
        },
    });
}

/**
 * Send all payslip data to the technicians via email.
 * @param {*} endpoint - The API endpoint to send the payslip data.
 * @param {*} lastUpdateClass - The CSS class to update with the last update timestamp.
 * @param {*} tbClass - The CSS class for the data table.
 * @param {*} btnSentClass - The CSS class for the send button.
 */

/**
 * Send all payslip data to the technicians via email or WhatsApp.
 * @param {*} endpoint - The API endpoint to send payslips.
 * @param {*} lastUpdateClass - The CSS class to update with the last update timestamp.
 * @param {*} tbClass - The CSS class for the data table.
 * @param {*} btnSentClass - The CSS class for the send button.
 */
async function sentAllPayslipTechnicianEDC(endpoint, lastUpdateClass, tbClass, btnSentClass) {
    const $btn = $("." + btnSentClass);

    // Show warning with send method selection
    Swal.fire({
        title: "Send All Payslips?",
        html: `
            <div class="text-start">
                <p class="mb-3">This will send <strong>ALL</strong> payslips to EDC technicians.</p>
                
                <div class="alert alert-warning" role="alert">
                    <i class="fas fa-exclamation-triangle me-2"></i>
                    <strong>Warning:</strong> This action will send payslips to multiple recipients at once.
                </div>

                <div class="mb-3">
                    <label class="form-label fw-bold">Send Method:</label>
                    <div class="form-check">
                        <input class="form-check-input" type="radio" name="sendAllMethod" id="sendAllMethodEmail" value="email" checked>
                        <label class="form-check-label" for="sendAllMethodEmail">
                            <i class="fas fa-envelope me-2 text-primary"></i>Email
                        </label>
                    </div>
                    <div class="form-check">
                        <input class="form-check-input" type="radio" name="sendAllMethod" id="sendAllMethodWhatsApp" value="whatsapp">
                        <label class="form-check-label" for="sendAllMethodWhatsApp">
                            <i class="fab fa-whatsapp me-2 text-success"></i>WhatsApp
                        </label>
                    </div>
                </div>

                <p class="mb-0 text-muted"><small>All payslips will be sent to the technicians' registered contacts.</small></p>
            </div>
        `,
        icon: "warning",
        showCancelButton: true,
        confirmButtonText: "Yes, Send All!",
        cancelButtonText: "Cancel",
        confirmButtonColor: "#28a745",
        cancelButtonColor: "#6c757d",
        reverseButtons: true,
        customClass: {
            popup: 'swal-wide',
            confirmButton: 'btn btn-success',
            cancelButton: 'btn btn-secondary'
        },
        preConfirm: () => {
            const selectedMethod = document.querySelector('input[name="sendAllMethod"]:checked');
            if (!selectedMethod) {
                Swal.showValidationMessage('Please select a send method');
                return false;
            }
            return selectedMethod.value;
        }
    }).then((result) => {
        if (result.isConfirmed) {
            const sendMethod = result.value;
            const methodName = sendMethod === "email" ? "Email" : "WhatsApp";
            const methodIcon = sendMethod === "email" ? "fa-envelope" : "fa-whatsapp";

            // Store original button content
            const originalBtnContent = $btn.html();
            
            // Show spinner and disable button
            $btn
                .prop("disabled", true)
                .removeClass("btn-success")
                .addClass("btn-label-success")
                .html(`<span class="spinner-border spinner-border-sm me-2" role="status"></span>Sending via ${methodName}...`);

            // TODO: Implement the actual sending logic
            $.ajax({
                url: endpoint,
                type: "POST",
                contentType: "application/json",
                data: JSON.stringify({
                    project_ms: "edc",
                    send_option: sendMethod
                }),
                dataType: "json",
                success: function (response) {
                    // Build logs HTML
                    let logsHtml = '';
                    
                    if (response.success_logs && response.success_logs.length > 0) {
                        logsHtml += '<div class="mt-3"><strong class="text-success">✓ Successfully Sent:</strong><ul class="text-start small mt-2">';
                        response.success_logs.forEach(log => {
                            logsHtml += `<li class="text-success">${log}</li>`;
                        });
                        logsHtml += '</ul></div>';
                    }
                    
                    if (response.failed_logs && response.failed_logs.length > 0) {
                        logsHtml += '<div class="mt-3"><strong class="text-danger">✗ Failed to Send:</strong><ul class="text-start small mt-2">';
                        response.failed_logs.forEach(log => {
                            logsHtml += `<li class="text-danger">${log}</li>`;
                        });
                        logsHtml += '</ul></div>';
                    }

                    Swal.fire({
                        icon: response.failed_logs && response.failed_logs.length > 0 ? "warning" : "success",
                        title: response.failed_logs && response.failed_logs.length > 0 ? "Completed with Errors" : "Success!",
                        html: `
                            <div class="text-start">
                                <p><i class="fas ${methodIcon} me-2"></i>${response.message || `All payslips sent via ${methodName}!`}</p>
                                ${response.total_sent !== undefined ? `<p class="mb-2"><strong>Total sent: ${response.total_sent}</strong></p>` : ''}
                                ${logsHtml}
                            </div>
                        `,
                        width: '600px',
                        timer: logsHtml ? 10000 : 5000,
                    });

                    const table = $("." + tbClass).DataTable();
                    if (table) {
                        table.ajax.reload(null, false);
                    }
                },
                error: function (xhr, status, error) {
                    let errorMsg = `Failed to send all payslips via ${methodName}.`;
                    if (xhr.responseJSON) {
                        if (xhr.responseJSON.message) {
                            errorMsg = xhr.responseJSON.message;
                        } else if (xhr.responseJSON.error) {
                            errorMsg = xhr.responseJSON.error;
                        }
                    }

                    Swal.fire({
                        icon: "error",
                        title: "Error!",
                        html: `
                            <div class="text-start">
                                <p>${errorMsg}</p>
                                <small class="text-muted">Please try again or contact support.</small>
                            </div>
                        `,
                        timer: 5000,
                    });
                },
                complete: function () {
                    // Restore original button content and state
                    $btn
                        .prop("disabled", false)
                        .removeClass("btn-label-success")
                        .addClass("btn-success")
                        .html(originalBtnContent);
                },
            });
        }
    });
}

/**
 * Send all payslip data to the ATM technicians via email or WhatsApp.
 * @param {*} endpoint - The API endpoint to send payslips.
 * @param {*} lastUpdateClass - The CSS class to update with the last update timestamp.
 * @param {*} tbClass - The CSS class for the data table.
 * @param {*} btnSentClass - The CSS class for the send button.
 */
async function sentAllPayslipTechnicianATM(endpoint, lastUpdateClass, tbClass, btnSentClass) {
    const $btn = $("." + btnSentClass);

    // Show warning with send method selection
    Swal.fire({
        title: "Send All Payslips?",
        html: `
            <div class="text-start">
                <p class="mb-3">This will send <strong>ALL</strong> payslips to ATM technicians.</p>
                
                <div class="alert alert-warning" role="alert">
                    <i class="fas fa-exclamation-triangle me-2"></i>
                    <strong>Warning:</strong> This action will send payslips to multiple recipients at once.
                </div>

                <div class="mb-3">
                    <label class="form-label fw-bold">Send Method:</label>
                    <div class="form-check">
                        <input class="form-check-input" type="radio" name="sendAllMethodATM" id="sendAllMethodEmailATM" value="email" checked>
                        <label class="form-check-label" for="sendAllMethodEmailATM">
                            <i class="fas fa-envelope me-2 text-primary"></i>Email
                        </label>
                    </div>
                    <div class="form-check">
                        <input class="form-check-input" type="radio" name="sendAllMethodATM" id="sendAllMethodWhatsAppATM" value="whatsapp">
                        <label class="form-check-label" for="sendAllMethodWhatsAppATM">
                            <i class="fab fa-whatsapp me-2 text-success"></i>WhatsApp
                        </label>
                    </div>
                </div>

                <p class="mb-0 text-muted"><small>All payslips will be sent to the technicians' registered contacts.</small></p>
            </div>
        `,
        icon: "warning",
        showCancelButton: true,
        confirmButtonText: "Yes, Send All!",
        cancelButtonText: "Cancel",
        confirmButtonColor: "#dc3545",
        cancelButtonColor: "#6c757d",
        reverseButtons: true,
        customClass: {
            popup: 'swal-wide',
            confirmButton: 'btn btn-danger',
            cancelButton: 'btn btn-secondary'
        },
        preConfirm: () => {
            const selectedMethod = document.querySelector('input[name="sendAllMethodATM"]:checked');
            if (!selectedMethod) {
                Swal.showValidationMessage('Please select a send method');
                return false;
            }
            return selectedMethod.value;
        }
    }).then((result) => {
        if (result.isConfirmed) {
            const sendMethod = result.value;
            const methodName = sendMethod === "email" ? "Email" : "WhatsApp";
            const methodIcon = sendMethod === "email" ? "fa-envelope" : "fa-phone-square-alt";

            // Store original button content
            const originalBtnContent = $btn.html();
            
            // Show spinner and disable button
            $btn
                .prop("disabled", true)
                .removeClass("btn-danger")
                .addClass("btn-label-danger")
                .html(`<span class="spinner-border spinner-border-sm me-2" role="status"></span>Sending via ${methodName}...`);

            // TODO: Implement the actual sending logic
            $.ajax({
                url: endpoint,
                type: "POST",
                contentType: "application/json",
                data: JSON.stringify({
                    project_ms: "atm",
                    send_option: sendMethod
                }),
                dataType: "json",
                success: function (response) {
                    // Build logs HTML
                    let logsHtml = '';
                    
                    if (response.success_logs && response.success_logs.length > 0) {
                        logsHtml += '<div class="mt-3"><strong class="text-success">✓ Successfully Sent:</strong><ul class="text-start small mt-2">';
                        response.success_logs.forEach(log => {
                            logsHtml += `<li class="text-success">${log}</li>`;
                        });
                        logsHtml += '</ul></div>';
                    }
                    
                    if (response.failed_logs && response.failed_logs.length > 0) {
                        logsHtml += '<div class="mt-3"><strong class="text-danger">✗ Failed to Send:</strong><ul class="text-start small mt-2">';
                        response.failed_logs.forEach(log => {
                            logsHtml += `<li class="text-danger">${log}</li>`;
                        });
                        logsHtml += '</ul></div>';
                    }

                    Swal.fire({
                        icon: response.failed_logs && response.failed_logs.length > 0 ? "warning" : "success",
                        title: response.failed_logs && response.failed_logs.length > 0 ? "Completed with Errors" : "Success!",
                        html: `
                            <div class="text-start">
                                <p><i class="fas ${methodIcon} me-2"></i>${response.message || `All payslips sent via ${methodName}!`}</p>
                                ${response.total_sent !== undefined ? `<p class="mb-2"><strong>Total sent: ${response.total_sent}</strong></p>` : ''}
                                ${logsHtml}
                            </div>
                        `,
                        width: '600px',
                        timer: logsHtml ? 10000 : 5000,
                    });

                    const table = $("." + tbClass).DataTable();
                    if (table) {
                        table.ajax.reload(null, false);
                    }
                },
                error: function (xhr, status, error) {
                    let errorMsg = `Failed to send all payslips via ${methodName}.`;
                    if (xhr.responseJSON) {
                        if (xhr.responseJSON.message) {
                            errorMsg = xhr.responseJSON.message;
                        } else if (xhr.responseJSON.error) {
                            errorMsg = xhr.responseJSON.error;
                        }
                    }

                    Swal.fire({
                        icon: "error",
                        title: "Error!",
                        html: `
                            <div class="text-start">
                                <p>${errorMsg}</p>
                                <small class="text-muted">Please try again or contact support.</small>
                            </div>
                        `,
                        timer: 5000,
                    });
                },
                complete: function () {
                    // Restore original button content and state
                    $btn
                        .prop("disabled", false)
                        .removeClass("btn-label-danger")
                        .addClass("btn-danger")
                        .html(originalBtnContent);
                },
            });
        }
    });
}

function sendIndividualPayslipTechnician(id, projMS) {
    if (!id || !projMS) {
        Swal.fire({
            icon: "error",
            title: "Error!",
            text: "Invalid parameters for sending payslip.",
            timer: 2500,
        });
        return;
    }

    const endpoint = window.EndpointSendIndividualPayslip || "";
    const projectName = projMS.toUpperCase();

    // Show warning with send method selection
    Swal.fire({
        title: "Send Payslip?",
        html: `
            <div class="text-start">
                <p class="mb-3">Send payslip for technician ID: <strong>${id}</strong> (${projectName})</p>
                
                <div class="alert alert-info" role="alert">
                    <i class="fas fa-info-circle me-2"></i>
                    <strong>Note:</strong> Please select how you want to send this payslip.
                </div>

                <div class="mb-3">
                    <label class="form-label fw-bold">Send Method:</label>
                    <div class="form-check">
                        <input class="form-check-input" type="radio" name="sendMethod" id="sendMethodEmail" value="email" checked>
                        <label class="form-check-label" for="sendMethodEmail">
                            <i class="fas fa-envelope me-2 text-primary"></i>Email
                        </label>
                    </div>
                    <div class="form-check">
                        <input class="form-check-input" type="radio" name="sendMethod" id="sendMethodWhatsApp" value="whatsapp">
                        <label class="form-check-label" for="sendMethodWhatsApp">
                            <i class="fab fa-whatsapp me-2 text-success"></i>WhatsApp
                        </label>
                    </div>
                </div>

                <p class="mb-0 text-muted"><small>The payslip will be sent to the technician's registered contact.</small></p>
            </div>
        `,
        icon: "question",
        showCancelButton: true,
        confirmButtonText: "Send",
        cancelButtonText: "Cancel",
        confirmButtonColor: "#28a745",
        cancelButtonColor: "#6c757d",
        reverseButtons: true,
        customClass: {
            popup: 'swal-wide',
            confirmButton: 'btn btn-success',
            cancelButton: 'btn btn-secondary'
        },
        preConfirm: () => {
            const selectedMethod = document.querySelector('input[name="sendMethod"]:checked');
            if (!selectedMethod) {
                Swal.showValidationMessage('Please select a send method');
                return false;
            }
            return selectedMethod.value;
        }
    }).then((result) => {
        if (result.isConfirmed) {
            const sendMethod = result.value;
            const methodName = sendMethod === "email" ? "Email" : "WhatsApp";
            const methodIcon = sendMethod === "email" ? "fa-envelope" : "fa-whatsapp";

            if (!endpoint) {
                Swal.fire({
                    icon: "error",
                    title: "Error!",
                    text: "Endpoint not configured.",
                    timer: 2500,
                });
                return;
            }

            // Show loading state
            Swal.fire({
                title: `Sending via ${methodName}...`,
                allowOutsideClick: false,
                allowEscapeKey: false,
                showConfirmButton: false,
                didOpen: () => {
                    Swal.showLoading();
                }
            });

            // Make AJAX request to send payslip
            $.ajax({
                url: endpoint,
                type: "POST",
                contentType: "application/json",
                data: JSON.stringify({
                    id: id,
                    project_ms: projMS.toLowerCase(),
                    send_option: sendMethod
                }),
                dataType: "json",
                success: function (response) {
                    Swal.fire({
                        icon: "success",
                        title: "Success!",
                        html: `
                            <div class="text-start">
                                <p><i class="fas ${methodIcon} me-2"></i>${response.message || `Payslip sent successfully via ${methodName}!`}</p>
                                ${response.recipient ? `<small class="text-muted">Sent to: ${response.recipient}</small>` : ''}
                            </div>
                        `,
                        timer: 3500,
                    }).then(() => {
                        // Reload the table to show updated data
                        const tableClass = projMS.toLowerCase() === "edc" 
                            ? "dt_slip_gaji_teknisi_edc" 
                            : "dt_slip_gaji_teknisi_atm";
                        const table = $("." + tableClass).DataTable();
                        if (table) {
                            table.ajax.reload(null, false);
                        }
                    });
                },
                error: function (xhr, status, error) {
                    let errorMsg = `Failed to send payslip via ${methodName}.`;
                    if (xhr.responseJSON) {
                        if (xhr.responseJSON.error) {
                            errorMsg = xhr.responseJSON.error;
                        } else if (xhr.responseJSON.message) {
                            errorMsg = xhr.responseJSON.message;
                        }
                    }

                    Swal.fire({
                        icon: "error",
                        title: "Error!",
                        html: `
                            <div class="text-start">
                                <p>${errorMsg}</p>
                                <small class="text-muted">Please try again or contact support.</small>
                            </div>
                        `,
                        timer: 5000,
                    });
                },
            });
        }
    });
}

/**
 * Regenerate PDF payslip for a technician with user confirmation.
 * @param {number} id - The ID of the payslip record.
 * @param {string} projMS - The project type ('edc' or 'atm').
 */
function regeneratePayslipTechnician(id, projMS) {
    if (!id || !projMS) {
        Swal.fire({
            icon: "error",
            title: "Error!",
            text: "Invalid parameters for regenerating payslip.",
            timer: 2500,
        });
        return;
    }

    // Determine the endpoint based on project type
    let endpoint = "";
    let projectName = "";
    
    if (projMS.toLowerCase() === "edc") {
        endpoint = window.EndpointRegeneratePayslipTechnicianEDC || "";
        projectName = "EDC";
    } else if (projMS.toLowerCase() === "atm") {
        endpoint = window.EndpointRegeneratePayslipTechnicianATM || "";
        projectName = "ATM";
    }

    if (!endpoint) {
        Swal.fire({
            icon: "error",
            title: "Error!",
            text: "Endpoint not configured for this project type.",
            timer: 2500,
        });
        return;
    }

    // Show warning confirmation dialog
    Swal.fire({
        title: "Regenerate Payslip?",
        html: `
            <div class="text-start">
                <p>This will regenerate the PDF payslip for technician ID: <strong>${id}</strong> (${projectName})</p>
                <div class="alert alert-warning" role="alert">
                    <i class="fas fa-exclamation-triangle me-2"></i>
                    <strong>Warning:</strong> If a payslip already exists, it will be overwritten with new data.
                </div>
                <p class="mb-0">Do you want to continue?</p>
            </div>
        `,
        icon: "warning",
        showCancelButton: true,
        confirmButtonText: "Yes, Regenerate",
        cancelButtonText: "Cancel",
        confirmButtonColor: "#f59e0b",
        cancelButtonColor: "#6c757d",
        reverseButtons: true,
        customClass: {
            popup: 'swal-wide',
            confirmButton: 'btn btn-warning',
            cancelButton: 'btn btn-secondary'
        }
    }).then((result) => {
        if (result.isConfirmed) {
            // Show loading state
            Swal.fire({
                title: 'Regenerating Payslip...',
                text: 'Please wait while the payslip is being regenerated.',
                allowOutsideClick: false,
                allowEscapeKey: false,
                showConfirmButton: false,
                didOpen: () => {
                    Swal.showLoading();
                }
            });

            // Make AJAX request to regenerate
            $.ajax({
                url: `${endpoint}/${id}`,
                type: "POST",
                dataType: "json",
                success: function (response) {
                    Swal.fire({
                        icon: "success",
                        title: "Success!",
                        html: `
                            <div class="text-start">
                                <p>${response.message || "Payslip regenerated successfully!"}</p>
                                ${response.filepath ? `<small class="text-muted">File: ${response.filepath}</small>` : ''}
                            </div>
                        `,
                        timer: 3500,
                    }).then(() => {
                        // Reload the table to show updated data
                        const tableClass = projMS.toLowerCase() === "edc" 
                            ? "dt_slip_gaji_teknisi_edc" 
                            : "dt_slip_gaji_teknisi_atm";
                        const table = $("." + tableClass).DataTable();
                        if (table) {
                            table.ajax.reload(null, false);
                        }
                    });
                },
                error: function (xhr, status, error) {
                    let errorMsg = "Failed to regenerate payslip.";
                    if (xhr.responseJSON) {
                        if (xhr.responseJSON.error) {
                            errorMsg = xhr.responseJSON.error;
                        } else if (xhr.responseJSON.message) {
                            errorMsg = xhr.responseJSON.message;
                        }
                    }

                    Swal.fire({
                        icon: "error",
                        title: "Error!",
                        html: `
                            <div class="text-start">
                                <p>${errorMsg}</p>
                                <small class="text-muted">Please try again or contact support.</small>
                            </div>
                        `,
                        timer: 5000,
                    });
                },
            });
        }
    });
}

/**
 * Show WhatsApp conversation for a payslip in a modal.
 * @param {number} id - The ID of the payslip record.
 * @param {string} projMS - The project type ('edc' or 'atm').
 */
function showPayslipWhatsAppConversation(id, projMS) {
    if (!id || !projMS) {
        Swal.fire({
            icon: "error",
            title: "Error!",
            text: "Invalid parameters for showing conversation.",
            timer: 2500,
        });
        return;
    }

    const endpoint = window.EndpointGetPayslipWhatsAppConversation || "";
    
    if (!endpoint) {
        Swal.fire({
            icon: "error",
            title: "Error!",
            text: "Endpoint not configured.",
            timer: 2500,
        });
        return;
    }

    // Show loading
    Swal.fire({
        title: "Loading Conversation...",
        text: "Please wait",
        allowOutsideClick: false,
        allowEscapeKey: false,
        showConfirmButton: false,
        didOpen: () => {
            Swal.showLoading();
        }
    });

    // Fetch conversation data
    $.ajax({
        url: endpoint,
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({
            id: id,
            project_ms: projMS
        }),
        dataType: "json",
        success: function (response) {
            Swal.close();
            
            if (response.success && response.conversation) {
                // Use the existing showWhatsAppMessageModal function from sp_whatsapp_msg_modal.js
                if (typeof showWhatsAppMessageModal === 'function') {
                    showWhatsAppMessageModal(response.conversation);
                } else {
                    // Fallback if the function is not available
                    let conversationHtml = '<div class="text-start">';
                    response.conversation.forEach(msg => {
                        conversationHtml += '<div class="mb-3">';
                        conversationHtml += '<p><strong>Message:</strong> ' + (msg.whatsapp_message_body || 'N/A') + '</p>';
                        conversationHtml += '<p><strong>Sent to:</strong> ' + (msg.destination_name || 'N/A') + '</p>';
                        if (msg.whatsapp_reply_text) {
                            conversationHtml += '<p><strong>Reply:</strong> ' + msg.whatsapp_reply_text + '</p>';
                        }
                        conversationHtml += '</div>';
                    });
                    conversationHtml += '</div>';
                    
                    Swal.fire({
                        title: "WhatsApp Conversation",
                        html: conversationHtml,
                        icon: "info",
                        confirmButtonText: "Close",
                        width: 700,
                    });
                }
            } else {
                Swal.fire({
                    icon: "warning",
                    title: "No Conversation",
                    text: "No WhatsApp conversation found for this payslip.",
                    timer: 2500,
                });
            }
        },
        error: function (xhr, status, error) {
            Swal.close();
            
            let errorMsg = "Error fetching conversation";
            if (xhr.responseJSON && xhr.responseJSON.error) {
                errorMsg = xhr.responseJSON.error;
            }
            
            Swal.fire({
                icon: "error",
                title: "Error!",
                text: errorMsg,
                timer: 2500,
            });
        },
    });
}
