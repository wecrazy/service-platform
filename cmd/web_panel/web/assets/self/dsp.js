async function refreshDataTicketDSP(endpoint, btnClass, tbClass, lastUpdateClass) {
  const $btn = $("." + btnClass);

  Swal.fire({
    title: "Are you sure?",
    text: "Do you really want to refresh the data?",
    icon: "warning",
    showCancelButton: true,
    confirmButtonText: "Yes, refresh it!",
    cancelButtonText: "Cancel",
    reverseButtons: true,
  }).then((result) => {
    if (result.isConfirmed) {
      $btn
        .prop("disabled", true)
        .removeClass("btn-info")
        .addClass("btn-label-info");

      $.ajax({
        url: endpoint,
        type: "GET",
        dataType: "json",
        success: function (response) {
          Swal.fire({
            icon: "success",
            title: "Success!",
            text: response.message || "Data refreshed successfully!",
          });

          const table = $("." + tbClass).DataTable();
          if (table) {
            table.ajax.reload(null, false);
          }

          const lastUpdateEndpoint = endpoint.replace("refresh", "last_update");
          GetLastUpdateDataTicketDSP(lastUpdateEndpoint, lastUpdateClass);
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
          });
        },
        complete: function () {
          $btn
            .prop("disabled", false)
            .removeClass("btn-label-info")
            .addClass("btn-info");
        },
      });
    }
  });
}


function GetLastUpdateDataTicketDSP(endpoint, lastUpdateClass) {
  $.ajax({
    url: endpoint,
    type: 'GET',
    dataType: 'json',
    success: function (res) {
      if (res.lastUpdated) {
        // Update the content inside the element
        $('.'+lastUpdateClass).html(
          `Last Update: <strong>${res.lastUpdated}</strong>`
        );
      } else {
        // Show a SweetAlert if no timestamp was returned
        Swal.fire({
          icon: 'info',
          title: 'No Data',
          text: res.message || 'No last‑update timestamp returned.'
        });
      }
    },
    error: function (xhr, status, error) {
      // Show a SweetAlert on AJAX error
      Swal.fire({
        icon: 'error',
        title: 'Request Failed',
        text: `Failed to fetch last update: ${xhr.status} ${xhr.statusText}`
      });
    }
  });
}

