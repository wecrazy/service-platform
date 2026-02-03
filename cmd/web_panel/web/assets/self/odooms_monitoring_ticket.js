// webGlobalURL is now set from the HTML template as window.webGlobalURL
const chartID = "container";

(function () {
    // Check if isDarkStyle is defined, otherwise default to false
    const isDark = (typeof isDarkStyle !== 'undefined') ? isDarkStyle : false;

    var themeUrl = isDark
        ? `${webGlobalURL}assets/vendor/libs/highcharts-11.4.8/code/themes/high-contrast-dark.js`
        : `${webGlobalURL}assets/vendor/libs/highcharts-11.4.8/code/themes/high-contrast-light.js`;

    var s = document.createElement('script');
    s.src = themeUrl;
    s.onload = function () {
        // Apply theme classes after script loads
        if (typeof jQuery !== 'undefined') {
            if (isDark) {
                $('#' + chartID).removeClass('highcharts-light').addClass('highcharts-dark');
            } else {
                $('#' + chartID).removeClass('highcharts-dark').addClass('highcharts-light');
            }
        }
    };
    document.head.appendChild(s);
})();

// --- Dynamic select2 population and cascading filter logic ---
$(document).ready(function () {
    // Helper to populate select2 (for multiple select, don't add "All" option)
    function populateSelect($select, data, allLabel) {
        $select.empty();
        // Don't add "All" option for multiple selects
        data.forEach(function (item) {
            $select.append(`<option value="${item}">${item}</option>`);
        });
        $select.val(null).trigger('change');
    }

    // Populate Data Date Range
    $.get('/odooms-monitoring/data-date-range-ticket-performance', function (res) {
        if (res.success && res.data) {
            let dateNow = new Date();
            let dateOptions = { month: 'short', year: 'numeric' };
            let dateFormatted = dateNow.toLocaleDateString('en-US', dateOptions);

            populateSelect($('#multicol-data-date'), res.data, `Current Month (${dateFormatted})`);
        }
    });

    // Populate SAC
    $.get('/odooms-monitoring/data-sac-ticket-performance', function (res) {
        if (res.success && res.data) {
            populateSelect($('#multicol-sac'), res.data, 'All SAC');
        }
    });

    // Populate Company
    function updateCompany(dataDate) {
        let url = '/odooms-monitoring/data-company-ticket-performance';
        if (dataDate) url += '?data_date=' + encodeURIComponent(dataDate);
        $.get(url, function (res) {
            if (res.success && res.data) {
                populateSelect($('#multicol-company'), res.data, 'All Company');
            } else {
                populateSelect($('#multicol-company'), [], 'All Company');
            }
        });
    }

    // Populate SLA Status
    function updateSLAStatus(dataDate) {
        let url = '/odooms-monitoring/data-sla-status-ticket-performance';
        if (dataDate) url += '?data_date=' + encodeURIComponent(dataDate);
        $.get(url, function (res) {
            if (res.success && res.data) {
                populateSelect($('#multicol-sla-status'), res.data, 'All SLA Status');
            } else {
                populateSelect($('#multicol-sla-status'), [], 'All SLA Status');
            }
        });
    }

    // Populate Task Type
    function updateTaskType(dataDate) {
        let url = '/odooms-monitoring/data-task-type-ticket-performance';
        if (dataDate) url += '?data_date=' + encodeURIComponent(dataDate);
        $.get(url, function (res) {
            if (res.success && res.data) {
                populateSelect($('#multicol-task-type'), res.data, 'All Task Type');
            } else {
                populateSelect($('#multicol-task-type'), [], 'All Task Type');
            }
        });
    }

    // Populate SPL and Technician (initially all)
    function updateSPL(sac) {
        let url = '/odooms-monitoring/data-spl-ticket-performance';
        let requestData = {};

        // Handle multiple SAC values (array)
        if (sac && Array.isArray(sac) && sac.length > 0) {
            requestData.sac = sac;
        } else if (sac && !Array.isArray(sac)) {
            requestData.sac = [sac];
        }

        $.ajax({
            url: url,
            type: 'POST',
            contentType: 'application/json',
            data: JSON.stringify(requestData),
            success: function (res) {
                if (res.success && res.data) {
                    populateSelect($('#multicol-spl'), res.data, 'All SPL');
                } else {
                    populateSelect($('#multicol-spl'), [], 'All SPL');
                }
            },
            error: function () {
                populateSelect($('#multicol-spl'), [], 'All SPL');
            }
        });
    }

    function updateTechnician(sac, spl) {
        let url = '/odooms-monitoring/data-technician-ticket-performance';
        let requestData = {};

        // Handle multiple SAC values
        if (sac && Array.isArray(sac) && sac.length > 0) {
            requestData.sac = sac;
        } else if (sac && !Array.isArray(sac) && sac) {
            requestData.sac = [sac];
        }

        // Handle multiple SPL values
        if (spl && Array.isArray(spl) && spl.length > 0) {
            requestData.spl = spl;
        } else if (spl && !Array.isArray(spl) && spl) {
            requestData.spl = [spl];
        }

        $.ajax({
            url: url,
            type: 'POST',
            contentType: 'application/json',
            data: JSON.stringify(requestData),
            success: function (res) {
                if (res.success && res.data) {
                    populateSelect($('#multicol-technician'), res.data, 'All Technician');
                } else {
                    populateSelect($('#multicol-technician'), [], 'All Technician');
                }
            },
            error: function () {
                populateSelect($('#multicol-technician'), [], 'All Technician');
            }
        });
    }

    // Initial load
    updateSPL("");
    updateTechnician("", "");
    updateCompany("");
    updateSLAStatus("");
    updateTaskType("");

    // On Data Date change, update SPL & Technician (reset both)
    $('#multicol-data-date').on('change', function () {
        const dataDate = $(this).val();
        const sac = $('#multicol-sac').val(); // Now returns array for multiple
        updateSPL(sac);
        updateTechnician(sac, null);
        updateCompany(dataDate);
        updateSLAStatus(dataDate);
        updateTaskType(dataDate);
        $('#multicol-spl').val(null).trigger('change');
    });

    // On SAC change, update SPL and Technician
    $('#multicol-sac').on('change', function () {
        const sac = $(this).val(); // Returns array for multiple
        updateSPL(sac);
        updateTechnician(sac, null);
        $('#multicol-spl').val(null).trigger('change');
    });
    // On SPL change, update Technician
    $('#multicol-spl').on('change', function () {
        const sac = $('#multicol-sac').val(); // Returns array
        const spl = $(this).val(); // Returns array
        updateTechnician(sac, spl);
    });
    // On Technician or Company change: no cascade needed

    // On Task Type change, handle Non-PM single selection logic
    $('#multicol-task-type').on('change', function () {
        const selected = $(this).val(); // Returns array
        const $select = $(this);
        
        if (selected && selected.includes('Non-PM')) {
            // If Non-PM is selected, remove all other options and keep only Non-PM
            if (selected.length > 1) {
                $select.val(['Non-PM']).trigger('change');
            }
            // Disable all options except Non-PM
            $select.find('option').each(function() {
                if ($(this).val() !== 'Non-PM') {
                    $(this).prop('disabled', true);
                }
            });
        } else {
            // Re-enable all options when Non-PM is not selected
            $select.find('option').prop('disabled', false);
        }
        
        // Update Select2 to reflect disabled state
        $select.trigger('change.select2');
    });
    
    // Prevent selecting disabled options in Task Type
    $('#multicol-task-type').on('select2:selecting', function(e) {
        const selected = $(this).val() || [];
        const newSelection = e.params.args.data.id;
        
        // If Non-PM is already selected and user tries to select another option
        if (selected.includes('Non-PM') && newSelection !== 'Non-PM') {
            e.preventDefault();
            // Show a brief visual feedback
            Swal.fire({
                icon: 'info',
                title: 'Selection Not Allowed',
                text: 'Non-PM option cannot be combined with other task types. Please deselect Non-PM first.',
                toast: true,
                position: 'top-end',
                showConfirmButton: false,
                timer: 3000,
                timerProgressBar: true
            });
        }
        
        // If trying to select Non-PM while other options are selected
        if (newSelection === 'Non-PM' && selected.length > 0 && !selected.includes('Non-PM')) {
            e.preventDefault();
            // Clear all and select only Non-PM
            $(this).val(['Non-PM']).trigger('change');
        }
    });

    // Initialize select2 with different configs for single vs multiple
    $('#multicol-data-date').select2({
        width: '100%',
        allowClear: true,
        placeholder: 'Current Month'
    });

    $('#multicol-sac, #multicol-spl, #multicol-technician, #multicol-company, #multicol-sla-status, #multicol-task-type').select2({
        width: '100%',
        allowClear: true,
        placeholder: 'Select one or more...',
        closeOnSelect: false,
        templateResult: function(option) {
            // Custom template for rendering options in dropdown
            if (!option.id) {
                return option.text;
            }
            
            // Check if option is disabled
            var $option = $(option.element);
            if ($option.is(':disabled')) {
                return $('<span class="disabled-option" style="opacity: 0.5; text-decoration: line-through; color: #999;">' + option.text + '</span>');
            }
            
            return option.text;
        }
    });

    // Initialize tooltips
    var tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
    var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });

    // Toggle chart description
    $('#toggle-chart-description').on('click', function () {
        const $btn = $(this);
        const $desc = $('#chart-description');
        const $icon = $btn.find('i');

        if ($desc.is(':visible')) {
            $desc.slideUp();
            $btn.html('<i class="fad fa-info-circle me-1"></i> Show Description');
        } else {
            $desc.slideDown();
            $btn.html('<i class="fad fa-info-circle me-1"></i> Hide Description');
        }
    });
});


