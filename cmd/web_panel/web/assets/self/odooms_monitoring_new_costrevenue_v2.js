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
            $('#' + chartRevenueCostID).addClass(isDark ? 'highcharts-dark' : 'highcharts-light');
        }
    };
    document.head.appendChild(s);
})();

// Load pattern fill module for Highcharts
Highcharts.setOptions({
    colors: ['#058DC7', '#50B432', '#ED561B', '#DDDF00', '#24CBE5', '#64E572', '#FF9655', '#FFF263', '#6AF9C4']
});

// Define custom patterns for better visual distinction
Highcharts.setOptions({
    defs: {
        hatchDiagonal: {
            tagName: 'pattern',
            id: 'hatch-diagonal',
            patternUnits: 'userSpaceOnUse',
            width: 8,
            height: 8,
            children: [{
                tagName: 'path',
                d: 'M-1,1 l2,-2 M0,8 l8,-8 M7,9 l2,-2',
                stroke: '#dc3545',
                strokeWidth: 2
            }]
        },
        hatchVertical: {
            tagName: 'pattern',
            id: 'hatch-vertical',
            patternUnits: 'userSpaceOnUse',
            width: 6,
            height: 6,
            children: [{
                tagName: 'path',
                d: 'M 2 0 L 2 6',
                stroke: '#6f42c1',
                strokeWidth: 2
            }]
        },
        hatchCross: {
            tagName: 'pattern',
            id: 'hatch-cross',
            patternUnits: 'userSpaceOnUse',
            width: 8,
            height: 8,
            children: [{
                tagName: 'path',
                d: 'M 0 4 L 8 4 M 4 0 L 4 8',
                stroke: '#17a2b8',
                strokeWidth: 1.5
            }]
        }
    }
});

$(document).ready(function () {
    // Initialize select2
    $('#select-year').select2({
        width: '100%',
        placeholder: 'Select Year'
    });
    
    $('#select-months').select2({
        width: '100%',
        placeholder: 'Select Months',
        closeOnSelect: false
    });
    
    $('#select-company').select2({
        width: '100%',
        placeholder: 'Select Companies',
        closeOnSelect: false
    });
    
    // Load available years
    loadAvailableYears();
    
    // Load chart on page load
    loadYearlyChart();
    
    // Year change handler
    $('#select-year').on('change', function() {
        const selectedYear = $(this).val();
        loadAvailableMonths(selectedYear);
        loadAvailableCompanies(selectedYear);
    });
    
    // Apply filter button
    $('#btn-apply-filter').on('click', function() {
        loadYearlyChart();
    });
});

function loadAvailableYears() {
    $.ajax({
        url: '/odooms-monitoring/available-years-cost-revenue',
        type: 'GET',
        success: function(response) {
            if (response.success && response.data) {
                const $select = $('#select-year');
                $select.empty();
                
                response.data.forEach(function(year) {
                    $select.append(new Option(year, year, false, false));
                });
                
                // Select current year by default
                const currentYear = new Date().getFullYear();
                $select.val(currentYear).trigger('change');
            }
        }
    });
}

function loadAvailableMonths(year) {
    if (!year) year = new Date().getFullYear();
    
    $.ajax({
        url: '/odooms-monitoring/available-months-cost-revenue?year=' + year,
        type: 'GET',
        success: function(response) {
            if (response.success && response.data) {
                const $select = $('#select-months');
                $select.empty();
                
                response.data.forEach(function(month) {
                    $select.append(new Option(month.name, month.value, false, false));
                });
                
                $select.val(null).trigger('change');
            }
        }
    });
}

function loadAvailableCompanies(year) {
    if (!year) year = new Date().getFullYear();
    
    $.ajax({
        url: '/odooms-monitoring/available-companies-cost-revenue?year=' + year,
        type: 'GET',
        success: function(response) {
            if (response.success && response.data) {
                const $select = $('#select-company');
                $select.empty();
                
                response.data.forEach(function(company) {
                    $select.append(new Option(company, company, false, false));
                });
                
                $select.val(null).trigger('change');
            }
        }
    });
}

