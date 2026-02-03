/**
 * Fetch the last update timestamp for the contract of a technician.
 * @param {*} endpoint - The API endpoint to fetch the last update.
 * @param {*} lastUpdateClass - The CSS class to update with the last update timestamp.
 */
function GetLastUpdateContractTechnician(endpoint, lastUpdateClass) {
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
                let lastUpdateStr = response.last_updated;
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
                    let timePart = lastUpdateStr;
                    let monthYear = response.last_updated_month_year || "";

                    htmlLastUpdate = `
                        <div class="d-flex flex-column align-items-start">
                            <h2 class="mb-2 text-primary fw-bold">
                                List <i class="far fa-user-hard-hat me-1 ms-1"></i> Teknisi Manage Service - ${monthYear}
                            </h2>
                            <div class="d-flex flex-column gap-1">
                                <small class="text-muted d-flex align-items-center">
                                    Last Update: <i class="fal fa-clock ms-2 me-2 text-info"></i>
                                    <span>${timePart}</span>
                                </small>
                            </div>
                        </div>
                    `;
                }
                
                // Disable/enable send button based on all_sent status
                if (response.all_contract_sent !== undefined) {
                    const $btn = $(".btn-sent-all-contract-technician");
                    if (response.all_contract_sent) {
                        $btn.prop("disabled", true).addClass("btn-secondary").removeClass("btn-danger");
                        $btn.html('<i class="far fa-check-circle me-2"></i>All Sent');
                    } else {
                        $btn.prop("disabled", false).addClass("btn-danger").removeClass("btn-secondary");
                        $btn.html('<i class="far fa-paper-plane me-2"></i>Sent All Contract');
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
 * Refresh the contract technician data table.
 * @param {*} refreshEndpoint - The API endpoint to refresh the data table.
 * @param {*} lastUpdateClass - The CSS class to update with the last update timestamp.
 * @param {*} tbClass - The CSS class of the data table to refresh.
 * @param {*} btnRefreshClass - The CSS class of the refresh button.
 */
async function refreshDataContractTechnician(endpoint, lastUpdateClass, tbClass, btnRefreshClass) {
     const $btn = $("." + btnRefreshClass);

    Swal.fire({
        title: "Are you sure?",
        text: "This will refresh the data from the server.",
        icon: "warning",
        showCancelButton: true,
        confirmButtonText: "Yes, refresh it!",
        cancelButtonText: "No, cancel!",
        confirmButtonColor: "#3085d6",
        cancelButtonColor: "#d33",
        reverseButtons: true,
    }).then((result) => {
        if (result.isConfirmed) {
            // Store original button content
            const originalBtnContent = $btn.html();
            
            // Show spinner and disable button
            $btn
                .prop("disabled", true)
                .removeClass("btn-info")
                .addClass("btn-label-info")
                .html('<span class="spinner-border spinner-border-sm me-2" role="status"></span>Refreshing...');

            $.ajax({
                url: endpoint,
                type: "GET",
                dataType: "json",
                success: function (response) {
                    Swal.fire({
                        icon: "success",
                        title: "Success!",
                        text: response.message || "Data refreshed successfully!",
                        timer: 5000,
                    });

                    const table = $("." + tbClass).DataTable();
                    if (table) {
                        table.ajax.reload(null, false);
                    }

                    const lastUpdateEndpoint = endpoint.replace("refresh", "last_update");
                    GetLastUpdateContractTechnician(lastUpdateEndpoint, lastUpdateClass);
                },
                error: function (xhr, status, error) {
                    let errorMsg = "Something went wrong.";
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
                        text: errorMsg,
                        timer: 5000,
                    });
                },
                complete: function () {
                    // Restore original button content and state
                    $btn
                        .prop("disabled", false)
                        .removeClass("btn-label-info")
                        .addClass("btn-info")
                        .html(originalBtnContent);
                },
            });
        }
    });
}

/**
 * Regenerate the PDF contract for a technician.
 * @param {number} id - The ID of the technician contract to regenerate.
 */