function downloadExcelReportAllDSPTicket(userID, endpoint, request) {
  let reportDownloadTimeout = 10 * 1000 * 60; // 15 minutes
  const reportBtn = document.querySelector('.excel-report');
  let dropdown = bootstrap.Dropdown.getInstance(reportBtn);
  if (!dropdown) {
    dropdown = new bootstrap.Dropdown(reportBtn);
  }
  dropdown.hide();
  reportBtn.disabled = true;
  reportBtn.innerHTML = `<i class="fad fa-spinner me-2"></i> Generating...`;
  // Add current datetime to filename (YYYY-MM-DD_HH-MM-SS)
  const now = new Date();
  const pad = (n) => n.toString().padStart(2, '0');
  const dateStr = `${now.getFullYear()}-${pad(now.getMonth()+1)}-${pad(now.getDate())}_${pad(now.getHours())}-${pad(now.getMinutes())}-${pad(now.getSeconds())}`;
  let filename;
  if (request == "ALL") {
    filename = `all_data_report_${dateStr}.xlsx`;
  } else {
    filename = `all_data_report(filtered)_${dateStr}.xlsx`;
  }

  let localStorageFilters = localStorage.getItem('datatableFiltersTicketDSP');
  if (localStorageFilters) {
    localStorageFilters = JSON.parse(localStorageFilters);

    localStorageFilters.draw = parseInt(localStorageFilters.draw) || 1;
    localStorageFilters.start = parseInt(localStorageFilters.start) || 0;
    localStorageFilters.length = parseInt(localStorageFilters.length) || 10;

    // Flatten nested search object
    if (typeof localStorageFilters.search === "object" && localStorageFilters.search !== null) {
      localStorageFilters["search[value]"] = localStorageFilters.search.value || "";
      delete localStorageFilters.search;
    }

    // Flatten nested order array
    if (Array.isArray(localStorageFilters.order) && localStorageFilters.order.length > 0) {
      localStorageFilters["order[0][column]"] = localStorageFilters.order[0].column;
      localStorageFilters["order[0][dir]"] = localStorageFilters.order[0].dir;
      delete localStorageFilters.order;
    }
  }

  $.ajax({
    url: endpoint,
    method: 'POST',
    data: {
      user_id: userID,
      ...localStorageFilters,
    },
    xhrFields: {
      responseType: 'blob' // assuming backend returns a file
    },
    timeout: reportDownloadTimeout,
    success: function (data, status, xhr) {
      const contentType = xhr.getResponseHeader('Content-Type');
      // If backend returns JSON error (not a file), handle it gracefully
      if (contentType && contentType.includes('application/json')) {
        // Always use FileReader to read the blob as text before parsing as JSON
        const reader = new FileReader();
        reader.onload = function () {
          try {
            const json = JSON.parse(reader.result);
            // Show only the error message if present
            if (json.error) {
              Swal.fire({
                title: 'Error',
                text: json.error,
                icon: 'error'
              });
            } else {
              let details = '';
              for (const key in json) {
                if (json.hasOwnProperty(key)) {
                  details += `${key}: ${json[key]}\n`;
                }
              }
              Swal.fire({
                title: 'Error',
                html: `<pre style="text-align:left;">${details}</pre>`,
                icon: 'error'
              });
            }
          } catch (e) {
            Swal.fire('Error', 'Malformed JSON error response.' + '(' + e + ')', 'error');
          }
        };
        reader.onerror = function () {
          Swal.fire('Error', 'Failed to read error blob.', 'error');
        };
        // Always treat 'data' as a Blob, never try to parse directly
        if (data instanceof Blob) {
          reader.readAsText(data);
        } else {
          // Fallback: if not a Blob, show as plain text
          Swal.fire('Error', 'Malformed error response (not a Blob).', 'error');
        }
        return;
      }

      // Otherwise, treat as file download
      const disposition = xhr.getResponseHeader('Content-Disposition');
      if (disposition && disposition.indexOf('filename=') !== -1) {
        filename = disposition.split('filename=')[1].replace(/['"]/g, '');
      }

      const url = window.URL.createObjectURL(data);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
    },
    error: function (xhr, status, error) {
      const contentType = xhr.getResponseHeader("Content-Type");

      console.error(error);
      console.error(xhr);
      console.error(status);

      // Handle blob (Excel download) errors
      if (xhr.responseType === 'blob' || contentType?.includes("application/octet-stream")) {
        const reader = new FileReader();
        reader.onload = function () {
          Swal.fire({
            title: 'Error',
            html: `<pre style="text-align:left;">${reader.result}</pre>`,
            icon: 'error'
          });
        };
        reader.onerror = function () {
          Swal.fire('Error', 'Failed to read error blob.', 'error');
        };
        reader.readAsText(xhr.response);
        return;
      }

      // Handle JSON errors (for API-related issues)
      if (contentType?.includes("application/json")) {
        try {
          const json = JSON.parse(xhr.responseText);
          let details = '';
          for (const key in json) {
            if (json.hasOwnProperty(key)) {
              details += `${key}: ${json[key]}\n`;
            }
          }
          Swal.fire({
            title: 'Error',
            html: `<pre style="text-align:left;">${details}</pre>`,
            icon: 'error'
          });
        } catch (e) {
          Swal.fire('Error', 'Malformed JSON error response.' + '(' + e + ')', 'error');
        }
      } else {
        // Fallback to plain text
        Swal.fire({
          title: 'Error',
          text: `Status: ${xhr.status} ${xhr.statusText}\n\n${xhr.responseText}`,
          icon: 'error'
        });
      }
    },
    complete: function () {
      reportBtn.disabled = false;
      reportBtn.innerHTML = ` <i class="fal fa-file-spreadsheet me-2"></i> Report`;
    }
  });
}