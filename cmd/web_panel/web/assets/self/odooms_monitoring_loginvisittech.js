// webGlobalURL is now set from the HTML template as window.webGlobalURL
const chartID = "container";

(function () {
    var themeUrl = isDarkStyle
        ? `${webGlobalURL}assets/vendor/libs/highcharts-11.4.8/code/themes/high-contrast-dark.js`
        : `${webGlobalURL}assets/vendor/libs/highcharts-11.4.8/code/themes/high-contrast-light.js`;
    var s = document.createElement('script');
    s.src = themeUrl;
    document.head.appendChild(s);
    if (typeof jQuery !== 'undefined') {
        if (isDarkStyle) {
            $('#' + chartID).removeClass('highcharts-light').addClass('highcharts-dark');
        } else {
            $('#' + chartID).removeClass('highcharts-dark').addClass('highcharts-light');
        }
    }
})();

// --- Dynamic select2 population and cascading filter logic ---
$(document).ready(function () {
    // Helper to populate select2
    function populateSelect($select, data, allLabel) {
        $select.empty();
        $select.append(`<option value="">${allLabel}</option>`);
        data.forEach(function (item) {
            $select.append(`<option value="${item}">${item}</option>`);
        });
        $select.val("").trigger('change');
    }

    // Populate SAC
    $.get('/odooms-monitoring/data-sac-login-visit-technician', function (res) {
        if (res.success && res.data) {
            populateSelect($('#multicol-sac'), res.data, 'All SAC');
        }
    });

    // Populate SPL and Technician (initially all)
    function updateSPL(sac) {
        let url = '/odooms-monitoring/data-spl-login-visit-technician';
        if (sac) url += '?sac=' + encodeURIComponent(sac);
        $.get(url, function (res) {
            if (res.success && res.data) {
                populateSelect($('#multicol-spl'), res.data, 'All SPL');
            } else {
                populateSelect($('#multicol-spl'), [], 'All SPL');
            }
        });
    }
    function updateTechnician(sac, spl) {
        let url = '/odooms-monitoring/data-technician-login-visit-technician?';
        if (sac) url += 'sac=' + encodeURIComponent(sac) + '&';
        if (spl) url += 'spl=' + encodeURIComponent(spl);
        $.get(url, function (res) {
            if (res.success && res.data) {
                populateSelect($('#multicol-technician'), res.data, 'All Technician');
            } else {
                populateSelect($('#multicol-technician'), [], 'All Technician');
            }
        });
    }
    // Initial load
    updateSPL("");
    updateTechnician("", "");

    // On SAC change, update SPL and Technician
    $('#multicol-sac').on('change', function () {
        const sac = $(this).val();
        updateSPL(sac);
        updateTechnician(sac, "");
        $('#multicol-spl').val("").trigger('change');
    });
    // On SPL change, update Technician
    $('#multicol-spl').on('change', function () {
        const sac = $('#multicol-sac').val();
        const spl = $(this).val();
        updateTechnician(sac, spl);
    });
    // On Technician change: no cascade needed

    // Initialize select2
    $('#multicol-sac, #multicol-spl, #multicol-technician').select2({
        width: '100%',
        allowClear: true,
        placeholder: function () {
            return $(this).find('option:first').text();
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
    $.ajax({
        url: '/odooms-monitoring/data-login-visit-technician',
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
            const categories = Data.map(item => item.date.date);
            const animDuration = 1400;
            const animGap = 1400;
            const chartSeries = [
                {
                    type: "line",
                    name: "Active Technicians",
                    desc: "Count of technicians created on that date (created_at)",
                    data: Data.map(item => ({
                        y: item.Total_Active_Technicians,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#00b894",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 3, symbol: "circle" },
                },
                {
                    type: "line",
                    name: "Login Technicians",
                    desc: "Count of technicians with last_login on that date (last_login)",
                    data: Data.map(item => ({
                        y: item.Total_Login_Technicians,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#fdcb6e",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 3, symbol: "circle" },
                },
                {
                    type: "line",
                    name: "Visit Technicians",
                    desc: "Count of technicians with last_visit on that date (last_visit)",
                    data: Data.map(item => ({
                        y: item.Total_Visit_Technicians,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#27ae60",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 3, symbol: "circle" },
                },
                {
                    type: "spline",
                    name: "SP1 Given",
                    desc: "Count of SP1 given (is_got_sp1 = true) on that date from Tech/SPL tables (created_at)",
                    data: Data.map(item => ({
                        y: item.Total_SP1_Given,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#e74c3c",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 4, symbol: "square" },
                    animation: { defer: animGap, duration: animDuration }
                },
                {
                    type: "spline",
                    name: "SP2 Given",
                    desc: "Count of SP2 given (is_got_sp2 = true) on that date from Tech/SPL tables (created_at)",
                    data: Data.map(item => ({
                        y: item.Total_SP2_Given,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#8e44ad",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 4, symbol: "diamond" },
                    animation: { defer: animGap * 2, duration: animDuration }
                },
                {
                    type: "spline",
                    name: "SP3 Given",
                    desc: "Count of SP3 given (is_got_sp3 = true) on that date from Tech/SPL tables (created_at)",
                    data: Data.map(item => ({
                        y: item.Total_SP3_Given,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#16a085",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 4, symbol: "triangle" },
                    animation: { defer: animGap * 3, duration: animDuration }
                },
                {
                    type: "spline",
                    name: "SP1 Replied",
                    desc: "Count of SP1 replied (what_sp = SP_TECHNICIAN or SP_SPL, number_of_sp = 1) on that date from SPWhatsappMsg table (created_at)",
                    data: Data.map(item => ({
                        y: item.Total_SP1_Replied,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#fdcb6e",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 4, symbol: "circle" },
                    animation: { defer: animGap * 4, duration: animDuration }
                },
                {
                    type: "spline",
                    name: "SP2 Replied",
                    desc: "Count of SP2 replied (what_sp = SP_TECHNICIAN or SP_SPL, number_of_sp = 2) on that date from SPWhatsappMsg table (created_at)",
                    data: Data.map(item => ({
                        y: item.Total_SP2_Replied,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#e67e22",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 4, symbol: "diamond" },
                    animation: { defer: animGap * 5, duration: animDuration }
                },
                {
                    type: "spline",
                    name: "SP3 Replied",
                    desc: "Count of SP3 replied (what_sp = SP_TECHNICIAN or SP_SPL, number_of_sp = 3) on that date from SPWhatsappMsg table (created_at)",
                    data: Data.map(item => ({
                        y: item.Total_SP3_Replied,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#000000",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 4, symbol: "triangle" },
                    animation: { defer: animGap * 6, duration: animDuration }
                },
                {
                    type: "spline",
                    name: "Cumulative Visit",
                    desc: "Cumulative sum of Total Visit Technicians up to that date.",
                    data: Data.map(item => ({
                        y: item.Cumulative_Visit,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#27ae60",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 5, symbol: "circle" },
                    animation: { defer: animGap * 7, duration: animDuration }
                },
                {
                    type: "spline",
                    name: "Cumulative SP Given",
                    desc: "Cumulative sum of SP1/2/3 given up to that date.",
                    data: Data.map(item => ({
                        y: item.Cumulative_SPGiven,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#e74c3c",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 5, symbol: "diamond" },
                    animation: { defer: animGap * 8, duration: animDuration }
                },
                {
                    type: "spline",
                    name: "Cumulative SP Replied",
                    desc: "Cumulative sum of SP1/2/3 replied up to that date.",
                    data: Data.map(item => ({
                        y: item.Cumulative_SPReplied,
                        color: (item.date.is_weekends || item.date.is_holiday) ? "red" : "#8e44ad",
                        holiday: item.date.holiday,
                        isWeekend: item.date.is_weekends
                    })),
                    marker: { radius: 5, symbol: "triangle" },
                    animation: { defer: animGap * 9, duration: animDuration }
                }
            ];
            Highcharts.chart(chartID, {
                chart: {
                    zooming: {
                        type: "x",
                    },
                    events: {
                        render: function () {
                            this.series.forEach(function (s, idx) {
                                if (s.graph && s.graph.element) {
                                    if (s.name.includes("Cumulative")) {
                                        s.graph.element.style.strokeWidth = '4px';
                                    }
                                }
                            });
                        }
                    }
                },
                exporting: {
                    enabled: true,
                },
                title: {
                    text: response.chart_title || "Technician Login/Visit Performance",
                },
                subtitle: {
                    text: response.last_update ? `Last Update Data: ${response.last_update}` : null,
                },
                xAxis: {
                    categories: categories,
                    labels: {
                        formatter: function () {
                            const item = Data[this.pos].date;
                            if (item.is_weekends || item.is_holiday) {
                                return `<span style='color:red'>${this.value}</span>`;
                            }
                            return this.value;
                        }
                    }
                },
                yAxis: {
                    title: {
                        text: "Total Count",
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
                        let holidayInfo = this.point.holiday && this.point.holiday.length > 0
                            ? `<br/><b>Holiday:</b> ${this.point.holiday.join(', ')}`
                            : '';
                        let weekendInfo = this.point.isWeekend ? "<br/><b>Weekend</b>" : "";
                        let desc = '';
                        // Find the series object by name
                        const seriesObj = chartSeries.find(s => s.name === this.series.name);
                        if (seriesObj && seriesObj.desc) {
                            desc = `<br/><i>${seriesObj.desc}</i>`;
                        }
                        return `<b>${this.series.name}</b><br/>Date: ${this.x}<br/>Value: ${this.y}${holidayInfo}${weekendInfo}${desc}`;
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
        const filters = {
            sac: $('#multicol-sac').val(),
            spl: $('#multicol-spl').val(),
            technician: $('#multicol-technician').val(),
        };
        loadChartWithFilters(filters);
    });
});

// Report (Master) download button
const btnReportID = 'report-login-visit-technician';
async function downloadReportMasterLoginVisitTechnician() {
    const $btn = $('#' + btnReportID);
    const $icon = $btn.find('i');
    const originalIcon = $icon.attr('class');
    $btn.prop('disabled', true);
    $icon.attr('class', 'fad fa-spinner fa-spin me-2');

    try {
        // Use fetch to handle file download
        const response = await fetch('/odooms-monitoring/download-master-login-visit-technician');
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
        Swal.fire({
            icon: 'error',
            title: 'Download Error',
            text: err.message || 'Failed to download report.'
        });
    } finally {
        $btn.prop('disabled', false);
        $icon.attr('class', originalIcon);
    }
}