function loadChartWithFilters(filters) {
    Swal.fire({
        title: 'Loading...',
        html: 'Fetching chart data, please wait.',
        allowOutsideClick: false,
        didOpen: () => {
            Swal.showLoading();
        }
    });

    // Line chart for Ticket Achievements
    $.ajax({
        url: '/odooms-monitoring/data-ticket-performance',
        type: 'POST',
        dataType: 'json',
        data: filters,
        timeout: 5 * 60 * 1000,
        success: function (response) {
            Swal.close();
            if (!response.success || !response.data || response.data.length === 0) {
                let errorMsg = 'No chart data available or error occurred!';
                if (response.error && response.error.length > 0) {
                    errorMsg = response.error;
                }
                Swal.fire({
                    icon: 'error',
                    title: 'Data Error',
                    text: errorMsg
                });
                return;
            }
            const Data = response.data;
            // Animation: PM & CM animate first (parallel), then Total Ticket, then Target, then Total Achievement
            // Animation time in ms
            const animDuration = 1400;
            const animGap = 1400;
            const chartSeries = [
                {
                    type: "line",
                    name: "Total PM (Accumulated)",
                    data: Data.map(item => [item.date, item.Total_PM]),
                    color: "#00b894", // bright teal
                    marker: { radius: 3, symbol: "circle" },
                },
                {
                    type: "line",
                    name: "Total PM (Per Day)",
                    data: Data.map(item => [item.date, item.Total_PM_Per_Day]),
                    color: "#00d2a0", // lighter teal
                    dashStyle: "Dot",
                    marker: { radius: 3, symbol: "circle" },
                    visible: false
                },
                {
                    type: "line",
                    name: "Total Non-PM (Accumulated)",
                    data: Data.map(item => [item.date, item.Total_Non_PM]),
                    color: "#e17055", // coral/salmon
                    marker: { radius: 3, symbol: "circle" },
                    animation: { defer: animGap, duration: animDuration }
                },
                {
                    type: "line",
                    name: "Total Non-PM (Per Day)",
                    data: Data.map(item => [item.date, item.Total_Non_PM_Per_Day]),
                    color: "#ff8c69", // lighter coral
                    dashStyle: "Dot",
                    marker: { radius: 3, symbol: "circle" },
                    animation: { defer: animGap, duration: animDuration },
                    visible: false
                },
                {
                    type: "line",
                    name: "Total Non-PM Overdue (Accumulated)",
                    data: Data.map(item => [item.date, item.Total_Non_PM_Overdue]),
                    color: "#d63031", // dark red
                    dashStyle: "ShortDash",
                    marker: { radius: 3, symbol: "triangle" },
                    animation: { defer: animGap, duration: animDuration },
                    visible: false
                },
                {
                    type: "line",
                    name: "Total Non-PM Overdue (Per Day)",
                    data: Data.map(item => [item.date, item.Total_Non_PM_Overdue_Per_Day]),
                    color: "#ff4757", // lighter red
                    dashStyle: "Dot",
                    marker: { radius: 3, symbol: "triangle" },
                    animation: { defer: animGap, duration: animDuration },
                    visible: false
                },
                {
                    type: "spline",
                    name: "Total Ticket (Accumulated)",
                    data: Data.map(item => [item.date, item.Total_Ticket]),
                    color: "#27ae60", // vivid green
                    dashStyle: "ShortDash",
                    marker: { radius: 4, symbol: "square" },
                    animation: { defer: animGap * 2, duration: animDuration }
                },
                {
                    type: "spline",
                    name: "Total Ticket (Per Day)",
                    data: Data.map(item => [item.date, item.Total_Ticket_Per_Day]),
                    color: "#2ecc71", // lighter green
                    dashStyle: "Dot",
                    marker: { radius: 4, symbol: "square" },
                    animation: { defer: animGap * 2, duration: animDuration },
                    visible: false
                },
                {
                    type: "spline",
                    name: "Target",
                    data: Data.map(item => [item.date, item.Target]),
                    color: "#e74c3c", // brave red
                    dashStyle: "Dash",
                    marker: { radius: 5, symbol: "triangle-down" },
                    animation: { defer: animGap * 3, duration: animDuration },
                    visible: false
                },
                {
                    type: "spline",
                    name: "Total Achievement (Accumulated)",
                    data: Data.map(item => [item.date, item.Total_Achievement]),
                    color: "#000000", // true black
                    dashStyle: "Solid",
                    marker: { radius: 5, symbol: "diamond" },
                    animation: { defer: animGap * 4, duration: animDuration },
                },
                {
                    type: "spline",
                    name: "Total Achievement / Day",
                    data: Data.map(item => [item.date, item.Total_Achievement_Per_Day]),
                    color: "#8e44ad", // purple
                    dashStyle: "Dot",
                    marker: { radius: 5, symbol: "diamond" },
                    animation: { defer: animGap * 5, duration: animDuration },
                    visible: false
                },
            ];
            Highcharts.chart(chartID, {
                chart: {
                    zooming: {
                        type: "x",
                    },
                    events: {
                        render: function () {
                            // Set line width for Target and Total Achievement series by correct index
                            this.series.forEach(function (s, idx) {
                                // Based on current series order (0-10):
                                // 0: Total PM (Accumulated)
                                // 1: Total PM (Per Day)
                                // 2: Total Non-PM (Accumulated)
                                // 3: Total Non-PM (Per Day)
                                // 4: Total Non-PM Overdue (Accumulated)
                                // 5: Total Non-PM Overdue (Per Day)
                                // 6: Total Ticket (Accumulated)
                                // 7: Total Ticket (Per Day)
                                // 8: Target
                                // 9: Total Achievement (Accumulated)
                                // 10: Total Achievement / Day
                                
                                if (idx === 8 && s.graph) { // Target
                                    if (s.graph.element) s.graph.element.style.strokeWidth = '3px';
                                }
                                if (idx === 9 && s.graph) { // Total Achievement (Accumulated)
                                    if (s.graph.element) s.graph.element.style.strokeWidth = '5px';
                                }
                                if (idx === 10 && s.graph) { // Total Achievement / Day
                                    if (s.graph.element) s.graph.element.style.strokeWidth = '4px';
                                }
                            });
                        }
                    }
                },
                // TODO: fix the exporting chart to image
                exporting: {
                    enabled: true,
                    url: '/odooms-monitoring/highcharts-export'
                },
                title: {
                    text: response.chart_title || "Ticket Performance (Achievements)",
                },
                subtitle: {
                    text: response.last_update ? `Last Update Data: ${response.last_update}` : null,
                },
                xAxis: {
                    type: "category",
                },
                yAxis: {
                    title: {
                        text: "Total JO",
                    },
                },
                legend: {
                    enabled: true,
                },
                credits: {
                    enabled: false,
                },
                tooltip: {
                    formatter: function () {
                        let desc = '';
                        switch (this.series.name) {
                            case 'Total PM (Accumulated)':
                                desc = "Cumulative count of Preventive Maintenance tickets (task_type = 'Preventive Maintenance') up to this date, based on received_spk_at."; 
                                break;
                            case 'Total PM (Per Day)':
                                desc = "Daily count of Preventive Maintenance tickets (task_type = 'Preventive Maintenance') on this date, based on received_spk_at."; 
                                break;
                            case 'Total Non-PM (Accumulated)':
                                desc = "Cumulative count of Non-PM tickets (task_type != 'Preventive Maintenance') up to this date, based on received_spk_at."; 
                                break;
                            case 'Total Non-PM (Per Day)':
                                desc = "Daily count of Non-PM tickets (task_type != 'Preventive Maintenance') on this date, based on received_spk_at."; 
                                break;
                            case 'Total Non-PM Overdue (Accumulated)':
                                desc = "Cumulative count of Non-PM tickets that are overdue (task_type != 'Preventive Maintenance' AND sla_status LIKE 'Overdue%') up to this date, based on received_spk_at."; 
                                break;
                            case 'Total Non-PM Overdue (Per Day)':
                                desc = "Daily count of Non-PM tickets that are overdue on this date, based on received_spk_at."; 
                                break;
                            case 'Total Ticket (Accumulated)':
                                desc = "Cumulative total tickets up to this date (all task_type), based on received_spk_at."; 
                                break;
                            case 'Total Ticket (Per Day)':
                                desc = "Daily count of all tickets on this date, based on received_spk_at."; 
                                break;
                            case 'Target':
                                desc = "Cumulative target: (last date's cumulative ticket count / num days, rounded up) for each day, based on received_spk_at."; 
                                break;
                            case 'Total Achievement (Accumulated)':
                                desc = "Cumulative count of all tickets completed up to this date (complete_wo, stage != 'Cancel')."; 
                                break;
                            case 'Total Achievement / Day':
                                desc = "Count of tickets completed on this date (complete_wo, stage != 'Cancel')."; 
                                break;
                        }
                        return '<b>' + this.series.name + '</b><br/>' +
                            'Date: ' + this.key + '<br/>' +
                            'Value: ' + this.y + (desc ? '<br/><i>' + desc + '</i>' : '');
                    }
                },
                series: chartSeries
            });
        },
        error: function (jqXHR, textStatus, errorThrown) {
            Swal.close();
            let errMsg = errorThrown;
            if (jqXHR.responseJSON && jqXHR.responseJSON.error) {
                errMsg = jqXHR.responseJSON.error;
            }
            Swal.fire({
                icon: 'error',
                title: jqXHR.status,
                html: `
                    <style>
                    .swal2-popup .swal2-title { margin-bottom: 0.2em !important; }
                    .swal2-popup .swal2-html-container { margin-top: 0.2em !important; }
                    </style>
                    <span class="text-danger">${jqXHR.statusText}</span> - <b>${errMsg}</b>
                `
            });
        }
    });

}

