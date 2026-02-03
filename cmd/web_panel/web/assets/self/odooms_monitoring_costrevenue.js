// webGlobalURL is now set from the HTML template as window.webGlobalURL
const chartRevenueCostID = "container-revenue-cost";

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
                $('#' + chartRevenueCostID).removeClass('highcharts-light').addClass('highcharts-dark');
            } else {
                $('#' + chartRevenueCostID).removeClass('highcharts-dark').addClass('highcharts-light');
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

    // On Data Date change, update SPL & Technician (reset both)
    $('#multicol-data-date').on('change', function () {
        const dataDate = $(this).val();
        const sac = $('#multicol-sac').val(); // Now returns array for multiple
        updateSPL(sac);
        updateTechnician(sac, null);
        updateCompany(dataDate);
        updateSLAStatus(dataDate);
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

    // Initialize select2 with different configs for single vs multiple
    $('#multicol-data-date').select2({
        width: '100%',
        allowClear: true,
        placeholder: 'Current Month'
    });

    $('#multicol-sac, #multicol-spl, #multicol-technician, #multicol-company, #multicol-sla-status').select2({
        width: '100%',
        allowClear: true,
        placeholder: 'Select one or more...',
        closeOnSelect: false
    });

    // Initialize tooltips
    var tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
    var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
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

    // Cost vs Revenue line/column chart with daily data
    $.ajax({
        url: '/odooms-monitoring/data-ticket-performance-revenue-cost',
        type: 'POST',
        data: filters,
        success: function (response) {
            Swal.close();
            
            if (response.success && response.data) {
                const Data = response.data;
                const drillData = response.drill_data || {};

                // Animation settings
                const animDuration = 1400;
                const animGap = 1400;

                const chartSeries = [
                    {
                        type: "column",
                        name: "Daily Revenue (Count)",
                        data: Data.map(item => [item.date, item.Daily_Revenue]),
                        color: "#2ecc71", // green
                        yAxis: 0,
                        animation: { duration: animDuration },
                        cursor: 'pointer',
                        point: {
                            events: {
                                click: function() {
                                    if (drillData.revenue_drill) {
                                        const clickedDate = this.name || this.series.xAxis.categories[this.x];
                                        console.log('Revenue clicked - this.name:', this.name, 'index:', this.x, 'categories:', this.series.xAxis.categories);
                                        doDrill('revenue_drill', `Revenue Breakdown - ${clickedDate}`, clickedDate);
                                    }
                                }
                            }
                        }
                    },
                    {
                        type: "column",
                        name: "Daily Cost to Technicians (Count)",
                        data: Data.map(item => [item.date, item.Daily_Cost_To_Technicians]),
                        color: "#e67e22", // orange
                        yAxis: 0,
                        animation: { duration: animDuration },
                        cursor: 'pointer',
                        point: {
                            events: {
                                click: function() {
                                    if (drillData.cost_to_tech_drill) {
                                        const clickedDate = this.name || this.series.xAxis.categories[this.x];
                                        console.log('Cost to Tech clicked - this.name:', this.name, 'index:', this.x);
                                        doDrill('cost_to_tech_drill', `Cost to Technicians - ${clickedDate}`, clickedDate);
                                    }
                                }
                            }
                        }
                    },
                    {
                        type: "column",
                        name: "Daily Cost of Penalty (Count)",
                        data: Data.map(item => [item.date, item.Daily_Cost_Of_Penalty]),
                        color: "#e74c3c", // red
                        yAxis: 0,
                        animation: { duration: animDuration },
                        cursor: 'pointer',
                        point: {
                            events: {
                                click: function() {
                                    if (drillData.cost_penalty_drill) {
                                        const clickedDate = this.name || this.series.xAxis.categories[this.x];
                                        console.log('Cost Penalty clicked - this.name:', this.name, 'index:', this.x);
                                        doDrill('cost_penalty_drill', `Cost of Penalty - ${clickedDate}`, clickedDate);
                                    }
                                }
                            }
                        }
                    },
                    {
                        type: "spline",
                        name: "Accumulated Revenue (Count)",
                        data: Data.map(item => [item.date, item.Accumulated_Revenue]),
                        color: "#27ae60", // dark green
                        dashStyle: "Solid",
                        marker: { radius: 4, symbol: "circle" },
                        yAxis: 0,
                        animation: { defer: animGap, duration: animDuration }
                    },
                    {
                        type: "spline",
                        name: "Accumulated Cost to Technicians (Count)",
                        data: Data.map(item => [item.date, item.Accumulated_Cost_To_Technicians]),
                        color: "#d35400", // dark orange
                        dashStyle: "Solid",
                        marker: { radius: 4, symbol: "square" },
                        yAxis: 0,
                        animation: { defer: animGap * 2, duration: animDuration }
                    },
                    {
                        type: "spline",
                        name: "Accumulated Cost of Penalty (Count)",
                        data: Data.map(item => [item.date, item.Accumulated_Cost_Of_Penalty]),
                        color: "#c0392b", // dark red
                        dashStyle: "Solid",
                        marker: { radius: 4, symbol: "triangle" },
                        yAxis: 0,
                        animation: { defer: animGap * 3, duration: animDuration }
                    },
                    {
                        type: "spline",
                        name: "Accumulated Profit (Count)",
                        data: Data.map(item => [item.date, item.Accumulated_Profit]),
                        color: "#3498db", // blue
                        dashStyle: "ShortDash",
                        marker: { radius: 5, symbol: "diamond" },
                        yAxis: 0,
                        animation: { defer: animGap * 4, duration: animDuration }
                    },
                    {
                        type: "spline",
                        name: "Accumulated Revenue (Price)",
                        data: Data.map(item => [item.date, item.Revenue_Price]),
                        color: "#16a085", // teal
                        dashStyle: "Dot",
                        marker: { radius: 3, symbol: "circle" },
                        yAxis: 1,
                        visible: false,
                        animation: { defer: animGap * 5, duration: animDuration }
                    },
                    {
                        type: "spline",
                        name: "Accumulated Cost to Technicians (Price)",
                        data: Data.map(item => [item.date, item.Cost_To_Technicians_Price]),
                        color: "#f39c12", // yellow-orange
                        dashStyle: "Dot",
                        marker: { radius: 3, symbol: "square" },
                        yAxis: 1,
                        visible: false,
                        animation: { defer: animGap * 6, duration: animDuration }
                    },
                    {
                        type: "spline",
                        name: "Accumulated Cost of Penalty (Price)",
                        data: Data.map(item => [item.date, item.Cost_Of_Penalty_Price]),
                        color: "#e74c3c", // red
                        dashStyle: "Dot",
                        marker: { radius: 3, symbol: "triangle" },
                        yAxis: 1,
                        visible: false,
                        animation: { defer: animGap * 7, duration: animDuration }
                    },
                    {
                        type: "spline",
                        name: "Accumulated Profit (Price)",
                        data: Data.map(item => [item.date, item.Profit_Price]),
                        color: "#9b59b6", // purple
                        dashStyle: "ShortDot",
                        marker: { radius: 4, symbol: "diamond" },
                        yAxis: 1,
                        visible: false,
                        animation: { defer: animGap * 8, duration: animDuration }
                    }
                ];

                const chart = Highcharts.chart(chartRevenueCostID, {
                    chart: {
                        zooming: {
                            type: "x",
                        },
                        events: {
                            render: function () {
                                // Set line width for key series
                                this.series.forEach(function (s, idx) {
                                    if (idx === 6 && s.graph) { // Accumulated Profit (Count)
                                        if (s.graph.element) s.graph.element.style.strokeWidth = '5px';
                                    }
                                });
                            }
                        }
                    },
                    title: {
                        text: response.chart_title || 'COST vs REVENUE',
                        align: "left",
                    },
                    subtitle: {
                        text: `Last Updated: ${response.last_update || ''}`,
                        align: "left",
                    },
                    accessibility: {
                        point: {
                            valueDescriptionFormat: "{index}. {xDescription}, {value}.",
                        },
                    },
                    xAxis: {
                        type: "category",
                        categories: Data.map(item => item.date),
                        accessibility: {
                            description: "Date",
                        },
                    },
                    yAxis: [
                        {
                            title: {
                                text: "Ticket Count",
                            },
                            labels: {
                                format: "{value}",
                            },
                        },
                        {
                            title: {
                                text: "Price (Rp)",
                            },
                            labels: {
                                format: "Rp {value:,.0f}",
                            },
                            opposite: true,
                        },
                    ],
                    tooltip: {
                        shared: false,
                        formatter: function () {
                            const seriesName = this.series.name;
                            const xValue = this.x;
                            const yValue = this.y;
                            const pointIndex = this.point.index;
                            const dataItem = Data[pointIndex];
                            
                            let tooltipHTML = '<b>' + xValue + '</b><br/>';
                            tooltipHTML += '<span style="color:' + this.color + '">\u25CF</span> ' + seriesName + ':<br/>';
                            
                            // Show count and price based on series name
                            if (seriesName.includes('Daily Revenue')) {
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;<b>' + Highcharts.numberFormat(yValue, 0, '.', ',') + ' tickets</b><br/>';
                                const price = dataItem.Daily_Revenue * (dataItem.Revenue_Price / dataItem.Accumulated_Revenue);
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;Price: <b>Rp ' + Highcharts.numberFormat(price, 0, '.', ',') + '</b>';
                            } 
                            else if (seriesName.includes('Daily Cost to Technicians')) {
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;<b>' + Highcharts.numberFormat(yValue, 0, '.', ',') + ' tickets</b><br/>';
                                const price = dataItem.Daily_Cost_To_Technicians * (dataItem.Cost_To_Technicians_Price / dataItem.Accumulated_Cost_To_Technicians);
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;Price: <b>Rp ' + Highcharts.numberFormat(price, 0, '.', ',') + '</b>';
                            }
                            else if (seriesName.includes('Daily Cost of Penalty')) {
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;<b>' + Highcharts.numberFormat(yValue, 0, '.', ',') + ' tickets</b><br/>';
                                const price = dataItem.Daily_Cost_Of_Penalty * (dataItem.Cost_Of_Penalty_Price / dataItem.Accumulated_Cost_Of_Penalty);
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;Price: <b>Rp ' + Highcharts.numberFormat(price, 0, '.', ',') + '</b>';
                            }
                            else if (seriesName.includes('Accumulated Revenue (Count)')) {
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;<b>' + Highcharts.numberFormat(yValue, 0, '.', ',') + ' tickets</b><br/>';
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;Total Price: <b>Rp ' + Highcharts.numberFormat(dataItem.Revenue_Price, 0, '.', ',') + '</b>';
                            }
                            else if (seriesName.includes('Accumulated Cost to Technicians (Count)')) {
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;<b>' + Highcharts.numberFormat(yValue, 0, '.', ',') + ' tickets</b><br/>';
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;Total Price: <b>Rp ' + Highcharts.numberFormat(dataItem.Cost_To_Technicians_Price, 0, '.', ',') + '</b>';
                            }
                            else if (seriesName.includes('Accumulated Cost of Penalty (Count)')) {
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;<b>' + Highcharts.numberFormat(yValue, 0, '.', ',') + ' tickets</b><br/>';
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;Total Price: <b>Rp ' + Highcharts.numberFormat(dataItem.Cost_Of_Penalty_Price, 0, '.', ',') + '</b>';
                            }
                            else if (seriesName.includes('Accumulated Profit (Count)')) {
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;<b>' + Highcharts.numberFormat(yValue, 0, '.', ',') + ' tickets</b><br/>';
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;Total Price: <b>Rp ' + Highcharts.numberFormat(dataItem.Profit_Price, 0, '.', ',') + '</b>';
                            }
                            else if (seriesName.includes('(Price)')) {
                                // Price series - just show the price
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;<b>Rp ' + Highcharts.numberFormat(yValue, 0, '.', ',') + '</b>';
                            }
                            else {
                                // Default for any other series
                                tooltipHTML += '&nbsp;&nbsp;&nbsp;<b>' + Highcharts.numberFormat(yValue, 0, '.', ',') + '</b>';
                            }
                            
                            return tooltipHTML;
                        }
                    },
                    legend: {
                        enabled: true,
                        layout: "horizontal",
                        align: "center",
                        verticalAlign: "bottom",
                    },
                    plotOptions: {
                        series: {
                            label: {
                                connectorAllowed: false,
                            },
                            marker: {
                                enabled: true,
                            },
                        },
                    },
                    series: chartSeries,
                    responsive: {
                        rules: [
                            {
                                condition: {
                                    maxWidth: 500,
                                },
                                chartOptions: {
                                    legend: {
                                        layout: "horizontal",
                                        align: "center",
                                        verticalAlign: "bottom",
                                    },
                                },
                            },
                        ],
                    },
                });

                // Save the original state for drill-down restoration
                const savedState = {
                    title: response.chart_title || 'COST vs REVENUE',
                    subtitle: `Last Updated: ${response.last_update || ''}`,
                    categories: Data.map(item => item.date),
                    series: chartSeries.map(s => ({
                        type: s.type,
                        name: s.name,
                        data: s.data,
                        color: s.color,
                        dashStyle: s.dashStyle,
                        marker: s.marker,
                        yAxis: s.yAxis,
                        visible: s.visible !== false,
                        animation: s.animation
                    }))
                };

                // Drill-down logic
                function doDrill(drillKey, newTitle, clickedDate) {
                    const detail = drillData[drillKey];
                    if (!detail || !Array.isArray(detail)) return;

                    console.log('Clicked Date:', clickedDate);
                    console.log('Drill Data:', detail);
                    console.log('Sample drill item dates:', detail.slice(0, 3).map(d => d.date));

                    // Filter drill data by clicked date
                    const filteredDetail = detail.filter(item => item.date === clickedDate);
                    
                    console.log('Filtered Detail:', filteredDetail);
                    
                    if (filteredDetail.length === 0) {
                        Swal.fire({
                            icon: 'info',
                            title: 'No Data',
                            text: `No breakdown data available for ${clickedDate}. Available dates: ${[...new Set(detail.map(d => d.date))].join(', ')}`,
                            timer: 5000
                        });
                        return;
                    }

                    // Prepare drill data
                    const drillItems = filteredDetail.map(item => ({
                        name: item.name,
                        y: item.y,
                        count: item.count,
                        price: item.price,
                        description: item.description
                    }));

                    // Remove all existing series
                    while (chart.series.length > 0) {
                        chart.series[0].remove(false);
                    }

                    // Determine color based on drill type
                    let drillColor = '#2ecc71'; // default green
                    if (drillKey === 'cost_to_tech_drill') {
                        drillColor = '#e67e22'; // orange
                    } else if (drillKey === 'cost_penalty_drill') {
                        drillColor = '#e74c3c'; // red
                    }

                    // Add drill-down column series
                    chart.addSeries({
                        type: 'column',
                        name: newTitle,
                        data: drillItems,
                        color: drillColor,
                        dataLabels: {
                            enabled: true,
                            formatter: function () {
                                return this.point.count + ' tickets<br/>' +
                                    'Rp ' + Highcharts.numberFormat(this.point.price, 0, '.', ',') + ' each<br/>' +
                                    'Total: Rp ' + Highcharts.numberFormat(this.y, 0, '.', ',');
                            },
                            useHTML: true,
                            style: {
                                fontSize: '9px',
                                fontWeight: 'bold'
                            }
                        },
                        tooltip: {
                            pointFormatter: function () {
                                const totalDrillTickets = drillItems.reduce((sum, item) => sum + item.count, 0);
                                const isRevenue = drillKey === 'revenue_drill';

                                let tooltipContent = '<span style="color:' + this.color + '">\u25CF</span> ' +
                                    this.series.name + ': <b>Rp ' + Highcharts.numberFormat(this.y, 0, '.', ',') + '</b><br/>' +
                                    'Tickets: <b>' + this.count + '</b> (of ' + totalDrillTickets + ' total)<br/>';

                                if (isRevenue) {
                                    tooltipContent += 'List Price: <b>Rp ' + Highcharts.numberFormat(this.price, 0, '.', ',') + '</b><br/>' +
                                        'Revenue Calculation: ' + this.count + ' tickets × Rp ' + Highcharts.numberFormat(this.price, 0, '.', ',') + '<br/>';
                                } else {
                                    tooltipContent += 'Cost per ticket: <b>Rp ' + Highcharts.numberFormat(this.price, 0, '.', ',') + '</b><br/>' +
                                        'Cost Calculation: ' + this.count + ' tickets × Rp ' + Highcharts.numberFormat(this.price, 0, '.', ',') + '<br/>';
                                }

                                tooltipContent += '<hr style="margin: 5px 0;"/>' +
                                    '<small>' + this.description + '</small>';

                                return tooltipContent;
                            }
                        }
                    }, false);

                    // Update xAxis categories to show company-task combinations
                    chart.xAxis[0].setCategories(drillItems.map(d => d.name), false);

                    const totalDrillTickets = drillItems.reduce((sum, item) => sum + item.count, 0);
                    const totalDrillValue = drillItems.reduce((sum, item) => sum + item.y, 0);

                    chart.setTitle({ text: newTitle });
                    chart.setTitle(null, {
                        text: `Total: ${totalDrillTickets} tickets | Total Value: Rp ${Highcharts.numberFormat(totalDrillValue, 0, '.', ',')}`,
                        useHTML: true
                    });
                    
                    chart.redraw();
                    addBackButton();
                }

                // Restore main view
                function restoreMain() {
                    // Remove all existing series
                    while (chart.series.length > 0) {
                        chart.series[0].remove(false);
                    }

                    // Restore original categories
                    chart.xAxis[0].setCategories(savedState.categories, false);

                    // Restore all original series with their point click events
                    savedState.series.forEach((s, idx) => {
                        const seriesConfig = {
                            type: s.type,
                            name: s.name,
                            data: s.data,
                            color: s.color,
                            dashStyle: s.dashStyle,
                            marker: s.marker,
                            yAxis: s.yAxis,
                            visible: s.visible,
                            animation: false, // Disable animation on restore for smoother experience
                        };

                        // Add click events back to the daily column series
                        if (idx === 0 || idx === 1 || idx === 2) { // Daily Revenue, Daily Cost to Tech, Daily Cost Penalty
                            seriesConfig.cursor = 'pointer';
                            seriesConfig.point = {
                                events: {
                                    click: function() {
                                        const clickedDate = this.name || this.series.xAxis.categories[this.x];
                                        let drillKey = '';
                                        let title = '';
                                        
                                        if (idx === 0 && drillData.revenue_drill) {
                                            drillKey = 'revenue_drill';
                                            title = `Revenue Breakdown - ${clickedDate}`;
                                        } else if (idx === 1 && drillData.cost_to_tech_drill) {
                                            drillKey = 'cost_to_tech_drill';
                                            title = `Cost to Technicians - ${clickedDate}`;
                                        } else if (idx === 2 && drillData.cost_penalty_drill) {
                                            drillKey = 'cost_penalty_drill';
                                            title = `Cost of Penalty - ${clickedDate}`;
                                        }
                                        
                                        if (drillKey) {
                                            doDrill(drillKey, title, clickedDate);
                                        }
                                    }
                                }
                            };
                        }

                        chart.addSeries(seriesConfig, false);
                    });

                    // Reset xAxis to category type for dates
                    chart.xAxis[0].update({
                        type: 'category'
                    }, false);

                    chart.setTitle({ text: savedState.title });
                    chart.setTitle(null, { text: savedState.subtitle, useHTML: true });
                    chart.redraw();

                    if (chart.customBackButton) {
                        chart.customBackButton.destroy();
                        chart.customBackButton = null;
                    }
                }

                // Add back button for drill-down navigation
                function addBackButton() {
                    if (chart.customBackButton) return;
                    chart.customBackButton = chart.renderer
                        .button('◀ Back', chart.plotLeft + 10, 10, function () {
                            restoreMain();
                        })
                        .attr({ zIndex: 5 })
                        .add();
                }

            } else {
                Swal.fire({
                    icon: 'error',
                    title: 'No Data',
                    text: response.error || 'No chart data available.'
                });
            }
        },
        error: function (jqXHR, textStatus, errorThrown) {
            console.error('AJAX Error loading cost-revenue chart:', textStatus, errorThrown);
            Swal.fire({
                icon: 'error',
                title: 'Error loading Revenue vs Cost chart',
                html: `
                    <style>
                    .swal2-popup .swal2-title { margin-bottom: 0.2em !important; }
                    .swal2-popup .swal2-html-container { margin-top: 0.2em !important; }
                    </style>
                    <span class="text-danger">${jqXHR.statusText}</span> - <b>${textStatus}</b>
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
            sla_status: $('#multicol-sla-status').val() // Array or null
        };

        // Convert arrays to comma-separated strings for backend
        if (Array.isArray(filters.sac)) filters.sac = filters.sac.join(',');
        if (Array.isArray(filters.spl)) filters.spl = filters.spl.join(',');
        if (Array.isArray(filters.technician)) filters.technician = filters.technician.join(',');
        if (Array.isArray(filters.company)) filters.company = filters.company.join(',');
        if (Array.isArray(filters.sla_status)) filters.sla_status = filters.sla_status.join(',');

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
        sla_status: $('#multicol-sla-status').val() || ''
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