// Declare Highcharts container IDs used in this file
const PM_CHART_ID = 'pie-chart-pm-mti';
const NON_PM_CHART_ID = 'bar-chart-non-pm-mti';

// Declare pivot container IDs
const tbPMDetailsClass = "dt_data_pm_mti";
const pvtPMMTIID = "pivot-pm-mti";
const tbNonPMDetailsClass = "dt_data_non_pm_mti";
const pvtNonPMMTIID = "pivot-non-pm-mti";

(function () {
    // Check if isDarkStyle is defined, otherwise default to false
    window.isDark = (typeof isDarkStyle !== 'undefined') ? isDarkStyle : false;

    var themeUrl = window.isDark
        ? `/assets/vendor/libs/highcharts-11.4.8/code/themes/high-contrast-dark.js`
        : `/assets/vendor/libs/highcharts-11.4.8/code/themes/high-contrast-light.js`;

    var s = document.createElement('script');
    s.src = themeUrl;
    s.onload = function () {
        // // Theme loaded, but we'll apply classes after chart creation
        // console.log('Highcharts theme loaded:', window.isDark ? 'dark' : 'light');
    };
    document.head.appendChild(s);
})();

async function refreshDataMTI(endpoint, lastUpdateClass, tbClass, btnRefreshClass) {
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
                .removeClass("btn-secondary")
                .addClass("btn-label-secondary")
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

                    const lastUpdateEndpoint = endpoint.replace("refresh-task", "last_update");
                    GetLastUpdateMTIData(lastUpdateEndpoint, lastUpdateClass);
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
                        .removeClass("btn-label-secondary")
                        .addClass("btn-secondary")
                        .html(originalBtnContent);
                },
            });
        }
    });
}

