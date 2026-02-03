/**
 * Page User List
 */

'use strict';

// Datatable (jquery)
$(function () {

  // var exportButton = 

  // Variable declaration for table
  var dt_bin_range = $('.dt_bin_range');
  if (dt_bin_range.length) {
    var dt_user = dt_bin_range.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 5, // Set the default page length
      ajax: webPath+'tab-parameters/bin_range/table',
      columns: [
        { data: '' },
        { data: 'id'},
        { data: 'name'},
        { data: 'bank_code'},
        { data: 'card_type'},
        { data: 'pan_range_low'},
        { data: 'pan_range_high'},
        { data: 'issuer_name'},
        { data: ''},
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
        {
          targets: 2,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="name" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 3,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="bank_code" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 4,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="card_type" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 5,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="pan_range_low" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 6,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="pan_range_high" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 7,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            const names = full.issuer_list.map(issuer => issuer.name).join(",");
            if (data == ""){
              data = "DEBIT_"
            }
            return ('<p class="selectable-choice" patch="tab-parameters/'+parts[parts.length - 2]+'" field="issuer_name" point="'+full['id']+'" origin="'+data+'" choices="'+names+'">'+data+'</p>');
          }
        },
        {
          // Actions
          targets: -1,
          // title: 'Actions',
          searchable: false,
          orderable: false,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return (
              '<div class="d-inline-block text-nowrap">' +
                '<div class="dropdown">' +
                  '<button class="btn btn-sm btn-icon dropdown-toggle hide-arrow" type="button" id="actionsDropdown" data-bs-toggle="dropdown" aria-haspopup="true" aria-expanded="false">' +
                    '<i class="bx bx-dots-vertical-rounded me-2"></i>' +
                  '</button>' +
                  '<div class="dropdown-menu dropdown-menu-end" aria-labelledby="actionsDropdown">' +
                    '<button class="btn dropdown-item text-danger deleteable" tab="'+tableId+'" delete="tab-parameters/'+parts[parts.length - 2]+'/'+full['id']+'">Remove</button>' +
                  '</div>' +
                '</div>' +
              '</div>'
              );
          }
        }
      ],
      // order: [[1, 'desc']],
      dom:
        '<"row mx-2"' +
        '<"col-md-2"<"me-3"l>>' +
        '<"col-md-10"<"dt-action-buttons text-xl-end text-lg-start text-md-end text-start d-flex align-items-center justify-content-end flex-md-row flex-column mb-3 mb-md-0"fB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search..'
      },
      // Buttons with Dropdown
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle mx-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'csv',
              text: '<i class="bx bx-file me-2" ></i>Csv',
              filename: `Parameter_bin_range_${timestampnow}`,
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              filename: `Parameter_bin_range_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              filename: `Parameter_bin_range_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
          ]
        },
        {
          text: '<i class="bx bx-add-to-queue" ></i>',
          className: 'add-new btn btn-outline-info py-1 px-2',
          action: function (e, dt, node, config) {
            var currentEndpoint = dt.ajax.url();
            // console.log("currentEndpoint :", currentEndpoint);
            var tableId = dt.settings()[0].sTableId;
            // console.log("Table ID:", tableId);
            var columnFields = [];
            var allColumns = dt.settings().init().columns;
            for (var i = 0; i < allColumns.length; i++) {
              var columnData = allColumns[i].data;
              if (columnData !== '' && columnData !== 'id' && columnData !== 'created_at' && columnData !== 'updated_at') {
                if (columnData == "issuer_name"){
                  var tableData = dt.data().toArray();
                  const list = tableData[0].issuer_list.map(issuer => issuer.name);
                  if (list.length > 0){
                    columnData = ["issuer_name",list];
                  }
                }
                columnFields.push(columnData);
              }
            }
            // var tableData = dt.data().toArray();
            // console.log("Table Data:", tableData[0].issuer_list);
            // const names = tableData[0].issuer_list.map(issuer => issuer.name);

            // console.log(names)


            // console.log("column", columnFields)
            // console.log("Table Class:", this);

            addNewForm(tableId, currentEndpoint +"/create", ...columnFields);
          }
        },
      ],
      // For responsive popup
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
      initComplete: function() {
        // Add editable functionality after the table is initialized
        enableEditableCells();
      }
    }).on('draw.dt', function() {
      enableEditableCells();
    });
    // To remove default btn-secondary in export buttons
    $('.dt-buttons > .btn-group > button').removeClass('btn-secondary');
  
  }


  // Variable declaration for table
  var dt_hsm_config = $('.dt_hsm_config');
  if (dt_hsm_config.length) {
    var dt_user = dt_hsm_config.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 5, // Set the default page length
      ajax: webPath+'tab-parameters/hsm_config/table',
      columns: [
        { data: '' },
        { data: 'hsm_ip'},
        { data: 'hsm_port'},
        { data: 'created_at'},
        { data: 'updated_at'},
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
        {
          targets: 1,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="hsm_ip" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 2,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="hsm_port" point="'+full['id']+'" >'+data+'</p>');
          }
        },
      ],
      // order: [[1, 'desc']],
      dom:
        '<"row mx-2"' +
        '<"col-md-2"<"me-3"l>>' +
        '<"col-md-10"<"dt-action-buttons text-xl-end text-lg-start text-md-end text-start d-flex align-items-center justify-content-end flex-md-row flex-column mb-3 mb-md-0"fB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search..'
      },
      // Buttons with Dropdown
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle mx-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'csv',
              text: '<i class="bx bx-file me-2" ></i>Csv',
              filename: `Parameter_hsm_config_${timestampnow}`,
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              filename: `Parameter_hsm_config_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              filename: `Parameter_hsm_config_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
          ]
        },
      ],
      // For responsive popup
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
    initComplete: function() {
        // Add editable functionality after the table is initialized
        enableEditableCells();
      }
    }).on('draw.dt', function() {
      enableEditableCells();
    });
    // To remove default btn-secondary in export buttons
    $('.dt-buttons > .btn-group > button').removeClass('btn-secondary');
  }


  // Variable declaration for table
  var dt_issuer = $('.dt_issuer');
  if (dt_issuer.length) {
    var dt_user = dt_issuer.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 5, // Set the default page length
      ajax: webPath+'tab-parameters/issuer/table',
      columns: [
        { data: '' },
        { data: 'issuer_name'},
        { data: 'issuer_type'},
        { data: 'issuer_url_service'},
        { data: 'issuer_host'},
        { data: 'status'},
        { data: 'created_at'},
        { data: 'created_by'},
        { data: 'updated_at'},
        { data: 'updated_by'},
        { data: ''},
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
        {
          targets: 1,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="issuer_name" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 2,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="issuer_type" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 3,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="issuer_url_service" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 4,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="issuer_host" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 4,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="status" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          // Actions
          targets: -1,
          // title: 'Actions',
          searchable: false,
          orderable: false,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return (
              '<div class="d-inline-block text-nowrap">' +
                '<div class="dropdown">' +
                  '<button class="btn btn-sm btn-icon dropdown-toggle hide-arrow" type="button" id="actionsDropdown" data-bs-toggle="dropdown" aria-haspopup="true" aria-expanded="false">' +
                    '<i class="bx bx-dots-vertical-rounded me-2"></i>' +
                  '</button>' +
                  '<div class="dropdown-menu dropdown-menu-end" aria-labelledby="actionsDropdown">' +
                    '<button class="btn dropdown-item text-danger deleteable" tab="'+tableId+'" delete="tab-parameters/'+parts[parts.length - 2]+'/'+full['id']+'">Remove</button>' +
                  '</div>' +
                '</div>' +
              '</div>'
              );
          }
        }
      ],
      // order: [[1, 'desc']],
      dom:
        '<"row mx-2"' +
        '<"col-md-2"<"me-3"l>>' +
        '<"col-md-10"<"dt-action-buttons text-xl-end text-lg-start text-md-end text-start d-flex align-items-center justify-content-end flex-md-row flex-column mb-3 mb-md-0"fB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search..'
      },
      // Buttons with Dropdown
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle mx-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'csv',
              text: '<i class="bx bx-file me-2" ></i>Csv',
              filename: `Parameter_issuer_${timestampnow}`,
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              filename: `Parameter_issuer_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              filename: `Parameter_issuer_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7, 8, 9],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
          ]
        },
        {
          text: '<i class="bx bx-add-to-queue" ></i>',
          className: 'add-new btn btn-outline-info py-1 px-2',
          action: function (e, dt, node, config) {
            var currentEndpoint = dt.ajax.url();
            // console.log("currentEndpoint :", currentEndpoint);
            var tableId = dt.settings()[0].sTableId;
            // console.log("Table ID:", tableId);
            var columnFields = [];
            var allColumns = dt.settings().init().columns;
            for (var i = 0; i < allColumns.length; i++) {
              var columnData = allColumns[i].data;
              if (columnData !== '' && columnData !== 'id' && columnData !== 'created_at' && columnData !== 'updated_at') {
                columnFields.push(columnData);
              }
            }
            // console.log("column", columnFields)
            // console.log("Table Class:", this);

            addNewForm(tableId, currentEndpoint +"/create", ...columnFields);
          }
        },
      ],
      // For responsive popup
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
    initComplete: function() {
        // Add editable functionality after the table is initialized
        enableEditableCells();
      }
    }).on('draw.dt', function() {
      enableEditableCells();
    });
    // To remove default btn-secondary in export buttons
    $('.dt-buttons > .btn-group > button').removeClass('btn-secondary');
  }


  // Variable declaration for table
  var dt_key_config = $('.dt_key_config');
  if (dt_key_config.length) {
    var dt_user = dt_key_config.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 5, // Set the default page length
      ajax: webPath+'tab-parameters/key_config/table',
      columns: [
        { data: '' },
        { data: 'key_type'},
        { data: 'value'},
        { data: 'created_at'},
        { data: 'updated_at'},
        { data: ''},
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
        {
          targets: 1,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="key_type" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 2,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="value" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          // Actions
          targets: -1,
          // title: 'Actions',
          searchable: false,
          orderable: false,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return (
              '<div class="d-inline-block text-nowrap">' +
                '<div class="dropdown">' +
                  '<button class="btn btn-sm btn-icon dropdown-toggle hide-arrow" type="button" id="actionsDropdown" data-bs-toggle="dropdown" aria-haspopup="true" aria-expanded="false">' +
                    '<i class="bx bx-dots-vertical-rounded me-2"></i>' +
                  '</button>' +
                  '<div class="dropdown-menu dropdown-menu-end" aria-labelledby="actionsDropdown">' +
                    '<button class="btn dropdown-item text-danger deleteable" tab="'+tableId+'" delete="tab-parameters/'+parts[parts.length - 2]+'/'+full['id']+'">Remove</button>' +
                  '</div>' +
                '</div>' +
              '</div>'
              );
          }
        }
      ],
      // order: [[1, 'desc']],
      dom:
        '<"row mx-2"' +
        '<"col-md-2"<"me-3"l>>' +
        '<"col-md-10"<"dt-action-buttons text-xl-end text-lg-start text-md-end text-start d-flex align-items-center justify-content-end flex-md-row flex-column mb-3 mb-md-0"fB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search..'
      },
      // Buttons with Dropdown
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle mx-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'csv',
              text: '<i class="bx bx-file me-2" ></i>Csv',
              filename: `Parameter_key_config_${timestampnow}`,
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              filename: `Parameter_key_config_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              filename: `Parameter_key_config_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
          ]
        },
        {
          text: '<i class="bx bx-add-to-queue" ></i>',
          className: 'add-new btn btn-outline-info py-1 px-2',
          action: function (e, dt, node, config) {
            var currentEndpoint = dt.ajax.url();
            // console.log("currentEndpoint :", currentEndpoint);
            var tableId = dt.settings()[0].sTableId;
            // console.log("Table ID:", tableId);
            var columnFields = [];
            var allColumns = dt.settings().init().columns;
            for (var i = 0; i < allColumns.length; i++) {
              var columnData = allColumns[i].data;
              if (columnData !== '' && columnData !== 'id' && columnData !== 'created_at' && columnData !== 'updated_at') {
                columnFields.push(columnData);
              }
            }
            // console.log("column", columnFields)
            // console.log("Table Class:", this);

            addNewForm(tableId, currentEndpoint +"/create", ...columnFields);
          }
        },
      ],
      // For responsive popup
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
      initComplete: function() {
        // Add editable functionality after the table is initialized
        enableEditableCells();
      }
    }).on('draw.dt', function() {
      enableEditableCells();
    });
    // To remove default btn-secondary in export buttons
    $('.dt-buttons > .btn-group > button').removeClass('btn-secondary');
  }


  // Variable declaration for table
  var dt_routes_config = $('.dt_routes_config');
  if (dt_routes_config.length) {
    var dt_user = dt_routes_config.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 5, // Set the default page length
      ajax: webPath+'tab-parameters/routes_config/table',
      columns: [
        { data: '' },
        { data: 'endpoint'},
        { data: 'url'},
        { data: 'status'},
        { data: 'created_at'},
        { data: 'updated_at'},
        { data: ''},
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
        {
          targets: 1,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="endpoint" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 2,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="url" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 3,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="status" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          // Actions
          targets: -1,
          // title: 'Actions',
          searchable: false,
          orderable: false,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return (
              '<div class="d-inline-block text-nowrap">' +
                '<div class="dropdown">' +
                  '<button class="btn btn-sm btn-icon dropdown-toggle hide-arrow" type="button" id="actionsDropdown" data-bs-toggle="dropdown" aria-haspopup="true" aria-expanded="false">' +
                    '<i class="bx bx-dots-vertical-rounded me-2"></i>' +
                  '</button>' +
                  '<div class="dropdown-menu dropdown-menu-end" aria-labelledby="actionsDropdown">' +
                    '<button class="btn dropdown-item text-danger deleteable" tab="'+tableId+'" delete="tab-parameters/'+parts[parts.length - 2]+'/'+full['id']+'">Remove</button>' +
                  '</div>' +
                '</div>' +
              '</div>'
              );
          }
        }
      ],
      // order: [[1, 'desc']],
      dom:
        '<"row mx-2"' +
        '<"col-md-2"<"me-3"l>>' +
        '<"col-md-10"<"dt-action-buttons text-xl-end text-lg-start text-md-end text-start d-flex align-items-center justify-content-end flex-md-row flex-column mb-3 mb-md-0"fB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search..'
      },
      // Buttons with Dropdown
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle mx-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'csv',
              text: '<i class="bx bx-file me-2" ></i>Csv',
              filename: `Parameter_routes_config_${timestampnow}`,
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              filename: `Parameter_routes_config_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              filename: `Parameter_routes_config_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
          ]
        },
        {
          text: '<i class="bx bx-add-to-queue" ></i>',
          className: 'add-new btn btn-outline-info py-1 px-2',
          action: function (e, dt, node, config) {
            var currentEndpoint = dt.ajax.url();
            // console.log("currentEndpoint :", currentEndpoint);
            var tableId = dt.settings()[0].sTableId;
            // console.log("Table ID:", tableId);
            var columnFields = [];
            var allColumns = dt.settings().init().columns;
            for (var i = 0; i < allColumns.length; i++) {
              var columnData = allColumns[i].data;
              if (columnData !== '' && columnData !== 'id' && columnData !== 'created_at' && columnData !== 'updated_at') {
                columnFields.push(columnData);
              }
            }
            // console.log("column", columnFields)
            // console.log("Table Class:", this);

            addNewForm(tableId, currentEndpoint +"/create", ...columnFields);
          }
        },
      ],
      // For responsive popup
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
    initComplete: function() {
        // Add editable functionality after the table is initialized
        enableEditableCells();
      }
    }).on('draw.dt', function() {
      enableEditableCells();
    });
    // To remove default btn-secondary in export buttons
    $('.dt-buttons > .btn-group > button').removeClass('btn-secondary');
  }

  // Variable declaration for table
  var dt_terminal_config = $('.dt_terminal_config');
  if (dt_terminal_config.length) {
    var dt_user = dt_terminal_config.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 5, // Set the default page length
      ajax: webPath+'tab-parameters/terminal_config/table',
      columns: [
        { data: '' },
        { data: 'tid'},
        { data: 'batch'},
        { data: 'trace'},
        { data: ''},
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
        {
          targets: 1,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="tid" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 2,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="batch" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 2,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="trace" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          // Actions
          targets: -1,
          // title: 'Actions',
          searchable: false,
          orderable: false,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return (
              '<div class="d-inline-block text-nowrap">' +
                '<div class="dropdown">' +
                  '<button class="btn btn-sm btn-icon dropdown-toggle hide-arrow" type="button" id="actionsDropdown" data-bs-toggle="dropdown" aria-haspopup="true" aria-expanded="false">' +
                    '<i class="bx bx-dots-vertical-rounded me-2"></i>' +
                  '</button>' +
                  '<div class="dropdown-menu dropdown-menu-end" aria-labelledby="actionsDropdown">' +
                    '<button class="btn dropdown-item text-danger deleteable" tab="'+tableId+'" delete="tab-parameters/'+parts[parts.length - 2]+'/'+full['id']+'">Remove</button>' +
                  '</div>' +
                '</div>' +
              '</div>'
              );
          }
        }
      ],
      // order: [[1, 'desc']],
      dom:
        '<"row mx-2"' +
        '<"col-md-2"<"me-3"l>>' +
        '<"col-md-10"<"dt-action-buttons text-xl-end text-lg-start text-md-end text-start d-flex align-items-center justify-content-end flex-md-row flex-column mb-3 mb-md-0"fB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search..'
      },
      // Buttons with Dropdown
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle mx-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'csv',
              text: '<i class="bx bx-file me-2" ></i>Csv',
              filename: `Parameter_terminal_config_${timestampnow}`,
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              filename: `Parameter_terminal_config_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              filename: `Parameter_terminal_config_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
          ]
        },
        {
          text: '<i class="bx bx-add-to-queue" ></i>',
          className: 'add-new btn btn-outline-info py-1 px-2',
          action: function (e, dt, node, config) {
            var currentEndpoint = dt.ajax.url();
            // console.log("currentEndpoint :", currentEndpoint);
            var tableId = dt.settings()[0].sTableId;
            // console.log("Table ID:", tableId);
            var columnFields = [];
            var allColumns = dt.settings().init().columns;
            for (var i = 0; i < allColumns.length; i++) {
              var columnData = allColumns[i].data;
              if (columnData !== '' && columnData !== 'id' && columnData !== 'created_at' && columnData !== 'updated_at') {
                columnFields.push(columnData);
              }
            }
            // console.log("column", columnFields)
            // console.log("Table Class:", this);

            addNewForm(tableId, currentEndpoint +"/create", ...columnFields);
          }
        },
      ],
      // For responsive popup
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
    initComplete: function() {
        // Add editable functionality after the table is initialized
        enableEditableCells();
      }
    }).on('draw.dt', function() {
      enableEditableCells();
    });
    // To remove default btn-secondary in export buttons
    $('.dt-buttons > .btn-group > button').removeClass('btn-secondary');
  }

  // Variable declaration for table
  var dt_terminal_keys = $('.dt_terminal_keys');
  if (dt_terminal_keys.length) {
    var dt_user = dt_terminal_keys.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 5, // Set the default page length
      ajax: webPath+'tab-parameters/terminal_keys/table',
      columns: [
        { data: '' },
        { data: 'tid' },
        { data: 'key_type' },
        { data: 'value' },
        { data: 'created_at' },
        { data: 'updated_at' },
        { data: ''},
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
        {
          targets: 1,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="tid" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 2,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="key_type" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 3,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="value" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          // Actions
          targets: -1,
          // title: 'Actions',
          searchable: false,
          orderable: false,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return (
              '<div class="d-inline-block text-nowrap">' +
                '<div class="dropdown">' +
                  '<button class="btn btn-sm btn-icon dropdown-toggle hide-arrow" type="button" id="actionsDropdown" data-bs-toggle="dropdown" aria-haspopup="true" aria-expanded="false">' +
                    '<i class="bx bx-dots-vertical-rounded me-2"></i>' +
                  '</button>' +
                  '<div class="dropdown-menu dropdown-menu-end" aria-labelledby="actionsDropdown">' +
                    '<button class="btn dropdown-item text-danger deleteable" tab="'+tableId+'" delete="tab-parameters/'+parts[parts.length - 2]+'/'+full['id']+'">Remove</button>' +
                  '</div>' +
                '</div>' +
              '</div>'
              );
          }
        }
      ],
      // order: [[1, 'desc']],
      dom:
        '<"row mx-2"' +
        '<"col-md-2"<"me-3"l>>' +
        '<"col-md-10"<"dt-action-buttons text-xl-end text-lg-start text-md-end text-start d-flex align-items-center justify-content-end flex-md-row flex-column mb-3 mb-md-0"fB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search..'
      },
      // Buttons with Dropdown
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle mx-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'csv',
              text: '<i class="bx bx-file me-2" ></i>Csv',
              filename: `Parameter_terminal_keys_${timestampnow}`,
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              filename: `Parameter_terminal_keys_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              filename: `Parameter_terminal_keys_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
          ]
        },
        {
          text: '<i class="bx bx-add-to-queue" ></i>',
          className: 'add-new btn btn-outline-info py-1 px-2',
          action: function (e, dt, node, config) {
            var currentEndpoint = dt.ajax.url();
            // console.log("currentEndpoint :", currentEndpoint);
            var tableId = dt.settings()[0].sTableId;
            // console.log("Table ID:", tableId);
            var columnFields = [];
            var allColumns = dt.settings().init().columns;
            for (var i = 0; i < allColumns.length; i++) {
              var columnData = allColumns[i].data;
              if (columnData !== '' && columnData !== 'id' && columnData !== 'created_at' && columnData !== 'updated_at') {
                columnFields.push(columnData);
              }
            }
            // console.log("column", columnFields)
            // console.log("Table Class:", this);

            addNewForm(tableId, currentEndpoint +"/create", ...columnFields);
          }
        },
      ],
      // For responsive popup
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
    initComplete: function() {
        // Add editable functionality after the table is initialized
        enableEditableCells();
      }
    }).on('draw.dt', function() {
      enableEditableCells();
    });
    // To remove default btn-secondary in export buttons
    $('.dt-buttons > .btn-group > button').removeClass('btn-secondary');
  }
  // Variable declaration for table
  var dt_transaction_status = $('.dt_transaction_status');
  if (dt_transaction_status.length) {
    var dt_user = dt_transaction_status.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 5, // Set the default page length
      ajax: webPath+'tab-parameters/transaction_status/table',
      columns: [
        { data: '' },
        { data: 'id' },
        { data: 'status' },
        { data: 'created_at' },
        { data: 'created_by' },
        { data: ''},
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
        {
          targets: 1,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="id" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 2,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="status" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          // Actions
          targets: -1,
          // title: 'Actions',
          searchable: false,
          orderable: false,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return (
              '<div class="d-inline-block text-nowrap">' +
                '<div class="dropdown">' +
                  '<button class="btn btn-sm btn-icon dropdown-toggle hide-arrow" type="button" id="actionsDropdown" data-bs-toggle="dropdown" aria-haspopup="true" aria-expanded="false">' +
                    '<i class="bx bx-dots-vertical-rounded me-2"></i>' +
                  '</button>' +
                  '<div class="dropdown-menu dropdown-menu-end" aria-labelledby="actionsDropdown">' +
                    '<button class="btn dropdown-item text-danger deleteable" tab="'+tableId+'" delete="tab-parameters/'+parts[parts.length - 2]+'/'+full['id']+'">Remove</button>' +
                  '</div>' +
                '</div>' +
              '</div>'
              );
          }
        }
      ],
      // order: [[1, 'desc']],
      dom:
        '<"row mx-2"' +
        '<"col-md-2"<"me-3"l>>' +
        '<"col-md-10"<"dt-action-buttons text-xl-end text-lg-start text-md-end text-start d-flex align-items-center justify-content-end flex-md-row flex-column mb-3 mb-md-0"fB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search..'
      },
      // Buttons with Dropdown
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle mx-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'csv',
              text: '<i class="bx bx-file me-2" ></i>Csv',
              filename: `Parameter_transaction_status_${timestampnow}`,
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              filename: `Parameter_transaction_status_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              filename: `Parameter_transaction_status_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
          ]
        },
        {
          text: '<i class="bx bx-add-to-queue" ></i>',
          className: 'add-new btn btn-outline-info py-1 px-2',
          action: function (e, dt, node, config) {
            var currentEndpoint = dt.ajax.url();
            // console.log("currentEndpoint :", currentEndpoint);
            var tableId = dt.settings()[0].sTableId;
            // console.log("Table ID:", tableId);
            var columnFields = [];
            var allColumns = dt.settings().init().columns;
            for (var i = 0; i < allColumns.length; i++) {
              var columnData = allColumns[i].data;
              if (columnData !== '' && columnData !== 'id' && columnData !== 'created_at' && columnData !== 'updated_at') {
                columnFields.push(columnData);
              }
            }
            // console.log("column", columnFields)
            // console.log("Table Class:", this);

            addNewForm(tableId, currentEndpoint +"/create", ...columnFields);
          }
        },
      ],
      // For responsive popup
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
    initComplete: function() {
        // Add editable functionality after the table is initialized
        enableEditableCells();
      }
    }).on('draw.dt', function() {
      enableEditableCells();
    });
    // To remove default btn-secondary in export buttons
    $('.dt-buttons > .btn-group > button').removeClass('btn-secondary');
  }
  // Variable declaration for table
  var dt_transaction_types = $('.dt_transaction_types');
  if (dt_transaction_types.length) {
    var dt_user = dt_transaction_types.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 5, // Set the default page length
      ajax: webPath+'tab-parameters/transaction_types/table',
      columns: [
        { data: '' },
        { data: 'code' },
        { data: 'name' },
        { data: 'created_at' },
        { data: 'created_by' },
        { data: ''},
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
        {
          targets: 1,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="code" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          targets: 2,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return ('<p class="editable" patch="tab-parameters/'+parts[parts.length - 2]+'" field="name" point="'+full['id']+'" >'+data+'</p>');
          }
        },
        {
          // Actions
          targets: -1,
          // title: 'Actions',
          searchable: false,
          orderable: false,
          render: function (data, type, full, meta) {
            const tableId = meta.settings.sTableId;
            const parts = meta.settings.ajax.split('/');
            return (
              '<div class="d-inline-block text-nowrap">' +
                '<div class="dropdown">' +
                  '<button class="btn btn-sm btn-icon dropdown-toggle hide-arrow" type="button" id="actionsDropdown" data-bs-toggle="dropdown" aria-haspopup="true" aria-expanded="false">' +
                    '<i class="bx bx-dots-vertical-rounded me-2"></i>' +
                  '</button>' +
                  '<div class="dropdown-menu dropdown-menu-end" aria-labelledby="actionsDropdown">' +
                    '<button class="btn dropdown-item text-danger deleteable" tab="'+tableId+'" delete="tab-parameters/'+parts[parts.length - 2]+'/'+full['id']+'">Remove</button>' +
                  '</div>' +
                '</div>' +
              '</div>'
              );
          }
        }
      ],
      // order: [[1, 'desc']],
      dom:
        '<"row mx-2"' +
        '<"col-md-2"<"me-3"l>>' +
        '<"col-md-10"<"dt-action-buttons text-xl-end text-lg-start text-md-end text-start d-flex align-items-center justify-content-end flex-md-row flex-column mb-3 mb-md-0"fB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search..'
      },
      // Buttons with Dropdown
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle mx-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'csv',
              text: '<i class="bx bx-file me-2" ></i>Csv',
              filename: `Parameter_transaction_types_${timestampnow}`,
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              filename: `Parameter_transaction_types_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              filename: `Parameter_transaction_types_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
                // prevent avatar to be display
                format: {
                  body: function (inner, coldex, rowdex) {
                    if (inner.length <= 0) return inner;
                    var el = $.parseHTML(inner);
                    var result = '';
                    $.each(el, function (index, item) {
                      if (item.classList !== undefined && item.classList.contains('user-name')) {
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
          ]
        },
        {
          text: '<i class="bx bx-add-to-queue" ></i>',
          className: 'add-new btn btn-outline-info py-1 px-2',
          action: function (e, dt, node, config) {
            var currentEndpoint = dt.ajax.url();
            // console.log("currentEndpoint :", currentEndpoint);
            var tableId = dt.settings()[0].sTableId;
            // console.log("Table ID:", tableId);
            var columnFields = [];
            var allColumns = dt.settings().init().columns;
            for (var i = 0; i < allColumns.length; i++) {
              var columnData = allColumns[i].data;
              if (columnData !== '' && columnData !== 'id' && columnData !== 'created_at' && columnData !== 'updated_at') {
                columnFields.push(columnData);
              }
            }
            // console.log("column", columnFields)
            // console.log("Table Class:", this);

            addNewForm(tableId, currentEndpoint +"/create", ...columnFields);
          }
        },
      ],
      // For responsive popup
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
      initComplete: function() {
        // Add editable functionality after the table is initialized
        enableEditableCells();
      }
    }).on('draw.dt', function() {
      enableEditableCells();
    });
    // To remove default btn-secondary in export buttons
    $('.dt-buttons > .btn-group > button').removeClass('btn-secondary');
  }

  
  // Delete Record
  $('.datatables-approval tbody').on('click', '.delete-record', function () {
    // dt_user.row($(this).parents('tr')).remove().draw();
    // let id = $(this).attr('id');
 
  });

  // Filter form control to default size
  // ? setTimeout used for multilingual table initialization
  setTimeout(() => {
    $('.dataTables_filter .form-control').removeClass('form-control-sm');
    $('.dataTables_length .form-select').removeClass('form-select-sm');
  }, 300);

});