// On page load, load chart with default filters
$(document).ready(function () {
    loadChartWithFilters({});
    // On filter form submit
    $('#filter-form').on('submit', function (e) {
        e.preventDefault();
        // Get values - these will be arrays for multiple selects
        const filters = {
            data_date: $('#multicol-data-date').val(),
            sac: $('#multicol-sac').val(), // Array or null
            spl: $('#multicol-spl').val(), // Array or null
            technician: $('#multicol-technician').val(), // Array or null
            company: $('#multicol-company').val(), // Array or null
            sla_status: $('#multicol-sla-status').val(), // Array or null
            task_type: $('#multicol-task-type').val() // Array or null
        };

        // Convert arrays to comma-separated strings for backend
        if (Array.isArray(filters.sac)) filters.sac = filters.sac.join(',');
        if (Array.isArray(filters.spl)) filters.spl = filters.spl.join(',');
        if (Array.isArray(filters.technician)) filters.technician = filters.technician.join(',');
        if (Array.isArray(filters.company)) filters.company = filters.company.join(',');
        if (Array.isArray(filters.sla_status)) filters.sla_status = filters.sla_status.join(',');
        if (Array.isArray(filters.task_type)) filters.task_type = filters.task_type.join(',');

        loadChartWithFilters(filters);
    });
});

