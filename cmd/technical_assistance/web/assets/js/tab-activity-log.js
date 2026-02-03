/**
 * App eCommerce Category List
 */

'use strict';

// Comment editor
$(function () {

  // Variable declaration for category list table
  var dt_activity_log_table = $('.dt_activity_log');
  if (dt_activity_log_table.length) {
    var dt_category = dt_activity_log_table.DataTable({
      lengthMenu: [10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 50, // Set the default page length
      ajax: webPath+'tab-activity-log/table', // JSON file to add data
      columns: [
        { data: '' },
        { data: 'date_time' },
        { data: 'action' },
        { data: 'fullname' },
        { data: 'status' },
        { data: 'detail' },
      ],
      columnDefs: [
        {
          // For Responsive
          className: 'control',
          searchable: false,
          orderable: false,
          responsivePriority: 2,
          targets: 0,
          render: function (data, type, full, meta) {
            return '';
          }
        },
      ],
      // order: [[0, 'desc']],
      dom:
      '<"card-header d-flex border-top rounded-0 flex-wrap py-md-0"' +
      '<"me-5 ms-n2 pe-5"l>' +
      '<"d-flex justify-content-start justify-content-md-end align-items-baseline"<"dt-action-buttons d-flex align-items-start align-items-md-center justify-content-sm-center mb-3 mb-sm-0"fB>>' +
      '>t' +
      '<"row mx-2"' +
      '<"col-sm-12 col-md-6"i>' +
      '<"col-sm-12 col-md-6"p>' +
      '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search Log'
      },
      buttons: [
        {
          extend: 'collection',
          filename: `Catatan_Aktifitas_${timestampnow}`,
          className: 'btn btn-label-secondary dropdown-toggle ms-2 me-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'print',
              filename: `Catatan_Aktifitas_${timestampnow}`,
              text: '<i class="bx bx-printer me-2" ></i>Print',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
              },
              customize: function (win) {
                // Customize print view for dark
                $(win.document.body)
                  .css('color', headingColor)
                  .css('border-color', borderColor)
                  .css('background-color', bodyBg);
                $(win.document.body)
                  .find('table')
                  .addClass('compact')
                  .css('color', 'inherit')
                  .css('border-color', 'inherit')
                  .css('background-color', 'inherit');
              }
            },
            {
              extend: 'csv',
              filename: `Catatan_Aktifitas_${timestampnow}`,
              text: '<i class="bx bx-file me-2" ></i>Csv',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
              }
            },
            {
              extend: 'excel',
              filename: `Catatan_Aktifitas_${timestampnow}`,
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
              }
            },
            {
              extend: 'pdf',
              filename: `Catatan_Aktifitas_${timestampnow}`,
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
              }
            },
            {
              extend: 'copy',
              filename: `Catatan_Aktifitas_${timestampnow}`,
              text: '<i class="bx bx-copy me-2" ></i>Copy',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
              }
            },
            {
              text: '<i class="bx bx-data"></i> All (CSV)',
              className: 'dropdown-item',
              action: function (e, dt, button, config) {
                fetch(webPath + 'tab-activity-log/table.csv', {
                    method: 'GET',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                })
                .then(response => {
                    if (!response.ok) {
                        throw new Error('Network response was not ok');
                    }
                    return response.blob();
                })
                .then(blob => {
                  // Combine them into the desired filename format
                  const filename = `Avtivity_Log_${timestampnow}.csv`;
              
                  // Create a link element to trigger the download
                  const url = window.URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.style.display = 'none';
                  a.href = url;
                  a.download = filename;
                  document.body.appendChild(a);
                  a.click();
              
                  // Clean up the URL object
                  window.URL.revokeObjectURL(url);
                })
                .catch(error => {
                    console.error('There was a problem with the fetch operation:', error);
                    Swal.fire({
                        icon: 'error',
                        title: 'Error',
                        text: 'An error occurred while downloading data. Please try again later.'
                    });
                });
              }
            }
          ]
        },
      ],
        // For responsive popup
        responsive: {
          details: {
            display: $.fn.dataTable.Responsive.display.modal({
              header: function (row) {
                var data = row.data();
                return 'Details of ' + data['registration_name'];
              }
            }),
            type: 'column',
            renderer: function (api, rowIdx, columns) {
              var data = $.map(columns, function (col, i) {
                return col.title !== '' // ? Do not show row in modal popup if title is blank (for check box)
                  ? '<tr data-dt-row="' +
                      col.rowIndex +
                      '" data-dt-column="' +
                      col.columnIndex +
                      '">' +
                      '<td>' +
                      col.title +
                      ':' +
                      '</td> ' +
                      '<td>' +
                      col.data +
                      '</td>' +
                      '</tr>'
                  : '';
              }).join('');
  
              return data ? $('<table class="table"/><tbody />').append(data) : false;
            }
          }
        },
    });
  }
});