function regenerateContractTechnician(id) {
    if (!id) {
        swal.fire({
            icon: "error",
            title: "Error!",
            text: "Invalid technician contract ID.",
            timer: 2500,
        });
        return;
    }

    let endpoint = "";
    endpoint = window.EndpointRegeneratePDFContractTechnician;

    if (!endpoint) {
        swal.fire({
            icon: "error",
            title: "Error!",
            text: "Regenerate endpoint not defined.",
            timer: 2500,
        });
        return;
    }

    // Show warning confirmation dialog
    Swal.fire({
        title: "Regenerate Contract?",
        html: `
            <div class="text-start">
                <p>This will regenerate the PDF contract for technician ID: <strong>${id}</strong></p>
                <div class="alert alert-warning" role="alert">
                    <i class="fas fa-exclamation-triangle me-2"></i>
                    <strong>Warning:</strong> If a contract already exists, it will be overwritten with new data.
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
                title: 'Regenerating Contract...',
                text: 'Please wait while the contract is being regenerated.',
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
                                <p>${response.message || "Contract regenerated successfully!"}</p>
                                ${response.filepath ? `<small class="text-muted">File: ${response.filepath}</small>` : ''}
                                <p class="mt-2">Silahkan close detail data dan buka kembali datanya untuk melihat perubahan data.</p>
                            </div>
                        `,
                        timer: 3500,
                    }).then(() => {
                        // Reload the table to show updated data
                        const tableClass = "dt_kontrak_teknisi";
                        const table = $("." + tableClass).DataTable();
                        if (table) {
                            table.ajax.reload(null, false);
                        }
                    });
                },
                error: function (xhr, status, error) {
                    let errorMsg = "Failed to regenerate contract.";
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
 * Send individual contract to technician via email / whatsapp
 * @param {number} id - The ID of the technician contract to send.
 */

function sendIndividualContractTechnician(id) {
    if (!id) {
        swal.fire({
            icon: "error",
            title: "Error!",
            text: "Invalid technician contract ID.",
            timer: 2500,
        });
        return;
    }

    const endpoint = window.EndpointSendIndividualContractTechnician || "";
    if (!endpoint) {
        swal.fire({
            icon: "error",
            title: "Error!",
            text: "Send endpoint not defined.",
            timer: 2500,
        });
        return;
    }

    // Show warning with send method selection
    Swal.fire({
        title: "Send Contract?",
        html: `
            <div class="text-start">
                <p class="mb-3">Send contract for technician ID: <strong>${id}</strong></p>
                
                <div class="alert alert-info" role="alert">
                    <i class="fas fa-info-circle me-2"></i>
                    <strong>Note:</strong> Please select how you want to send this contract.
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

                <p class="mb-0 text-muted"><small>The contract will be sent to the technician's registered contact.</small></p>
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
            const methodIcon = sendMethod === "email" ? "fal fa-envelope" : "fab fa-whatsapp";

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

            // Make AJAX request to send contract
            $.ajax({
                url: endpoint,
                type: "POST",
                contentType: "application/json",
                data: JSON.stringify({
                    id: id,
                    send_option: sendMethod
                }),
                dataType: "json",
                success: function (response) {
                    Swal.fire({
                        icon: "success",
                        title: "Success!",
                        html: `
                            <div class="text-start">
                                <p><i class="${methodIcon} me-2"></i>${response.message || `Contract sent successfully via ${methodName}!`}</p>
                                ${response.recipient ? `<small class="text-muted">Sent to: ${response.recipient}</small>` : ''}
                            </div>
                        `,
                        timer: 3500,
                    }).then(() => {
                        // Reload the table to show updated data
                        const tableClass = "dt_kontrak_teknisi";
                        const table = $("." + tableClass).DataTable();
                        if (table) {
                            table.ajax.reload(null, false);
                        }
                    });
                },
                error: function (xhr, status, error) {
                    let errorMsg = `Failed to send contract via ${methodName}.`;
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
 * Show WhatsApp conversation in modal
 * @param {number} id - The ID of the technician contract to view conversation.
 */
function showContractWhatsAppConversation(id) {
    if (!id) {
        swal.fire({
            icon: "error",
            title: "Error!",
            text: "Invalid technician contract ID.",
            timer: 2500,
        });
        return;
    }

    const endpoint = window.EndpointGetContractTechnicianWhatsAppConversation || "";

    if (!endpoint) {
        swal.fire({
            icon: "error",
            title: "Error!",
            text: "Conversation endpoint not defined.",
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
                    text: "No WhatsApp conversation found for this contract.",
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

/**
 * Send all contracts to technicians via email / whatsapp
 * @param {*} lastUpdateClass - The CSS class to update with the last update timestamp.
 * @param {*} tbClass - The CSS class of the data table to refresh.
 * @param {*} btnSendAllClass - The CSS class of the send all button.
 */
async function sentAllContractTechnician(lastUpdateClass, tbClass, btnSentClass) {
    const endpoint = window.EndpointSendAllContractTechnician || "";
    if (!endpoint) {
        swal.fire({
            icon: "error",
            title: "Error!",
            text: "Send all endpoint not defined.",
            timer: 2500,
        });
        return;
    }

    const $btn = $("." + btnSentClass);

    // Show warning with send method selection
    Swal.fire({
        title: "Send All Contracts?",
        html: `
            <div class="text-start">
                <p class="mb-3">This will send <strong>ALL</strong> contracts to technicians.</p>

                <div class="alert alert-warning" role="alert">
                    <i class="fas fa-exclamation-triangle me-2"></i>
                    <strong>Warning:</strong> This action will send contracts to multiple recipients at once.
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

                <p class="mb-0 text-muted"><small>All contracts will be sent to the technicians' registered contacts.</small></p>
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
            const methodIcon = sendMethod === "email" ? "fal fa-envelope" : "fab fa-whatsapp";

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
                                <p><i class="${methodIcon} me-2"></i>${response.message || `All contracts sent via ${methodName}!`}</p>
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
                    let errorMsg = `Failed to send all contracts via ${methodName}.`;
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