function GetLastUpdateMTIData(endpoint, lastUpdateClass) {
    $.ajax({
        url: endpoint,
        type: "GET",
        dataType: "json",
        success: function (response) {
            if (response && response.last_update) {
                $("." + lastUpdateClass).text(response.last_update);
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

// Helper function to open map in a small window positioned on the left
function openMapWindow(url) {
    window.open(url, '_blank', 'width=400,height=600,left=50,top=100');
}

function openPopupODOOMSPhotos(idTask, table) {
    // Photos
    window.open(
        "/odooms/task-photos/" + idTask + "?table=" + table,
        "popupWindowRight",
        "width=400,height=700,top=20,left=" +
        (window.screen.width - 420) +
        ",scrollbars=yes,resizable=yes"
    );
}

function initPivotPMMTI(endpoint, dataReq) {
    // Ensure required DataTables fields are numeric
    dataReq.draw = parseInt(dataReq.draw) || 1;
    dataReq.start = parseInt(dataReq.start) || 0;
    dataReq.length = parseInt(dataReq.length) || 10;

    // Flatten nested search object (required by your Go backend)
    if (typeof dataReq.search === "object") {
        dataReq["search[value]"] = dataReq.search.value || "";
        delete dataReq.search;
    }

    // Flatten nested order object
    if (Array.isArray(dataReq.order) && dataReq.order.length > 0) {
        dataReq["order[0][column]"] = dataReq.order[0].column;
        dataReq["order[0][dir]"] = dataReq.order[0].dir;
        delete dataReq.order;
    }

    $.ajax({
        url: endpoint.replace('table_pm', 'pivot_pm'), // Use pivot endpoint
        type: "POST",
        data: dataReq,
        success: function (response) {
            const jsonData = response.data || [];

            // Map data for pivot table
            const mappedData = jsonData.map(item => ({
                "NAMA VENDOR BANK": item.nama_vendor_bank || "Unknown",
                "TERKUNJUNGI Count": parseInt(item.terkunjungi_count) || 0,
                "TERKUNJUNGI Percentage": parseFloat(item.terkunjungi_percentage) || 0,
                "GAGAL TERKUNJUNGI Count": parseInt(item.gagal_terkunjungi_count) || 0,
                "GAGAL TERKUNJUNGI Percentage": parseFloat(item.gagal_terkunjungi_percentage) || 0,
                "BELUM KUNJUNGAN Count": parseInt(item.belum_kunjungan_count) || 0,
                "BELUM KUNJUNGAN Percentage": parseFloat(item.belum_kunjungan_percentage) || 0,
                "Total Count of TID": parseInt(item.total_count) || 0,
                "Run Rate": parseInt(item.run_rate) || 0,
                "RUN RATE (%)": parseFloat(item.run_rate_percentage) || 0,
                "TARGET PER HARI": parseInt(item.target_per_hari) || 0
            }));

            // Create custom HTML table matching Excel layout exactly
            let tableHTML = `
                <div class="table-responsive">
                    <table class="table table-bordered small" style="font-size: 0.85rem;">
                        <thead>
                            <tr style="background-color: #f8f9fa;">
                                <th rowspan="2" style="vertical-align: middle; background-color: #6c757d; color: white; text-align: center;">NAMA VENDOR<br>(BANK)</th>
                                <th colspan="2" style="background-color: #28a745; color: white; text-align: center;">TERKUNJUNGI</th>
                                <th colspan="2" style="background-color: #fd7e14; color: white; text-align: center;">GAGAL TERKUNJUNGI</th>
                                <th colspan="2" style="background-color: #17a2b8; color: white; text-align: center;">BELUM KUNJUNGAN</th>
                                <th rowspan="2" style="vertical-align: middle; background-color: #6c757d; color: white; text-align: center;">Total Count of TID</th>
                                <th rowspan="2" style="vertical-align: middle; background-color: #6c757d; color: white; text-align: center;">Run Rate</th>
                                <th rowspan="2" style="vertical-align: middle; background-color: #6c757d; color: white; text-align: center;">RUN RATE (%)</th>
                                <th rowspan="2" style="vertical-align: middle; background-color: #6c757d; color: white; text-align: center;">TARGET PER HARI</th>
                            </tr>
                            <tr style="background-color: #f8f9fa;">
                                <th style="background-color: #28a745; color: white; text-align: center;">Count of TID</th>
                                <th style="background-color: #28a745; color: white; text-align: center;">Percentage (%)</th>
                                <th style="background-color: #fd7e14; color: white; text-align: center;">Count of TID</th>
                                <th style="background-color: #fd7e14; color: white; text-align: center;">Percentage (%)</th>
                                <th style="background-color: #17a2b8; color: white; text-align: center;">Count of TID</th>
                                <th style="background-color: #17a2b8; color: white; text-align: center;">Percentage (%)</th>
                            </tr>
                        </thead>
                        <tbody>
            `;

            // Calculate totals
            let totals = {
                terkunjungiCount: 0,
                terkunjungiPercentage: 0,
                gagalCount: 0,
                gagalPercentage: 0,
                belumCount: 0,
                belumPercentage: 0,
                totalCount: 0,
                runRate: 0,
                runRatePercentage: 0,
                targetPerHari: 0
            };

            // Add data rows
            mappedData.forEach(item => {
                totals.terkunjungiCount += item["TERKUNJUNGI Count"];
                totals.gagalCount += item["GAGAL TERKUNJUNGI Count"];
                totals.belumCount += item["BELUM KUNJUNGAN Count"];
                totals.totalCount += item["Total Count of TID"];
                totals.runRate += item["Run Rate"];
                totals.targetPerHari += item["TARGET PER HARI"];

                tableHTML += `
                    <tr>
                        <td style="background-color: #f8f9fa; font-weight: bold;">
                            <a href="#" class="vendor-bank-filter text-decoration-none fw-bold" 
                               data-vendor="${item["NAMA VENDOR BANK"]}" 
                               style="color: #0d6efd; cursor: pointer;">
                                ${item["NAMA VENDOR BANK"]}
                            </a>
                        </td>
                        <td class="text-center">${item["TERKUNJUNGI Count"].toLocaleString()}</td>
                        <td class="text-center">${item["TERKUNJUNGI Percentage"].toFixed(2)}%</td>
                        <td class="text-center">${item["GAGAL TERKUNJUNGI Count"].toLocaleString()}</td>
                        <td class="text-center">${item["GAGAL TERKUNJUNGI Percentage"].toFixed(2)}%</td>
                        <td class="text-center">${item["BELUM KUNJUNGAN Count"].toLocaleString()}</td>
                        <td class="text-center" style="background-color: ${item["BELUM KUNJUNGAN Percentage"] > 95 ? '#ffcccc' : 'transparent'};">${item["BELUM KUNJUNGAN Percentage"].toFixed(2)}%</td>
                        <td class="text-center">${item["Total Count of TID"].toLocaleString()}</td>
                        <td class="text-center" style="font-weight: bold;">${item["Run Rate"].toLocaleString()}</td>
                        <td class="text-center" style="background-color: ${item["RUN RATE (%)"] > 15 ? '#ffcccc' : 'transparent'};">${item["RUN RATE (%)"].toFixed(2)}%</td>
                        <td class="text-center">${item["TARGET PER HARI"]}</td>
                    </tr>
                `;
            });

            // Calculate average percentages for totals
            const avgTerkunjungiPercentage = mappedData.length > 0 ? (totals.terkunjungiCount / totals.totalCount * 100) : 0;
            const avgGagalPercentage = mappedData.length > 0 ? (totals.gagalCount / totals.totalCount * 100) : 0;
            const avgBelumPercentage = mappedData.length > 0 ? (totals.belumCount / totals.totalCount * 100) : 0;
            const avgRunRatePercentage = mappedData.length > 0 ? (totals.runRate / totals.totalCount * 100) : 0;

            // Add totals row
            tableHTML += `
                    <tr style="background-color: #e9ecef; font-weight: bold;">
                        <td class="bg-dark text-white" style="text-align: center;">Grand Total</td>
                        <td class="text-center">${totals.terkunjungiCount.toLocaleString()}</td>
                        <td class="text-center">${avgTerkunjungiPercentage.toFixed(2)}%</td>
                        <td class="text-center">${totals.gagalCount.toLocaleString()}</td>
                        <td class="text-center">${avgGagalPercentage.toFixed(2)}%</td>
                        <td class="text-center">${totals.belumCount.toLocaleString()}</td>
                        <td class="text-center">${avgBelumPercentage.toFixed(2)}%</td>
                        <td class="text-center">${totals.totalCount.toLocaleString()}</td>
                        <td class="text-center">${totals.runRate.toLocaleString()}</td>
                        <td class="text-center">${avgRunRatePercentage.toFixed(2)}%</td>
                        <td class="text-center">${totals.targetPerHari.toLocaleString()}</td>
                    </tr>
                        </tbody>
                    </table>
                </div>
            `;

            // Clear existing pivot and add custom table
            $("#" + pvtPMMTIID).html(tableHTML);

            // Add click event handler for vendor bank filtering
            $("#" + pvtPMMTIID).off('click', '.vendor-bank-filter').on('click', '.vendor-bank-filter', function (e) {
                e.preventDefault();

                const vendorBank = $(this).data('vendor');

                // Find the source input field
                const sourceInput = $('#ft_dt_data_pm_mti_source');

                if (sourceInput.length > 0) {
                    // Set the value in the source input field
                    sourceInput.val(vendorBank);

                    // Trigger the input event to apply the filter (if there's an event handler)
                    sourceInput.trigger('input').trigger('keyup').trigger('change');

                    // Also try to apply DataTable column search if table exists
                    const pmTable = $('.dt_data_pm_mti').DataTable();
                    if (pmTable) {
                        // Get column index from data attribute (10 based on your input)
                        const columnIndex = parseInt(sourceInput.data('column-index')) || 10;
                        pmTable.column(columnIndex).search(vendorBank).draw();
                    }

                    // Show success message
                    Swal.fire({
                        icon: 'success',
                        title: 'Filter Applied',
                        text: `Source filter set to: ${vendorBank}`,
                        timer: 2000,
                        showConfirmButton: false
                    });

                    // Scroll to the main table
                    $('html, body').animate({
                        scrollTop: $('.dt_data_pm_mti').offset().top - 100
                    }, 800);
                } else {
                    Swal.fire({
                        icon: 'warning',
                        title: 'Filter Input Not Found',
                        text: 'Could not find the source filter input field.',
                        timer: 2000
                    });
                }
            });

            // Store bank data for pie chart selection
            window.bankData = mappedData;

            // Generate initial pie chart data from totals (all banks combined)
            let selectedBank = 'all';
            let pieChartData = [
                {
                    name: 'TERKUNJUNGI',
                    y: totals.terkunjungiCount
                },
                {
                    name: 'GAGAL TERKUNJUNGI',
                    y: totals.gagalCount
                },
                {
                    name: 'BELUM KUNJUNGAN',
                    y: totals.belumCount
                }
            ];

            // Create bank selector dropdown
            const bankSelectorHTML = `
                <div class="mb-3 bank-selector-wrapper">
                    <label for="bank-selector" class="form-label fw-bold">Select Bank/Vendor:</label>
                    <select id="bank-selector" class="form-select">
                        <option value="all">All Banks Combined</option>
                        ${mappedData.map(bank => `<option value="${bank["NAMA VENDOR BANK"]}">${bank["NAMA VENDOR BANK"]}</option>`).join('')}
                    </select>
                </div>
            `;

            // Add bank selector above the pie chart
            const pieChartContainer = $(`#${PM_CHART_ID}`).parent();
            pieChartContainer.find('.bank-selector-wrapper').remove(); // Remove old selector wrapper if exists
            pieChartContainer.prepend(bankSelectorHTML);

            // Function to update pie chart based on selected bank
            function updatePieChart(selectedBank) {
                let chartData;
                let chartTitle;
                let chartSubtitle;

                if (selectedBank === 'all') {
                    chartData = [
                        { name: 'TERKUNJUNGI', y: totals.terkunjungiCount },
                        { name: 'GAGAL TERKUNJUNGI', y: totals.gagalCount },
                        { name: 'BELUM KUNJUNGAN', y: totals.belumCount }
                    ];
                    chartTitle = 'PM Status Distribution - All Banks';
                    chartSubtitle = `Total TID Count: ${totals.totalCount.toLocaleString()}`;
                } else {
                    const bankData = mappedData.find(bank => bank["NAMA VENDOR BANK"] === selectedBank);
                    if (bankData) {
                        chartData = [
                            { name: 'TERKUNJUNGI', y: bankData["TERKUNJUNGI Count"] },
                            { name: 'GAGAL TERKUNJUNGI', y: bankData["GAGAL TERKUNJUNGI Count"] },
                            { name: 'BELUM KUNJUNGAN', y: bankData["BELUM KUNJUNGAN Count"] }
                        ];
                        chartTitle = `PM Status Distribution - ${selectedBank}`;
                        chartSubtitle = `Total TID Count: ${bankData["Total Count of TID"].toLocaleString()}`;
                    }
                }

                // Update existing chart or create new one
                const chart = Highcharts.chart(PM_CHART_ID, {
                    chart: {
                        type: 'pie',
                        height: 400
                    },
                    title: {
                        text: chartTitle
                    },
                    subtitle: {
                        text: chartSubtitle
                    },
                    tooltip: {
                        pointFormat: '{series.name}: <b>{point.y}</b> ({point.percentage:.1f}%)<br/>'
                    },
                    plotOptions: {
                        pie: {
                            allowPointSelect: true,
                            cursor: 'pointer',
                            dataLabels: {
                                enabled: true,
                                format: '<b>{point.name}</b>: {point.y} ({point.percentage:.1f}%)',
                                style: {
                                    fontSize: '12px'
                                }
                            },
                            showInLegend: true
                        }
                    },
                    legend: {
                        align: 'center',
                        verticalAlign: 'bottom',
                        layout: 'horizontal'
                    },
                    series: [{
                        name: 'Count',
                        colorByPoint: true,
                        data: chartData
                    }],
                    credits: {
                        enabled: false
                    }
                });

                // Apply theme classes after chart creation
                if (window.isDark) {
                    $(`#${PM_CHART_ID}`).removeClass('highcharts-light').addClass('highcharts-dark');
                } else {
                    $(`#${PM_CHART_ID}`).removeClass('highcharts-dark').addClass('highcharts-light');
                }

                return chart;
            }

            // Create initial pie chart
            updatePieChart(selectedBank);

            // Add event listener for bank selector
            $('#bank-selector').on('change', function () {
                selectedBank = $(this).val();
                updatePieChart(selectedBank);
            });

        },
        error: function (xhr, status, error) {
            let statusCode = xhr.status;
            let statusText = xhr.statusText || 'Unknown Status';
            let errorMsg = error && error !== 'error' ? `: ${error}` : '';

            Swal.fire({
                icon: 'error',
                title: 'Request Failed',
                text: `Failed to load pivot PM data (Status ${statusCode} ${statusText}${errorMsg})`
            });

            // Show error message in pivot container
            $("#" + pvtPMMTIID).html(`
        <div class="alert alert-danger">
          <h5>Error Loading Pivot Data</h5>
          <p>Status ${statusCode} ${statusText}${errorMsg}</p>
        </div>
      `);
        }
    });
}

function initPivotNonPMMTI(endpoint, dataReq) {
    // Ensure required DataTables fields are numeric
    dataReq.draw = parseInt(dataReq.draw) || 1;
    dataReq.start = parseInt(dataReq.start) || 0;
    dataReq.length = parseInt(dataReq.length) || 10;

    // Flatten nested search object (required by your Go backend)
    if (typeof dataReq.search === "object") {
        dataReq["search[value]"] = dataReq.search.value || "";
        delete dataReq.search;
    }

    // Flatten nested order object
    if (Array.isArray(dataReq.order) && dataReq.order.length > 0) {
        dataReq["order[0][column]"] = dataReq.order[0].column;
        dataReq["order[0][dir]"] = dataReq.order[0].dir;
        delete dataReq.order;
    }

    $.ajax({
        url: endpoint.replace('table_non_pm', 'pivot_non_pm'), // Use pivot endpoint
        type: "POST",
        data: dataReq,
        success: function (response) {
            const jsonData = response.data || [];

            // Map data for Non-PM pivot table based on excel structure
            const mappedData = jsonData.map(item => ({
                "Provinsi": item.provinsi || "N/A",
                "Kota": item.kota || "Unknown",
                "Nama SP Leader": item.sp_leader || "Unknown",
                "Nama Teknisi": item.teknisi || "Unknown",
                "Activity": item.activity || "Unknown",
                "Priority 1 Miss SLA": parseInt(item.priority_1_miss_sla) || 0,
                "Priority 1 Must Visit Today": parseInt(item.priority_1_must_visit_today) || 0,
                "Priority 2 Must Visit Tomorrow": parseInt(item.priority_2_must_visit_tomorrow) || 0,
                "Priority 3 Must Visit More Than Tomorrow": parseInt(item.priority_3_must_visit_more_than_tomorrow) || 0,
                "Pending Merchant / Bucket Business MTI": parseInt(item.pending_merchant_or_bucket_business) || 0
            }));

            // Create custom HTML table matching Excel layout exactly
            let tableHTML = `
                <div class="table-responsive">
                    <table class="table table-bordered small" style="font-size: 0.85rem;">
                        <thead>
                            <tr style="background-color: #f8f9fa;">
                                <th rowspan="2" style="vertical-align: middle; background-color: #6c757d; color: white; text-align: center;">Provinsi</th>
                                <th rowspan="2" style="vertical-align: middle; background-color: #6c757d; color: white; text-align: center;">Kota</th>
                                <th rowspan="2" style="vertical-align: middle; background-color: #6c757d; color: white; text-align: center;">Nama SP Leader</th>
                                <th rowspan="2" style="vertical-align: middle; background-color: #6c757d; color: white; text-align: center;">Nama Teknisi</th>
                                <th rowspan="2" style="vertical-align: middle; background-color: #6c757d; color: white; text-align: center;">Activity</th>
                                <th colspan="2" style="background-color: #ffd700; color: black; text-align: center;">Prioritas 1</th>
                                <th style="background-color: #FFB347; color: black; text-align: center;">Prioritas 2</th>
                                <th style="background-color: #87CEEB; color: black; text-align: center;">Prioritas 3</th>
                                <th rowspan="2" style="vertical-align: middle; background-color: #f0f0f0; color: black; text-align: center;">Pending Merchant /<br>Bucket Business MTI</th>
                            </tr>
                            <tr style="background-color: #f8f9fa;">
                                <th style="background-color: #dc3545; color: white; text-align: center; font-size: 0.75rem;">Miss SLA</th>
                                <th style="background-color: #90EE90; color: black; text-align: center; font-size: 0.75rem;">HARUS SELESAI<br>HARI INI / HO<br>Termasuk re-schedule</th>
                                <th style="background-color: #FFB347; color: black; text-align: center; font-size: 0.75rem;">HARUS SELESAI<br>H+1</th>
                                <th style="background-color: #87CEEB; color: black; text-align: center; font-size: 0.75rem;">HARUS SELESAI<br>>= H+2</th>
                            </tr>
                        </thead>
                        <tbody>
            `;

            // Calculate totals
            let totals = {
                priority1MissSLA: 0,
                priority1MustVisitToday: 0,
                priority2MustVisitTomorrow: 0,
                priority3MustVisitMoreThanTomorrow: 0,
                pendingMerchantOrBucketBusiness: 0
            };

            // Add data rows
            mappedData.forEach(item => {
                totals.priority1MissSLA += item["Priority 1 Miss SLA"];
                totals.priority1MustVisitToday += item["Priority 1 Must Visit Today"];
                totals.priority2MustVisitTomorrow += item["Priority 2 Must Visit Tomorrow"];
                totals.priority3MustVisitMoreThanTomorrow += item["Priority 3 Must Visit More Than Tomorrow"];
                totals.pendingMerchantOrBucketBusiness += item["Pending Merchant / Bucket Business MTI"];

                tableHTML += `
                    <tr>
                        <td class="text-center">${item["Provinsi"]}</td>
                        <td class="text-center">${item["Kota"]}</td>
                        <td class="text-center">${item["Nama SP Leader"]}</td>
                        <td class="text-center">
                            <a href="javascript:void(0);" 
                               class="text-primary fw-bold technician-filter-link" 
                               data-technician="${item["Nama Teknisi"]}" 
                               data-activity="${item["Activity"]}"
                               title="Click to filter detail data for this technician and activity"
                               style="cursor: pointer; text-decoration: underline;">
                                ${item["Nama Teknisi"]}
                            </a>
                        </td>
                        <td class="text-center">${item["Activity"]}</td>
                        <td class="text-center" style="background-color: #dc3545; color: white;">${item["Priority 1 Miss SLA"].toLocaleString()}</td>
                        <td class="text-center" style="background-color: #90EE90; color: black;">${item["Priority 1 Must Visit Today"].toLocaleString()}</td>
                        <td class="text-center" style="background-color: #FFB347; color: black;">${item["Priority 2 Must Visit Tomorrow"].toLocaleString()}</td>
                        <td class="text-center" style="background-color: #87CEEB; color: black;">${item["Priority 3 Must Visit More Than Tomorrow"].toLocaleString()}</td>
                        <td class="text-center" style="background-color: #f0f0f0; color: black;">${item["Pending Merchant / Bucket Business MTI"].toLocaleString()}</td>
                    </tr>
                `;
            });

            // Add totals row
            tableHTML += `
                    <tr style="background-color: #e9ecef; font-weight: bold;">
                        <td class="bg-dark text-white text-center" colspan="5">GRAND TOTAL</td>
                        <td class="text-center" style="background-color: #dc3545; color: white;">${totals.priority1MissSLA.toLocaleString()}</td>
                        <td class="text-center" style="background-color: #90EE90; color: black;">${totals.priority1MustVisitToday.toLocaleString()}</td>
                        <td class="text-center" style="background-color: #FFB347; color: black;">${totals.priority2MustVisitTomorrow.toLocaleString()}</td>
                        <td class="text-center" style="background-color: #87CEEB; color: black;">${totals.priority3MustVisitMoreThanTomorrow.toLocaleString()}</td>
                        <td class="text-center" style="background-color: #f0f0f0; color: black;">${totals.pendingMerchantOrBucketBusiness.toLocaleString()}</td>
                    </tr>
                        </tbody>
                    </table>
                </div>
            `;

            // Clear existing pivot and add custom table
            $("#" + pvtNonPMMTIID).html(tableHTML);

            // Add hover effects for technician filter links
            if (!$('#technician-filter-styles').length) {
                $('<style id="technician-filter-styles">')
                    .text(`
                        .technician-filter-link:hover {
                            background-color: #e3f2fd !important;
                            border-radius: 4px;
                            padding: 2px 4px;
                            transform: scale(1.05);
                            transition: all 0.2s ease;
                        }
                        .technician-filter-link:active {
                            transform: scale(0.98);
                        }
                    `)
                    .appendTo('head');
            }

            // Generate combination chart data for Non-PM priorities
            // Priority 1 will be stacked, Priority 2 & 3 will be regular bars
            let chartData = [
                {
                    name: 'Priority 1 - Miss SLA',
                    data: [totals.priority1MissSLA],
                    color: '#dc3545',
                    stack: 'priority1'
                },
                {
                    name: 'Priority 1 - SLA Today / HO',
                    data: [totals.priority1MustVisitToday],
                    color: '#90EE90',
                    stack: 'priority1'
                },
                {
                    name: 'Priority 2 - SLA H+1',
                    data: [totals.priority2MustVisitTomorrow],
                    color: '#FFB347',
                    stack: 'priority2'
                },
                {
                    name: 'Priority 3 - SLA >= H+2',
                    data: [totals.priority3MustVisitMoreThanTomorrow],
                    color: '#87CEEB',
                    stack: 'priority3'
                }
            ];

            // Create combination chart for Non-PM priorities
            Highcharts.chart(NON_PM_CHART_ID, {
                chart: {
                    type: 'column',
                    backgroundColor: '#ffffff'
                },
                title: {
                    text: 'Non-PM Task Priorities Distribution'
                },
                subtitle: {
                    text: 'Priority 1: Stacked bars | Priority 2 & 3: Regular bars'
                },
                xAxis: {
                    categories: ['Task Priorities'],
                    labels: {
                        style: {
                            fontSize: '12px',
                            fontFamily: 'Verdana, sans-serif'
                        }
                    }
                },
                yAxis: {
                    min: 0,
                    title: {
                        text: 'Number of Tasks'
                    },
                    stackLabels: {
                        enabled: true,
                        style: {
                            fontWeight: 'bold',
                            color: 'gray'
                        }
                    }
                },
                legend: {
                    align: 'right',
                    x: -30,
                    verticalAlign: 'bottom',
                    y: 25,
                    floating: true,
                    backgroundColor: 'white',
                    borderColor: '#CCC',
                    borderWidth: 1,
                    shadow: false
                },
                tooltip: {
                    headerFormat: '<b>{point.x}</b><br/>',
                    pointFormat: '{series.name}: {point.y}<br/>Total: {point.stackTotal}'
                },
                plotOptions: {
                    column: {
                        stacking: 'normal',
                        dataLabels: {
                            enabled: true,
                            color: 'white',
                            style: {
                                textOutline: '1px contrast'
                            }
                        }
                    }
                },
                series: chartData,
                credits: { enabled: false },
            });

            // Apply theme classes after chart creation
            if (window.isDark) {
                $(`#${NON_PM_CHART_ID}`).removeClass('highcharts-light').addClass('highcharts-dark');
            } else {
                $(`#${NON_PM_CHART_ID}`).removeClass('highcharts-dark').addClass('highcharts-light');
            }

            // Add click event handler for technician filter links
            $(document).off('click', '.technician-filter-link').on('click', '.technician-filter-link', function (e) {
                e.preventDefault();

                const technician = $(this).data('technician');
                const activity = $(this).data('activity');

                // Find the technician and task_type input fields using the same pattern as PM
                let technicianInput = $('#ft_dt_data_non_pm_mti_technician');
                let taskTypeInput = $('#ft_dt_data_non_pm_mti_task_type');

                // Fallback to generic field names if specific IDs don't exist
                if (technicianInput.length === 0) {
                    technicianInput = $('input[name="technician"]');
                }
                if (taskTypeInput.length === 0) {
                    taskTypeInput = $('input[name="task_type"]');
                }

                // Clear existing filters first
                if (technicianInput.length > 0) {
                    technicianInput.val('');
                }
                if (taskTypeInput.length > 0) {
                    taskTypeInput.val('');
                }

                // Set the specific filters
                if (technicianInput.length > 0) {
                    technicianInput.val(technician);
                    // Trigger input events to apply the filter
                    technicianInput.trigger('input').trigger('keyup').trigger('change');
                }

                if (taskTypeInput.length > 0) {
                    taskTypeInput.val(activity);
                    // Trigger input events to apply the filter
                    taskTypeInput.trigger('input').trigger('keyup').trigger('change');
                }

                // Also try to apply DataTable column search if table exists
                const nonPmTable = $('.dt_data_non_pm_mti').DataTable();
                if (nonPmTable) {
                    // Get column indexes for technician and task_type
                    const technicianColumnIndex = parseInt(technicianInput.data('column-index'));
                    const taskTypeColumnIndex = parseInt(taskTypeInput.data('column-index'));

                    if (!isNaN(technicianColumnIndex)) {
                        nonPmTable.column(technicianColumnIndex).search(technician);
                    }
                    if (!isNaN(taskTypeColumnIndex)) {
                        nonPmTable.column(taskTypeColumnIndex).search(activity);
                    }

                    // Redraw the table
                    nonPmTable.draw();
                }

                // Show success message
                Swal.fire({
                    icon: 'success',
                    title: 'Filter Applied',
                    text: `Technician: ${technician}, Activity: ${activity}`,
                    timer: 2000,
                    showConfirmButton: false
                });

                // Scroll to the detail table if it exists
                if ($('.' + tbNonPMDetailsClass).length > 0) {
                    $('html, body').animate({
                        scrollTop: $('.' + tbNonPMDetailsClass).offset().top - 100
                    }, 500);
                }
            });

        },
        error: function (xhr, status, error) {
            let statusCode = xhr.status;
            let statusText = xhr.statusText || 'Unknown Status';
            let errorMsg = error && error !== 'error' ? `: ${error}` : '';

            Swal.fire({
                icon: 'error',
                title: 'Request Failed',
                text: `Failed to load pivot Non-PM data (Status ${statusCode} ${statusText}${errorMsg})`
            });

            // Show error message in pivot container
            $("#" + pvtNonPMMTIID).html(`
        <div class="alert alert-danger">
          <h5>Error Loading Non-PM Pivot Data</h5>
          <p>Status ${statusCode} ${statusText}${errorMsg}</p>
        </div>
      `);
        }
    });
}

function downloadExcelReportPMMTI(userId, endpoint, reportType) {
    var $dropdown = $('.excel-report-pm.dropdown-toggle');
    var $dropdownMenu = $('.excel-report-pm').parent().find('.dropdown-menu');
    
    // Store original content
    var originalText = $dropdown.html();
    
    // Show spinner in dropdown button and keep it clickable for dropdown to work
    $dropdown.html('<span class="spinner-border spinner-border-sm me-2" role="status"></span>Generating...');
    
    // Disable all dropdown menu items to prevent multiple downloads
    $dropdownMenu.find('a').addClass('disabled').css({
        'pointer-events': 'none',
        'opacity': '0.5',
        'color': '#6c757d'
    });
    
    // Close dropdown menu initially but allow it to open again
    $dropdown.dropdown('hide');
    
    // Prepare form data object
    var formData = {};
    
    // Add current filter data if reportType is 'Filtered'
    if (reportType === 'Filtered') {
        // Get stored filter data from localStorage
        var storedFilters = localStorage.getItem('datatableFiltersPMMTI');
        if (storedFilters) {
            try {
                var filters = JSON.parse(storedFilters);
                // Add all filter data to form
                $.each(filters, function(key, value) {
                    if (value !== null && value !== undefined && value !== '') {
                        formData[key] = value;
                    }
                });
            } catch (e) {
                console.error('Error parsing stored filters:', e);
                // Fallback to just search value
                var currentSearch = $('#dt_data_pm_mti_wrapper .dataTables_filter input').val();
                if (currentSearch) {
                    formData['search[value]'] = currentSearch;
                }
            }
        } else {
            // Fallback to current search value if no stored filters
            var currentSearch = $('#dt_data_pm_mti_wrapper .dataTables_filter input').val();
            if (currentSearch) {
                formData['search[value]'] = currentSearch;
            }
        }
    }
    
    // Create a temporary form for download using jQuery
    var $form = $('<form>', {
        method: 'POST',
        action: endpoint,
        style: 'display: none;'
    });
    
    // Add form data as hidden inputs using jQuery
    $.each(formData, function(key, value) {
        $form.append($('<input>', {
            type: 'hidden',
            name: key,
            value: value
        }));
    });
    
    // Append form to body and submit
    $('body').append($form);
    $form.submit();
    
    // Clean up
    $form.remove();
    
    // Show success message immediately but don't restore button yet
    Swal.fire({
        icon: 'info',
        title: 'Download Initiated',
        text: reportType + ' PM MTI report generation started. Button will be restored shortly.',
        timer: 2000,
        showConfirmButton: false
    });
    
    // Function to restore button state
    function restoreButtonState() {
        $dropdown.html(originalText);
        $dropdownMenu.find('a').removeClass('disabled').css({
            'pointer-events': 'auto',
            'opacity': '',
            'color': ''
        });
    }
    
    // Restore button after a reasonable delay (5 seconds)
    // This is more reliable than waiting for focus events
    setTimeout(function() {
        restoreButtonState();
    }, 5000);
}

function downloadExcelReportNonPMMTI(userId, endpoint, reportType) {
    var $dropdown = $('.excel-report-non-pm.dropdown-toggle');
    var $dropdownMenu = $('.excel-report-non-pm').parent().find('.dropdown-menu');
    
    // Store original content
    var originalText = $dropdown.html();
    
    // Show spinner in dropdown button and keep it clickable for dropdown to work
    $dropdown.html('<span class="spinner-border spinner-border-sm me-2" role="status"></span>Generating...');
    
    // Disable all dropdown menu items to prevent multiple downloads
    $dropdownMenu.find('a').addClass('disabled').css({
        'pointer-events': 'none',
        'opacity': '0.5',
        'color': '#6c757d'
    });
    
    // Close dropdown menu initially but allow it to open again
    $dropdown.dropdown('hide');
    
    // Prepare form data object
    var formData = {};
    
    // Add current filter data if reportType is 'Filtered'
    if (reportType === 'Filtered') {
        // Get stored filter data from localStorage 
        var storedFilters = localStorage.getItem('datatableFiltersNonPMMTI');
        if (storedFilters) {
            try {
                var filters = JSON.parse(storedFilters);
                // Add all filter data to form
                $.each(filters, function(key, value) {
                    if (value !== null && value !== undefined && value !== '') {
                        formData[key] = value;
                    }
                });
            } catch (e) {
                console.error('Error parsing stored filters:', e);
                // Fallback to just search value
                var currentSearch = $('#dt_data_non_pm_mti_wrapper .dataTables_filter input').val();
                if (currentSearch) {
                    formData['search[value]'] = currentSearch;
                }
            }
        } else {
            // Fallback to current search value if no stored filters
            var currentSearch = $('#dt_data_non_pm_mti_wrapper .dataTables_filter input').val();
            if (currentSearch) {
                formData['search[value]'] = currentSearch;
            }
        }
    }
    
    // Create a temporary form for download using jQuery
    var $form = $('<form>', {
        method: 'POST',
        action: endpoint,
        style: 'display: none;'
    });
    
    // Add form data as hidden inputs using jQuery
    $.each(formData, function(key, value) {
        $form.append($('<input>', {
            type: 'hidden',
            name: key,
            value: value
        }));
    });
    
    // Append form to body and submit
    $('body').append($form);
    $form.submit();
    
    // Clean up
    $form.remove();
    
    // Show success message immediately but don't restore button yet
    Swal.fire({
        icon: 'info',
        title: 'Download Initiated',
        text: reportType + ' Non-PM MTI report generation started. Button will be restored shortly.',
        timer: 2000,
        showConfirmButton: false
    });
    
    // Function to restore button state
    function restoreButtonState() {
        $dropdown.html(originalText);
        $dropdownMenu.find('a').removeClass('disabled').css({
            'pointer-events': 'auto',
            'opacity': '',
            'color': ''
        });
    }
    
    // Restore button after a reasonable delay (5 seconds)
    // This is more reliable than waiting for focus events
    setTimeout(function() {
        restoreButtonState();
    }, 5000);
}