// Report (Master) download button
const btnReportID = 'report-dropdown-btn';

function handleReportClick(event, type) {
    event.stopPropagation();

    // Check if button is already disabled
    const $btn = $('#' + btnReportID);
    if ($btn.prop('disabled')) {
        return; // Exit early if already processing
    }

    // Close the dropdown immediately
    $btn.dropdown('hide');

    if (type === 'master') {
        downloadReportMasterTicketPerformance();
    } else {
        downloadReportFiltered();
    }
}

async function downloadReportMasterTicketPerformance() {
    const $btn = $('#' + btnReportID);
    const $icon = $btn.find('i');
    const originalIcon = $icon.attr('class');

    // Disable the entire dropdown button and prevent any clicks
    $btn.prop('disabled', true);
    $btn.addClass('disabled');
    $btn.attr('data-bs-toggle', ''); // Remove dropdown functionality temporarily
    $icon.attr('class', 'fad fa-spinner fa-spin me-2');

    // Create AbortController for timeout
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 5 * 60 * 1000); // 5 minutes timeout

    try {
        // Use fetch to handle file download with timeout
        const response = await fetch('/odooms-monitoring/download-master-ticket-performance', {
            signal: controller.signal
        });

        clearTimeout(timeoutId); // Clear timeout if request completes

        if (!response.ok) {
            throw new Error('No report file found for today');
        }
        const disposition = response.headers.get('Content-Disposition');
        let filename = 'report.xlsx';
        if (disposition && disposition.indexOf('filename=') !== -1) {
            filename = disposition.split('filename=')[1].replace(/"/g, '').trim();
        }
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        setTimeout(() => {
            window.URL.revokeObjectURL(url);
            document.body.removeChild(a);
        }, 100);
    } catch (err) {
        clearTimeout(timeoutId); // Clear timeout on error

        let errorMessage = 'Failed to download report.';
        if (err.name === 'AbortError') {
            errorMessage = 'Request timeout - The download took longer than 5 minutes. Please try again.';
        } else if (err.message) {
            errorMessage = err.message;
        }

        Swal.fire({
            icon: 'error',
            title: 'Download Error',
            html: errorMessage
        });
    } finally {
        // Re-enable the dropdown button
        $btn.prop('disabled', false);
        $btn.removeClass('disabled');
        $btn.attr('data-bs-toggle', 'dropdown'); // Restore dropdown functionality
        $icon.attr('class', originalIcon);
    }
}

