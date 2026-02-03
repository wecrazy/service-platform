/**
 * DataTables Extensions (jquery)
 */

'use strict';
$(function () {


  // Table
  // --------------------------------------------------------------------

  var dt_settlement_log = $('.dt_settlement_log');
  if (dt_settlement_log.length) {
    var dt_settlement_log_tab = dt_settlement_log.DataTable({
      lengthMenu: [3, 5, 10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 3, // Set the default page length
      scrollX:true,
      scrollY:true,
      serverSide: true,
      ajax: {
				url: webPath+'tab-settlement-log/table',
				type: 'POST',
      },
      columns: [        
        { data: "id"},
        { data: "mid"},
        { data: "tid"},
        { data: "trace"},
        { data: "batch"},
        { data: "settle_date"},
        { data: "total_transaction"},
        { data: "total_amount"},
        { data: "clearing_flag"},
        { data: "created_at"},
        { data: "updated_at"},
      ],
      order: [[0, 'desc']],
      columnDefs: [
        {
          // Actions
          targets: 6,
          render: function (data, type, full, meta) {
            return (data+`<button class="btn " onclick="$('.dt_settlement_details_log').DataTable().ajax.url( webPath + 'tab-settlement-log/details/table?_id='+${full['id']}).load();"><i class='bx bx-info-circle'></i></button>`);
          }
        }
        // {
        //   // Actions
        //   targets: -1,
        //   title: 'Actions',
        //   searchable: false,
        //   orderable: false,
        //   render: function (data, type, full, meta) {
        //     return (
        //       '<div class="d-inline-block">' +
        //       '<a href="javascript:;" class="btn btn-sm btn-icon dropdown-toggle hide-arrow" data-bs-toggle="dropdown"><i class="bx bx-dots-vertical-rounded"></i></a>' +
        //       '<div class="dropdown-menu dropdown-menu-end m-0">' +
        //       '<a href="javascript:;" class="dropdown-item">Details</a>' +
        //       '<a href="javascript:;" class="dropdown-item">Archive</a>' +
        //       '<div class="dropdown-divider"></div>' +
        //       '<a href="javascript:;" class="dropdown-item text-danger delete-record">Delete</a>' +
        //       '</div>' +
        //       '</div>' +
        //       '<a href="javascript:;" class="item-edit text-body"><i class="bx bxs-edit"></i></a>'
        //     );
        //   }
        // }
      ],
      dom:'<"card-header d-flex border-top rounded-0 flex-wrap py-md-0"' +
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
          filename: `Settlement_${timestampnow}`,
          className: 'btn btn-label-secondary dropdown-toggle ms-2 me-3',
          text: '<i class="bx bx-export me-1"></i>Ekstrak',
          buttons: [
            {
              extend: 'print',
              filename: `Settlement_${timestampnow}`,
              text: '<i class="bx bx-printer me-2" ></i>Print',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('product-name')) {
                        result = result + item.lastChild.firstChild.textContent;
                      } else if (item.innerText === undefined) {
                        result = result + item.textContent;
                      } else result = result + item.innerText;
                    });
                    return result;
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
              filename: `Settlement_${timestampnow}`,
              text: '<i class="bx bx-file me-2" ></i>Csv',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('product-name')) {
                        result = result + item.lastChild.firstChild.textContent;
                      } else if (item.innerText === undefined) {
                        result = result + item.textContent;
                      } else result = result + item.innerText;
                    });
                    return result;
                  }
                }
              }
            },
            {
              extend: 'excel',
              filename: `Settlement_${timestampnow}`,
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('product-name')) {
                        result = result + item.lastChild.firstChild.textContent;
                      } else if (item.innerText === undefined) {
                        result = result + item.textContent;
                      } else result = result + item.innerText;
                    });
                    return result;
                  }
                }
              }
            },
            {
              extend: 'pdf',
              filename: `Settlement_${timestampnow}`,
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('product-name')) {
                        result = result + item.lastChild.firstChild.textContent;
                      } else if (item.innerText === undefined) {
                        result = result + item.textContent;
                      } else result = result + item.innerText;
                    });
                    return result;
                  }
                }
              }
            },
            {
              extend: 'copy',
              filename: `Settlement_${timestampnow}`,
              text: '<i class="bx bx-copy me-2" ></i>Copy',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('product-name')) {
                        result = result + item.lastChild.firstChild.textContent;
                      } else if (item.innerText === undefined) {
                        result = result + item.textContent;
                      } else result = result + item.innerText;
                    });
                    return result;
                  }
                }
              }
            },
            {
              text: '<i class="bx bx-data"></i> All (CSV)',
              className: 'dropdown-item',
              action: function (e, dt, button, config) {
                fetch(webPath + 'tab-settlement-log/table.csv', {
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
                  const filename = `Settlement_Report_${timestampnow}.csv`;
              
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
      
    });
  }
  var dt_settlement_details_log = $('.dt_settlement_details_log');
  if (dt_settlement_details_log.length) {
    var dt_settlement_details_log_tab = dt_settlement_details_log.DataTable({
      lengthMenu: [10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 50, // Set the default page length
      scrollX:true,
      scrollY:true,
      // ajax: webPath + 'tab-settlement-log/details/table',
      serverSide: true,
      ajax: {
				url: webPath+'tab-settlement-log/details/table',
				type: 'POST',
      },
      columns: [        
        { data: "" },
        { data: "id" },
        { data: "settlement_id" },
        { data: "transaction_id" },
        { data: "transaction_type" },
        { data: "mid" },
        { data: "tid" },
        { data: "card_type" },
        { data: "amount" },
        { data: "transaction_date" },
        { data: "trace" },
        { data: "response_code" },
        { data: "response_at" },
        { data: "approval_code" },
        { data: "reff_id" },
        { data: "issuer_id" },
        { data: "contactless_flag" },
        { data: "pfid" },
        { data: "void_id" },
        { data: "status" },
        { data: "created_at" },
        { data: "batch" },
        
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
          responsivePriority: 3,
          targets: 2,
          render: function (data, type, full, meta) {
            if (full['status'] == 2){
              data = data + ` <b class="text-warning" >(BATCH UPLOAD)</b>`
            }
            return data;
          }
        },
      ],
      dom:'<"card-header d-flex border-top rounded-0 flex-wrap py-md-0"' +
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
          filename: `Settlement_Details_${timestampnow}`,
          className: 'btn btn-label-secondary dropdown-toggle ms-2 me-3',
          text: '<i class="bx bx-export me-1"></i>Ekstrak',
          buttons: [
            {
              extend: 'print',
              filename: `Settlement_Details_${timestampnow}`,
              text: '<i class="bx bx-printer me-2" ></i>Print',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('product-name')) {
                        result = result + item.lastChild.firstChild.textContent;
                      } else if (item.innerText === undefined) {
                        result = result + item.textContent;
                      } else result = result + item.innerText;
                    });
                    return result;
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
              filename: `Settlement_Details_${timestampnow}`,
              text: '<i class="bx bx-file me-2" ></i>Csv',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('product-name')) {
                        result = result + item.lastChild.firstChild.textContent;
                      } else if (item.innerText === undefined) {
                        result = result + item.textContent;
                      } else result = result + item.innerText;
                    });
                    return result;
                  }
                }
              }
            },
            {
              extend: 'excel',
              filename: `Settlement_Details_${timestampnow}`,
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('product-name')) {
                        result = result + item.lastChild.firstChild.textContent;
                      } else if (item.innerText === undefined) {
                        result = result + item.textContent;
                      } else result = result + item.innerText;
                    });
                    return result;
                  }
                }
              }
            },
            {
              extend: 'pdf',
              filename: `Settlement_Details_${timestampnow}`,
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('product-name')) {
                        result = result + item.lastChild.firstChild.textContent;
                      } else if (item.innerText === undefined) {
                        result = result + item.textContent;
                      } else result = result + item.innerText;
                    });
                    return result;
                  }
                }
              }
            },
            {
              extend: 'copy',
              filename: `Settlement_Details_${timestampnow}`,
              text: '<i class="bx bx-copy me-2" ></i>Copy',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('product-name')) {
                        result = result + item.lastChild.firstChild.textContent;
                      } else if (item.innerText === undefined) {
                        result = result + item.textContent;
                      } else result = result + item.innerText;
                    });
                    return result;
                  }
                }
              }
            },
            {
              text: '<i class="bx bx-data"></i> All (CSV)',
              className: 'dropdown-item',
              action: function (e, dt, button, config) {
                fetch(webPath + 'tab-settlement-log/details/table.csv', {
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
                  const filename = `Settlement_Details_Report_${timestampnow}.csv`;
              
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
            },
            {
              text: '<i class="bx bx-data"></i> All Simple (CSV)',
              className: 'dropdown-item',
              action: function (e, dt, button, config) {
                fetch(webPath + 'tab-settlement-log/details/table2.csv', {
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
                  const filename = `All_Simple_Settlement_Details_Report_${timestampnow}.csv`;
              
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
      
    });
  }
  
  // Filter form control to default size
  // ? setTimeout used for multilingual table initialization
  setTimeout(() => {
    $('.dataTables_filter .form-control').removeClass('form-control-sm');
    $('.dataTables_length .form-select').removeClass('form-select-sm');
  }, 200);
});
