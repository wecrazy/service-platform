/**
 * app-ecommerce-product-list
 */

'use strict';

// Datatable (jquery)
$(function () {
  const bsDatepickerRange = $('#bs-datepicker-daterange');
  if (bsDatepickerRange.length) {
    bsDatepickerRange.datepicker({
      todayHighlight: true,
      format: 'yyyy/mm/dd',
      orientation: isRtl ? 'auto right' : 'auto left'
    });
  }
  // Function to set s_date 2 days before and e_date as tomorrow
  function setDateRange() {
    var today = new Date();
    var twoDaysBefore = new Date(today);
    twoDaysBefore.setDate(today.getDate() - 2); // Set s_date 2 days before
    
    var tomorrow = new Date(today);
    tomorrow.setDate(today.getDate() + 1); // Set e_date as tomorrow
    
    // Format dates as yyyy/mm/dd
    var s_dateFormatted = formatDate(twoDaysBefore);
    var e_dateFormatted = formatDate(tomorrow);
    
    // Set input values
    // $('#filter_s_date').val(s_dateFormatted);
    $('#filter_s_date').val("2024/01/01");
    $('#filter_e_date').val(e_dateFormatted);
  }
    
  // Format date as yyyy/mm/dd
  function formatDate(date) {
      var yyyy = date.getFullYear().toString();
      var mm = (date.getMonth() + 1).toString().padStart(2, '0'); // January is 0!
      var dd = date.getDate().toString().padStart(2, '0');
      return yyyy + '/' + mm + '/' + dd;
  }
  
  // Set initial date range
  setDateRange();

  var datatables_transactions_log = $(".datatables_transactions_log");
	if (datatables_transactions_log.length) {
    datatables_transactions_log.find('tbody').html(generateSkeletonLoadingHTML(10, 9));
		let d = new Date();
    var titleName = "Report Transactions"+d.getFullYear()+"-"+d.getMonth()+"-"+d.getDate();
		var column_maas =  Array.from({ length: 30 }, (_, i) => i);
		var datatables_transactions_log = datatables_transactions_log.DataTable({
			lengthMenu: [10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 50, // Set the default page length
			serverSide: true, // Enable server-side processing
			ajax: {
				url: webPath+'tab-transaction-log/table',
				type: 'POST',
        data: function (d) {
          // Merge additional data from the input field with the default data
          d.filter_status = $('input[name="filter_status"]:checked').val();
          d.filter_type = $('input[name="filter_type"]:checked').val();
          d.filter_s_date = $('#filter_s_date').val();
          d.filter_e_date = $('#filter_e_date').val();
          return d;
        }
			},
      scrollX:true,
      scrollY:true,
			columns: [
        { data: "" },
        { data: "id" },
        { data: "transaction_id" },
        { data: "transaction_type" },
        { data: "mid" },
        { data: "tid" },
        { data: "card_type" },
        { data: "amount" },
        { data: "transaction_date_str" },
        { data: "trace" },
        { data: "contactless_flag" },
        { data: "pfid" },
        { data: "response_code" },
        { data: "response_at_str" },
        { data: "approval_code" },
        { data: "reff_id" },
        { data: "issuer_id" },
        { data: "issuer_name" },
        { data: "status" },
        { data: "status_str" },
        { data: "longitude" },
        { data: "latitude" },
        { data: "void_id" },
        { data: "settle_flag" },
        { data: "created_at_str" },
        { data: "updated_at_str" },
        { data: "settled_at_str" },
        { data: "batch_u_flag" },
			],
      order: [[1, 'desc']],
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
        {
          // order number
          targets: 2,
          render: function (data, type, full, meta) {
            var reversal = ''
            if (full.reversal_flag == 1){
              reversal = "(REVERSAL)"
            }
            return (`${data} <p class="text-warning">${reversal}</p>`);
          }
        },
      ],
        dom:
        '<"card-header d-flex border-top rounded-0 flex-wrap py-md-0"' +
        '<"me-5 ms-n2 pe-5"l>' +
        '<"d-flex justify-content-start justify-content-md-end align-items-baseline"<"dt-action-buttons d-flex align-items-start align-items-md-center justify-content-sm-center mb-3 mb-sm-0"fB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
        buttons: [
          {
            extend: 'collection',
            filename: `Transactions_${timestampnow}`,
            className: 'btn btn-label-secondary dropdown-toggle ms-2 me-3',
            text: '<i class="bx bx-export me-1"></i>Export',
            buttons: [
              {
                extend: 'print',
                filename: `Transactions_${timestampnow}`,
                text: '<i class="bx bx-printer me-2" ></i>Print',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22],
                  format: {
                    body: function (data, row, column, node) {
                        return data; // Add row numbers to CSV
                    }
                  }
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
                filename: `Transactions_${timestampnow}`,
                text: '<i class="bx bx-file me-2" ></i>Csv',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22],
                  format: {
                    body: function (data, row, column, node) {
                        return data; // Add row numbers to CSV
                    }
                  }
                }
              },
              {
                extend: 'excel',
                filename: `Transactions_${timestampnow}`,
                text: '<i class="bx bxs-file-export me-2"></i>Excel',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22],
                  format: {
                    body: function (data, row, column, node) {
                        return data; // Add row numbers to CSV
                    }
                  }
                }
              },
              {
                extend: 'pdf',
                filename: `Transactions_${timestampnow}`,
                text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22],
                  format: {
                    body: function (data, row, column, node) {
                        return data; // Add row numbers to CSV
                    }
                  }
                }
              },
              {
                extend: 'copy',
                filename: `Transactions_${timestampnow}`,
                text: '<i class="bx bx-copy me-2" ></i>Copy',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22],
                  format: {
                    body: function (data, row, column, node) {
                        return data; // Add row numbers to CSV
                    }
                  }
                }
              },
              {
                text: '<i class="bx bx-data"></i> All (CSV)',
                className: 'dropdown-item',
                action: function (e, dt, button, config) {
                  fetch(webPath + 'tab-transaction-log/table.csv', {
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
                    const filename = `Catatan_Transaksi_Report_${timestampnow}.csv`;
                
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
        responsive: {
          details: {
            display: $.fn.dataTable.Responsive.display.modal({
              header: function (row) {
                var data = row.data();
                return 'Details of ' + data['id'];
              }
            }),
            type: 'column',
            renderer: function (api, rowIdx, columns) {
              console.log(api)
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
              setTimeout(function() { enableEditableCells(); }, 200);
              return data ? $('<table class="table"/><tbody />').append(data) : false;
            }
          }
        },
        initComplete:function (){
          $('.filter-trx').each(function() {
            $(this).removeAttr('disabled');
          });
        }
		});
    $('.filter-trx').each(function() {
      $(this).on('change', function() {
        if ($(this).val() != "") {
            datatables_transactions_log.ajax.reload();
        }
      });
    });
	}
});