async function downloadReportFiltered() {
    const $btn = $('#' + btnReportID);
    const $icon = $btn.find('i');
    const originalIcon = $icon.attr('class');

    // Disable the entire dropdown button and prevent any clicks
    $btn.prop('disabled', true);
    $btn.addClass('disabled');
    $btn.attr('data-bs-toggle', ''); // Remove dropdown functionality temporarily
    $icon.attr('class', 'fad fa-spinner fa-spin me-2');

    // Get current filter values
    const filters = {
        data_date: $('#multicol-data-date').val() || '',
        sac: $('#multicol-sac').val() || '',
        spl: $('#multicol-spl').val() || '',
        technician: $('#multicol-technician').val() || '',
        company: $('#multicol-company').val() || '',
        sla_status: $('#multicol-sla-status').val() || '',
        task_type: $('#multicol-task-type').val() || ''
    };

    // Convert arrays to comma-separated strings, handle null/empty values
    if (Array.isArray(filters.sac) && filters.sac.length > 0) {
        filters.sac = filters.sac.join(',');
    } else {
        filters.sac = '';
    }

    if (Array.isArray(filters.spl) && filters.spl.length > 0) {
        filters.spl = filters.spl.join(',');
    } else {
        filters.spl = '';
    }

    if (Array.isArray(filters.technician) && filters.technician.length > 0) {
        filters.technician = filters.technician.join(',');
    } else {
        filters.technician = '';
    }

    if (Array.isArray(filters.company) && filters.company.length > 0) {
        filters.company = filters.company.join(',');
    } else {
        filters.company = '';
    }

    if (Array.isArray(filters.sla_status) && filters.sla_status.length > 0) {
        filters.sla_status = filters.sla_status.join(',');
    } else {
        filters.sla_status = '';
    }

    if (Array.isArray(filters.task_type) && filters.task_type.length > 0) {
        filters.task_type = filters.task_type.join(',');
    } else {
        filters.task_type = '';
    }

    // Create AbortController for timeout
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 5 * 60 * 1000); // 5 minutes timeout

    try {
        // Use fetch to handle file download with timeout
        const response = await fetch('/odooms-monitoring/download-filtered-ticket-performance', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
            },
            body: new URLSearchParams(filters),
            signal: controller.signal
        });

        clearTimeout(timeoutId); // Clear timeout if request completes

        if (!response.ok) {
            // Try to get detailed error message from response body
            let errorDetails = `HTTP ${response.status}: ${response.statusText}`;
            try {
                const errorData = await response.json();
                if (errorData && errorData.error) {
                    errorDetails = `${errorDetails}<br><br>${errorData.error}`;
                }
            } catch (parseErr) {
                // If we can't parse JSON, just use the status
                console.warn('Could not parse error response as JSON:', parseErr);
            }
            throw new Error(errorDetails);
        }
        const disposition = response.headers.get('Content-Disposition');
        let filename = 'filtered_report.xlsx';
        if (disposition && disposition.indexOf('filename=') !== -1) {
            filename = disposition.split('filename=')[1].replace(/"/g, '').trim();
        }
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        setTimeout(() => {
            window.URL.revokeObjectURL(url);
            document.body.removeChild(a);
        }, 100);
    } catch (err) {
        clearTimeout(timeoutId); // Clear timeout on error

        let errorMessage = 'Failed to download filtered report.';
        if (err.name === 'AbortError') {
            errorMessage = 'Request timeout - The download took longer than 5 minutes. Please try again.';
        } else if (err.message) {
            errorMessage = err.message;
        }

        Swal.fire({
            icon: 'error',
            title: 'Download Error',
            html: errorMessage
        });
    } finally {
        // Re-enable the dropdown button
        $btn.prop('disabled', false);
        $btn.removeClass('disabled');
        $btn.attr('data-bs-toggle', 'dropdown'); // Restore dropdown functionality
        $icon.attr('class', originalIcon);
    }
}