function loadYearlyChart() {
    Swal.fire({
        title: 'Loading...',
        html: 'Fetching chart data, please wait.',
        allowOutsideClick: false,
        didOpen: () => {
            Swal.showLoading();
        }
    });

    // Get filter values
    const selectedYear = $('#select-year').val() || new Date().getFullYear();
    const selectedMonths = $('#select-months').val() || [];
    const selectedCompanies = $('#select-company').val() || [];
    
    // Build query parameters
    let queryParams = 'year=' + selectedYear;
    if (selectedMonths.length > 0) {
        queryParams += '&months=' + selectedMonths.join(',');
    }
    if (selectedCompanies.length > 0) {
        queryParams += '&companies=' + selectedCompanies.join(',');
    }

    // Yearly stacked bar chart for Cost vs Revenue
    $.ajax({
        url: '/odooms-monitoring/data-new-cost-revenue-yearly?' + queryParams,
        type: 'GET',
        success: function (response) {
            Swal.close();
            
            if (response.success && response.data) {
                const data = response.data;
                
                // Prepare data for single stacked bar per month
                const months = data.map(d => d.month);
                
                // Stacked bar data - building from bottom to top
                // Stack order: Payroll -> Profit -> BA Lost -> Over SLA -> Cost of Penalties
                // Revenue is the TOTAL of all these components, not a separate segment
                const payrollData = data.map(d => ({ 
                    y: d.payroll,
                    count: d.payroll_ticket_count
                }));
                const profitData = data.map(d => ({ 
                    y: d.profit,
                    count: d.revenue_ticket_count
                }));
                const baLostData = data.map(d => ({
                    y: d.ba_lost,
                    count: d.ba_lost_ticket_count
                }));
                const overSLAData = data.map(d => ({
                    y: d.over_sla,
                    count: d.over_sla_ticket_count
                }));
                const penaltiesData = data.map(d => ({
                    y: d.cost_penalties,
                    count: d.cost_penalties_count
                }));
                const revenueData = data.map(d => d.revenue); // For reference/label only
                const grossProfitData = data.map(d => d.gross_profit);

                // Define patterns using Highcharts pattern fill
                // BA Lost - red/danger diagonal cross pattern
                const patternBALost = {
                    pattern: {
                        path: {
                            d: 'M 0 0 L 10 10 M 10 0 L 0 10',
                            strokeWidth: 2
                        },
                        width: 10,
                        height: 10,
                        color: '#dc3545',
                        backgroundColor: '#ffe5e5',
                        opacity: 0.85
                    }
                };
                
                // Over SLA - red diagonal pattern
                const patternOverSLA = {
                    pattern: {
                        path: {
                            d: 'M 0 10 L 10 0 M -1 1 L 1 -1 M 9 11 L 11 9',
                            strokeWidth: 2
                        },
                        width: 8,
                        height: 8,
                        color: '#dc3545',
                        opacity: 0.8
                    }
                };
                
                // Cost of Penalties - gray horizontal lines pattern
                const patternPenalties = {
                    pattern: {
                        path: {
                            d: 'M 0 0 L 10 0 M 0 3 L 10 3 M 0 6 L 10 6 M 0 9 L 10 9',
                            strokeWidth: 1.5
                        },
                        width: 10,
                        height: 10,
                        color: '#6c757d',
                        opacity: 0.9
                    }
                };

                // Create the chart
                Highcharts.chart(chartRevenueCostID, {
                    chart: {
                        type: 'column',
                        zoomType: 'xy'
                    },
                    title: {
                        text: 'Revenue vs Cost Analysis - ' + (selectedYear || new Date().getFullYear()),
                        align: 'left',
                        style: {
                            fontSize: '18px',
                            fontWeight: 'bold'
                        }
                    },
                    subtitle: {
                        text: 'Single stacked bar showing waterfall from Revenue → Payroll → Profit, with potential gains from BA Lost, Over SLA, and Penalties. Click segments for details.',
                        align: 'left'
                    },
                    xAxis: {
                        categories: months,
                        crosshair: true,
                        title: {
                            text: 'Month'
                        }
                    },
                    yAxis: [{
                        // Left Y-axis - Amount (Rp) for stacked bars
                        title: {
                            text: 'Amount (Rp)',
                            style: {
                                color: '#666',
                                fontWeight: 'bold'
                            }
                        },
                        labels: {
                            formatter: function() {
                                const absValue = Math.abs(this.value);
                                if (absValue >= 1000000000) {
                                    return 'Rp ' + Highcharts.numberFormat(absValue / 1000000000, 1) + 'B';
                                } else if (absValue >= 1000000) {
                                    return 'Rp ' + Highcharts.numberFormat(absValue / 1000000, 1) + 'M';
                                } else if (absValue >= 1000) {
                                    return 'Rp ' + Highcharts.numberFormat(absValue / 1000, 0) + 'K';
                                } else {
                                    return 'Rp ' + Highcharts.numberFormat(absValue, 0);
                                }
                            }
                        },
                        stackLabels: {
                            enabled: false // Disable default stack label, we'll use custom annotation
                        }
                    }, {
                        // Right Y-axis - Gross Profit (Rp)
                        title: {
                            text: 'Gross Profit (Rp)',
                            style: {
                                color: '#28a745',
                                fontWeight: 'bold'
                            }
                        },
                        labels: {
                            formatter: function() {
                                const absValue = Math.abs(this.value);
                                if (absValue >= 1000000000) {
                                    return 'Rp ' + Highcharts.numberFormat(absValue / 1000000000, 1) + 'B';
                                } else if (absValue >= 1000000) {
                                    return 'Rp ' + Highcharts.numberFormat(absValue / 1000000, 1) + 'M';
                                } else if (absValue >= 1000) {
                                    return 'Rp ' + Highcharts.numberFormat(absValue / 1000, 0) + 'K';
                                } else {
                                    return 'Rp ' + Highcharts.numberFormat(absValue, 0);
                                }
                            },
                            style: {
                                color: '#28a745'
                            }
                        },
                        opposite: true
                    }],
                    tooltip: {
                        shared: true,
                        useHTML: true,
                        formatter: function() {
                            const monthIndex = this.points[0].point.index;
                            const monthData = data[monthIndex];
                            
                            let tooltip = '<div style="padding:8px; min-width:250px;"><b style="font-size:14px;">' + this.x + '</b><br/>';
                            
                            tooltip += '<div style="margin-top:8px;"><strong>Waterfall Breakdown:</strong></div>';
                            tooltip += '<table style="width:100%; margin-top:5px;">';
                            
                            // Revenue (starting point)
                            tooltip += '<tr><td><span style="color:#28a745">●</span> Revenue:</td><td style="text-align:right"><b>Rp ' + 
                                       Highcharts.numberFormat(monthData.revenue, 0, ',', '.') + '</b></td><td style="text-align:right">(' + 
                                       monthData.revenue_ticket_count + ' tickets)</td></tr>';
                            
                            // Payroll (cost)
                            tooltip += '<tr><td><span style="color:#ffc107">●</span> - Payroll:</td><td style="text-align:right"><b>Rp ' + 
                                       Highcharts.numberFormat(monthData.payroll, 0, ',', '.') + '</b></td><td style="text-align:right">(' + 
                                       monthData.payroll_ticket_count + ' tickets)</td></tr>';
                            
                            // Profit
                            tooltip += '<tr style="border-top:1px solid #ccc;"><td><span style="color:#17a2b8">●</span> = Profit:</td><td style="text-align:right"><b>Rp ' + 
                                       Highcharts.numberFormat(monthData.profit, 0, ',', '.') + '</b></td><td></td></tr>';
                            
                            tooltip += '<tr><td colspan="3" style="padding-top:8px;"><strong>Potential Additional Revenue:</strong></td></tr>';
                            
                            // BA Lost
                            tooltip += '<tr><td><span style="color:#dc3545">●</span> BA Lost:</td><td style="text-align:right"><b>Rp ' + 
                                       Highcharts.numberFormat(monthData.ba_lost, 0, ',', '.') + '</b></td><td style="text-align:right">(' + 
                                       monthData.ba_lost_ticket_count + ' devices)</td></tr>';
                            
                            // Over SLA
                            tooltip += '<tr><td><span style="color:#fd7e14">●</span> Over SLA:</td><td style="text-align:right"><b>Rp ' + 
                                       Highcharts.numberFormat(monthData.over_sla, 0, ',', '.') + '</b></td><td style="text-align:right">(' + 
                                       monthData.over_sla_ticket_count + ' tickets)</td></tr>';
                            
                            // Cost of Penalties
                            tooltip += '<tr><td><span style="color:#6f42c1">●</span> Penalties:</td><td style="text-align:right"><b>Rp ' + 
                                       Highcharts.numberFormat(monthData.cost_penalties, 0, ',', '.') + '</b></td><td style="text-align:right">(' + 
                                       monthData.cost_penalties_count + ' tickets)</td></tr>';
                            
                            // Gross Profit
                            tooltip += '<tr style="border-top:2px solid #000; font-size:13px;"><td><strong>Gross Profit:</strong></td><td style="text-align:right"><strong>Rp ' + 
                                       Highcharts.numberFormat(monthData.gross_profit, 0, ',', '.') + '</strong></td><td></td></tr>';
                            
                            tooltip += '</table></div>';
                            return tooltip;
                        }
                    },
                    plotOptions: {
                        column: {
                            stacking: 'normal',
                            cursor: 'pointer',
                            dataLabels: {
                                enabled: true,
                                formatter: function() {
                                    // Show amounts in Rupiah format
                                    if (this.y > 0) {
                                        if (this.y >= 1000000000) {
                                            return 'Rp ' + Highcharts.numberFormat(this.y / 1000000000, 1) + 'B';
                                        } else if (this.y >= 1000000) {
                                            return 'Rp ' + Highcharts.numberFormat(this.y / 1000000, 1) + 'M';
                                        } else if (this.y >= 1000) {
                                            return 'Rp ' + Highcharts.numberFormat(this.y / 1000, 0) + 'K';
                                        } else {
                                            return 'Rp ' + Highcharts.numberFormat(this.y, 0);
                                        }
                                    }
                                    return null;
                                },
                                style: {
                                    color: '#000',
                                    textOutline: '2px contrast',
                                    fontWeight: 'bold',
                                    fontSize: '10px'
                                }
                            },
                            point: {
                                events: {
                                    click: function() {
                                        showDrillDown(this.category, this.series.name);
                                    }
                                }
                            }
                        },
                        series: {
                            stacking: 'normal',
                            dataLabels: {
                                enabled: true
                            }
                        },
                        line: {
                            cursor: 'default',
                            marker: {
                                enabled: true
                            },
                            dataLabels: {
                                enabled: true,
                                formatter: function() {
                                    // Show profit in Rupiah format
                                    if (this.y >= 1000000000) {
                                        return 'Rp ' + Highcharts.numberFormat(this.y / 1000000000, 1) + 'B';
                                    } else if (this.y >= 1000000) {
                                        return 'Rp ' + Highcharts.numberFormat(this.y / 1000000, 1) + 'M';
                                    } else if (this.y >= 1000) {
                                        return 'Rp ' + Highcharts.numberFormat(this.y / 1000, 0) + 'K';
                                    } else {
                                        return 'Rp ' + Highcharts.numberFormat(this.y, 0);
                                    }
                                },
                                style: {
                                    color: '#007bff',
                                    textOutline: '2px contrast',
                                    fontWeight: 'bold',
                                    fontSize: '11px'
                                }
                            }
                        }
                    },
                    legend: {
                        layout: 'horizontal',
                        align: 'center',
                        verticalAlign: 'bottom',
                        itemStyle: {
                            fontSize: '12px'
                        }
                    },
                    series: [
                        // Stack from bottom to top: Payroll -> Profit -> BA Lost -> Over SLA -> Cost of Penalties
                        // The TOTAL height represents Revenue
                        {
                            name: 'Payroll (Cost to Technicians)',
                            data: payrollData.map(d => d.y),
                            color: '#f4a460', // Sandy brown/tan solid color
                            type: 'column',
                            stack: 'total',
                            yAxis: 0,
                            index: 4, // Higher index = drawn first (bottom of stack)
                            legendIndex: 0
                        },
                        {
                            name: 'Profit (Revenue - Payroll)',
                            data: profitData.map(d => d.y),
                            color: '#00bcd4', // Bright cyan blue solid
                            type: 'column',
                            stack: 'total',
                            yAxis: 0,
                            index: 3,
                            legendIndex: 1
                        },
                        {
                            name: 'BA Lost',
                            data: baLostData.map(d => d.y),
                            color: patternBALost, // Red danger diagonal cross pattern
                            type: 'column',
                            stack: 'total',
                            yAxis: 0,
                            index: 2,
                            legendIndex: 2
                        },
                        {
                            name: 'Over SLA',
                            data: overSLAData.map(d => d.y),
                            color: patternOverSLA, // Red/purple diagonal pattern
                            type: 'column',
                            stack: 'total',
                            yAxis: 0,
                            index: 1,
                            legendIndex: 3
                        },
                        {
                            name: 'Cost of Penalties (From Customers)',
                            data: penaltiesData.map(d => d.y),
                            color: patternPenalties, // Gray horizontal lines
                            type: 'column',
                            stack: 'total',
                            yAxis: 0,
                            index: 0, // Lower index = drawn last (top of stack)
                            legendIndex: 4
                        }
                    ],
                    // Add annotations for Revenue and Gross Profit labels
                    annotations: [{
                        shapes: [
                            // Revenue vertical line on the left side of each bar
                            ...data.map((d, index) => ({
                                type: 'path',
                                points: [
                                    { x: index - 0.25, y: 0, xAxis: 0, yAxis: 0 }, // Offset to left side of bar
                                    { x: index - 0.25, y: d.revenue, xAxis: 0, yAxis: 0 }
                                ],
                                strokeWidth: 3,
                                stroke: '#28a745'
                            })),
                            // Gross Profit vertical line on the right side of each bar
                            ...data.map((d, index) => ({
                                type: 'path',
                                points: [
                                    { x: index + 0.25, y: d.revenue, xAxis: 0, yAxis: 0 }, // Offset to right side of bar
                                    { x: index + 0.25, y: d.revenue + d.ba_lost + d.over_sla + d.cost_penalties, xAxis: 0, yAxis: 0 }
                                ],
                                strokeWidth: 3,
                                stroke: '#007bff',
                                dashStyle: 'Dash'
                            }))
                        ],
                        labels: [
                            // Revenue labels on the left side
                            ...data.map((d, index) => ({
                                point: {
                                    x: index - 0.25, // Align with left-side vertical line
                                    y: d.revenue / 2, // Position at middle of revenue
                                    xAxis: 0,
                                    yAxis: 0
                                },
                                text: '<b>Revenue</b><br/>Rp ' + 
                                      (d.revenue >= 1000000000 ? 
                                          Highcharts.numberFormat(d.revenue / 1000000000, 1) + 'B' :
                                          d.revenue >= 1000000 ? 
                                          Highcharts.numberFormat(d.revenue / 1000000, 1) + 'M' : 
                                          Highcharts.numberFormat(d.revenue / 1000, 0) + 'K'),
                                verticalAlign: 'middle',
                                align: 'right', // Align text to right edge
                                x: -10, // Small negative offset from line position
                                backgroundColor: 'rgba(40, 167, 69, 0.9)',
                                borderColor: '#28a745',
                                borderWidth: 2,
                                borderRadius: 5,
                                padding: 8,
                                style: {
                                    fontSize: '11px',
                                    color: '#ffffff',
                                    fontWeight: 'bold'
                                }
                            })),
                            // Gross Profit labels on the right side
                            ...data.map((d, index) => ({
                                point: {
                                    x: index + 0.25, // Align with right-side vertical line
                                    y: d.revenue + (d.ba_lost + d.over_sla + d.cost_penalties) / 2, // Position at middle of gross profit area
                                    xAxis: 0,
                                    yAxis: 0
                                },
                                text: '<b>Gross Profit</b><br/>Rp ' + 
                                      (d.gross_profit >= 1000000000 ? 
                                          Highcharts.numberFormat(d.gross_profit / 1000000000, 1) + 'B' :
                                          d.gross_profit >= 1000000 ? 
                                          Highcharts.numberFormat(d.gross_profit / 1000000, 1) + 'M' : 
                                          Highcharts.numberFormat(d.gross_profit / 1000, 0) + 'K'),
                                verticalAlign: 'middle',
                                align: 'left', // Align text to left edge
                                x: 10, // Small positive offset from line position
                                backgroundColor: 'rgba(0, 123, 255, 0.9)',
                                borderColor: '#007bff',
                                borderWidth: 2,
                                borderRadius: 5,
                                padding: 8,
                                style: {
                                    fontSize: '11px',
                                    color: '#ffffff',
                                    fontWeight: 'bold'
                                }
                            }))
                        ]
                    }],
                    credits: {
                        enabled: false
                    },
                    exporting: {
                        enabled: true,
                        buttons: {
                            contextButton: {
                                menuItems: [
                                    'viewFullscreen',
                                    'printChart',
                                    'separator',
                                    'downloadPNG',
                                    'downloadJPEG',
                                    'downloadPDF',
                                    'downloadSVG',
                                    'separator',
                                    'downloadCSV',
                                    'downloadXLS'
                                ]
                            }
                        },
                        url: '/odooms-monitoring/highcharts-export'
                    }
                });
            } else {
                Swal.fire({
                    icon: 'warning',
                    title: 'No Data',
                    text: 'No data available for the chart.'
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

// Show drill-down details when clicking on a bar
function showDrillDown(month, seriesName) {
    // Map series name to category
    const categoryMap = {
        'Payroll (Cost to Technicians)': 'payroll',
        'Profit (Revenue - Payroll)': 'revenue',
        'BA Lost': 'ba_lost',
        'Over SLA': 'over_sla',
        'Cost of Penalties (From Customers)': 'penalties'
    };

    const category = categoryMap[seriesName];
    if (!category) {
        console.log('No drill-down available for:', seriesName);
        return;
    }

    // Get selected year
    const selectedYear = $('#select-year').val() || new Date().getFullYear();

    Swal.fire({
        title: 'Loading Details...',
        html: 'Fetching drill-down data for <b>' + seriesName + '</b> in <b>' + month + ' ' + selectedYear + '</b>',
        allowOutsideClick: false,
        didOpen: () => {
            Swal.showLoading();
        }
    });

    $.ajax({
        url: '/odooms-monitoring/drill-down-cost-revenue',
        type: 'POST',
        contentType: 'application/json',
        data: JSON.stringify({
            month: month,
            category: category
        }),
        success: function(response) {
            if (response.success && response.data && response.data.length > 0) {
                const data = response.data;
                
                // Get selected year for title
                const selectedYear = $('#select-year').val() || new Date().getFullYear();
                
                // Build HTML table for drill-down
                // Check if this is Over SLA or BA Lost category for different column layout
                const isOverSLA = (category === 'over_sla');
                const isBALost = (category === 'ba_lost');
                
                let tableHTML = '<div style="max-height: 500px; overflow-y: auto; text-align: left;">';
                tableHTML += '<table class="table table-sm table-striped table-hover">';
                tableHTML += '<thead class="bg-label-danger" style="position: sticky; top: 0; z-index: 10;">';
                tableHTML += '<tr>';
                
                // BA Lost has detailed device columns
                if (isBALost) {
                    tableHTML += '<th>#</th>';
                    tableHTML += '<th>Serial Number</th>';
                    tableHTML += '<th>Vendor</th>';
                    tableHTML += '<th>Location</th>';
                    tableHTML += '<th>Merk</th>';
                    tableHTML += '<th>EDC Type</th>';
                    tableHTML += '<th>Device</th>';
                    tableHTML += '<th>Status EDC</th>';
                    tableHTML += '<th>Head</th>';
                    tableHTML += '<th>SP</th>';
                    tableHTML += '<th>Region</th>';
                    tableHTML += '<th class="text-end">Price</th>';
                } else {
                    tableHTML += '<th>Company</th>';
                    tableHTML += '<th>Task Type</th>';
                    if (isOverSLA) {
                        tableHTML += '<th>SLA Status</th>';
                    }
                    tableHTML += '<th class="text-end">Count</th>';
                    tableHTML += '<th class="text-end">Price/Unit</th>';
                    tableHTML += '<th class="text-end">Total</th>';
                }
                
                tableHTML += '</tr>';
                tableHTML += '</thead>';
                tableHTML += '<tbody>';
                
                let grandTotal = 0;
                let totalCount = 0;
                
                if (isBALost) {
                    // BA Lost detailed records
                    data.forEach(function(row, index) {
                        tableHTML += '<tr>';
                        tableHTML += '<td>' + (index + 1) + '</td>';
                        tableHTML += '<td><strong>' + (row.serial_number || '-') + '</strong></td>';
                        tableHTML += '<td>' + (row.vendor || '-') + '</td>';
                        tableHTML += '<td>' + (row.location || '-') + '</td>';
                        tableHTML += '<td>' + (row.merk || '-') + '</td>';
                        tableHTML += '<td>' + (row.edc_type || '-') + '</td>';
                        tableHTML += '<td>' + (row.device || '-') + '</td>';
                        tableHTML += '<td>' + (row.status_edc || '-') + '</td>';
                        tableHTML += '<td>' + (row.head || '-') + '</td>';
                        tableHTML += '<td>' + (row.sp || '-') + '</td>';
                        tableHTML += '<td>' + (row.region || '-') + '</td>';
                        tableHTML += '<td class="text-end">Rp ' + Highcharts.numberFormat(row.price, 0, ',', '.') + '</td>';
                        tableHTML += '</tr>';
                        grandTotal += row.price;
                        totalCount++;
                    });
                } else {
                    // Other categories
                    data.forEach(function(row) {
                        tableHTML += '<tr>';
                        tableHTML += '<td>' + row.company + '</td>';
                        tableHTML += '<td>' + row.task_type + '</td>';
                        if (isOverSLA) {
                            tableHTML += '<td>' + (row.sla_status || '-') + '</td>';
                        }
                        tableHTML += '<td class="text-end">' + Highcharts.numberFormat(row.count, 0) + '</td>';
                        tableHTML += '<td class="text-end">Rp ' + Highcharts.numberFormat(row.price, 0, ',', '.') + '</td>';
                        tableHTML += '<td class="text-end"><strong>Rp ' + Highcharts.numberFormat(row.total, 0, ',', '.') + '</strong></td>';
                        tableHTML += '</tr>';
                        grandTotal += row.total;
                        totalCount += row.count;
                    });
                }
                
                tableHTML += '</tbody>';
                tableHTML += '<tfoot class="table-active bg-label-danger" style="position: sticky; bottom: 0;">';
                tableHTML += '<tr>';
                
                if (isBALost) {
                    tableHTML += '<th colspan="11">Grand Total</th>';
                    tableHTML += '<th class="text-end"><strong>Rp ' + Highcharts.numberFormat(grandTotal, 0, ',', '.') + '</strong></th>';
                } else {
                    tableHTML += '<th colspan="' + (isOverSLA ? '3' : '2') + '">Grand Total</th>';
                    tableHTML += '<th class="text-end">' + Highcharts.numberFormat(totalCount, 0) + '</th>';
                    tableHTML += '<th></th>';
                    tableHTML += '<th class="text-end"><strong>Rp ' + Highcharts.numberFormat(grandTotal, 0, ',', '.') + '</strong></th>';
                }
                
                tableHTML += '</tr>';
                tableHTML += '</tfoot>';
                tableHTML += '</table>';
                
                // Add note for BA Lost
                if (isBALost) {
                    tableHTML += '<div class="alert alert-info mt-3" style="text-align: left;">';
                    tableHTML += '<i class="fas fa-info-circle me-2"></i>';
                    tableHTML += '<strong>BA Lost Details:</strong> EDC devices from BA Lost table (' + totalCount + ' devices) where link_foto IS NULL AND note_all IS NULL. ';
                    tableHTML += 'Total value: <strong>Rp ' + Highcharts.numberFormat(grandTotal, 0, ',', '.') + '</strong>';
                    tableHTML += '</div>';
                }
                
                tableHTML += '</div>';
                
                Swal.fire({
                    title: seriesName + ' - ' + month + ' ' + selectedYear,
                    html: tableHTML,
                    width: '95%',
                    confirmButtonText: 'Close',
                    confirmButtonColor: '#3085d6'
                });
            } else {
                Swal.fire({
                    icon: 'info',
                    title: 'No Details Available',
                    text: 'No detail data available for this selection.',
                    confirmButtonText: 'OK'
                });
            }
        },
        error: function(jqXHR, textStatus, errorThrown) {
            console.error('AJAX Error loading drill-down:', textStatus, errorThrown);
            Swal.fire({
                icon: 'error',
                title: 'Error Loading Details',
                text: 'Failed to fetch drill-down data. Please try again.',
                confirmButtonText: 'OK'
            });
        }
    });
}
