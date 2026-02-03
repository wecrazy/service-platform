/**
 * Form Picker
 */

'use strict';

(function () {
    // Populate Modal GUI
    // var listedArrayTitle = ['Log Service Open ACH', 'Log Service FDS', 'Log Service Uplink', 'Log Open ACH Settle', 'Log Service Transaction'];

    // // Loop through each title in the array
    // for (let i = 0; i < listedArrayTitle.length; i++) {
    //     var title = listedArrayTitle[i];
    //     var idSuffix = i + 1; // Calculate the ID suffix for each element

    //     // Generate HTML content for the row
    //     var rowHtml = '<div class="row mb-1">' +
    //         '<div class="col-md-3">' +
    //         '<span>' + title + '</span>' +
    //         '</div>' +
    //         '<div class="col-md-9">' +
    //         '<div class="input-group input-daterange" id="bs-datepicker-daterange-' + idSuffix + '">' +
    //         '<input type="text" id="startDate-' + idSuffix + '" placeholder="MM/DD/YYYY" class="form-control" />' +
    //         '<span class="input-group-text">to</span>' +
    //         '<input type="text" id="endDate-' + idSuffix + '" placeholder="MM/DD/YYYY" class="form-control" />' +
    //         '<button id="submitButton-' + idSuffix + '" class="btn btn-outline-primary" type="button"><i class="bx bxs-download"></i>Log</button>' +
    //         '</div>' +
    //         '</div>' +
    //         '</div>';

    //     // Append the HTML content to the modal body
    //     $('#download-log-modal-body').append(rowHtml);
    // }

    // Flat Picker
    // --------------------------------------------------------------------
    // Initialize Flatpickr for each date input
    const dateInputs = ['#flatpickr-date-1', '#flatpickr-date-2', '#flatpickr-date-3', '#flatpickr-date-4', '#flatpickr-date-5'];

    dateInputs.forEach(inputId => {
        const inputElement = document.querySelector(inputId);
        if (inputElement) {
            flatpickr(inputElement, {
                monthSelectorType: 'static'
            });
        }
    });
    // Initialize Bootstrap Datepicker for each date range picker
    const dateRangePickers = ['#bs-datepicker-daterange-1', '#bs-datepicker-daterange-2', '#bs-datepicker-daterange-3', '#bs-datepicker-daterange-4', '#bs-datepicker-daterange-5'];

    dateRangePickers.forEach(pickerId => {
        const pickerElement = $(pickerId);
        if (pickerElement.length) {
            pickerElement.datepicker({
                todayHighlight: true,
                orientation: isRtl ? 'auto right' : 'auto left'
            });
        }
    });

    for (let i = 1; i <= 5; i++) {
        const startDateInput = $('#startDate-' + i);
        const endDateInput = $('#endDate-' + i);
        const submitButton = $('#submitButton-' + i);

        submitButton.click(function () {
            var startDate = startDateInput.val().trim();
            var endDate = endDateInput.val().trim();

            if (startDate && endDate) {
                // Both inputs have values
                Swal.fire({
                    icon: 'question',
                    title: 'Confirmation',
                    text: 'Are you sure you want to log the selected date range: ' + startDate + ' to ' + endDate + '?',
                    showCancelButton: true,
                    confirmButtonText: 'Yes, log it!',
                    cancelButtonText: 'Cancel'
                }).then((result) => {
                    if (result.isConfirmed) {
                        // User confirmed, proceed with the action
                        // Here you can perform your AJAX request
                        // You can access the startDate and endDate variables here
                        // Use them to send data to your server
                    }
                });
            } else {
                // One or both inputs are empty
                Swal.fire({
                    icon: 'warning',
                    title: 'Warning',
                    text: 'Please select both start and end dates.'
                });
            }
        });
    }


    var dt_web_system_log = $('.dt_web_system_log');
    if (dt_web_system_log.length) {
      var dt_basic = dt_web_system_log.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
      pageLength: 5, // Set the default page length
        ajax: webPath + "tab-settings/system_log/log_web_dir/table",
        columns: [
          { data: '' },
          { data: 'name' },
          { data: 'name' },
          { data: 'name' },
          { data: 'size' },
          { data: 'modTime' },
          { data: '' }
        ],
        columnDefs: [
          {
            // For Responsive
            className: 'control',
            orderable: false,
            searchable: false,
            responsivePriority: 1,
            targets: 0,
            render: function (data, type, full, meta) {
              return '';
            }
          },
          {
            // For Checkboxes
            targets: 1,
            orderable: false,
            searchable: false,
            responsivePriority: 2,
            checkboxes: true,
            render: function () {
              return '<input type="checkbox" class="dt-checkboxes form-check-input">';
            },
            checkboxes: {
              selectAllRender: '<input type="checkbox" class="form-check-input">'
            }
          },
          {
            targets: 2,
            searchable: false,
            visible: false
          },
          {
            // Avatar image/badge, Name and post
            targets: 3,
            responsivePriority: 3,
            render: function (data, type, full, meta) {
            //   var $user_img = full['avatar'],
              var  $name = full['name'];
                // $post = full['post'];
                var $output = '';
            var stateNum = Math.floor(Math.random() * 6);
            var states = ['success', 'danger', 'warning', 'info', 'dark', 'primary', 'secondary'];
            var $state = states[stateNum];
            $output =
            "<i class='bx bx-file text-"+ $state+"' style='font-size: 30px;'></i>";
              // Creates full output for row
              var $row_output =
                '<div class="d-flex justify-content-start align-items-center user-name">' +
                '<div class="avatar-wrapper">' +
                '<div class="avatar me-2">' +
                $output +
                '</div>' +
                '</div>' +
                '<div class="d-flex flex-column">' +
                '<span class="emp_name text-truncate">' +
                $name +
                '</span>' +
                '</div>' +
                '</div>';
              return $row_output;
            }
          },
          {
            // Actions
            targets: -1,
            title: 'Actions',
            orderable: false,
            searchable: false,
            render: function (data, type, full, meta) {
                const path = meta.settings.ajax;
              return (
                '<a href="javascript:;" class="btn btn-sm btn-icon item-edit"><i class="bx bx-detail"></i></a>' +
                '<a href="'+path +'/view/'+full['view']+'" class="btn btn-sm btn-icon item-edit"><i class="bx bx-download"></i></a>' +
                '<div class="d-inline-block">' +
                '<a href="javascript:;" class="btn btn-sm btn-icon dropdown-toggle hide-arrow" data-bs-toggle="dropdown"><i class="bx bx-dots-vertical-rounded"></i></a>' +
                '<ul class="dropdown-menu dropdown-menu-end m-0">' +
                '<div class="dropdown-divider"></div>' +
                '<li><a href="javascript:;" class="dropdown-item text-danger delete-record">Delete</a></li>' +
                '</ul>' +
                '</div>' 
              );
            }
          }
        ],
        order: [[3, 'asc']],
        dom: `
        <"row"<"col-sm-12 col-md-6"l>
        <"col-sm-12 col-md-6 d-flex justify-content-center justify-content-md-end"fB>>
        t
        <"row"<"col-sm-12 col-md-6"i><"col-sm-12 col-md-6"p>
        >`,
        buttons: [
          {
            extend: 'collection',
            className: 'btn btn-sm btn-label-primary dropdown-toggle ms-3 mt-3 mb-0',
            text: '<i class="bx bx-export me-sm-1"></i> <span class="d-none d-sm-inline-block">Export</span>',
            buttons: [
              {
                extend: 'csv',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-file me-1" ></i>Csv',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
                extend: 'copy',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-copy me-1" ></i>Copy',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
              }
            ]
          },
        ],
        responsive: {
          details: {
            display: $.fn.dataTable.Responsive.display.modal({
              header: function (row) {
                var data = row.data();
                return 'Details of ' + data['full_name'];
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
        }
      });

    }
    var dt_fds_system_log = $('.dt_fds_system_log');
    if (dt_fds_system_log.length) {
      var dt_basic = dt_fds_system_log.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
      pageLength: 5, // Set the default page length
        ajax: webPath + "tab-settings/system_log/log_fds_dir/table",
        columns: [
          { data: '' },
          { data: 'name' },
          { data: 'name' },
          { data: 'name' },
          { data: 'size' },
          { data: 'modTime' },
          { data: '' }
        ],
        columnDefs: [
          {
            // For Responsive
            className: 'control',
            orderable: false,
            searchable: false,
            responsivePriority: 1,
            targets: 0,
            render: function (data, type, full, meta) {
              return '';
            }
          },
          {
            // For Checkboxes
            targets: 1,
            orderable: false,
            searchable: false,
            responsivePriority: 2,
            checkboxes: true,
            render: function () {
              return '<input type="checkbox" class="dt-checkboxes form-check-input">';
            },
            checkboxes: {
              selectAllRender: '<input type="checkbox" class="form-check-input">'
            }
          },
          {
            targets: 2,
            searchable: false,
            visible: false
          },
          {
            // Avatar image/badge, Name and post
            targets: 3,
            responsivePriority: 3,
            render: function (data, type, full, meta) {
            //   var $user_img = full['avatar'],
              var  $name = full['name'];
                // $post = full['post'];
                var $output = '';
            var stateNum = Math.floor(Math.random() * 6);
            var states = ['success', 'danger', 'warning', 'info', 'dark', 'primary', 'secondary'];
            var $state = states[stateNum];
            $output =
            "<i class='bx bx-file text-"+ $state+"' style='font-size: 30px;'></i>";
              // Creates full output for row
              var $row_output =
                '<div class="d-flex justify-content-start align-items-center user-name">' +
                '<div class="avatar-wrapper">' +
                '<div class="avatar me-2">' +
                $output +
                '</div>' +
                '</div>' +
                '<div class="d-flex flex-column">' +
                '<span class="emp_name text-truncate">' +
                $name +
                '</span>' +
                '</div>' +
                '</div>';
              return $row_output;
            }
          },
          {
            // Actions
            targets: -1,
            title: 'Actions',
            orderable: false,
            searchable: false,
            render: function (data, type, full, meta) {
                const path = meta.settings.ajax;
              return (
                '<a href="javascript:;" class="btn btn-sm btn-icon item-edit"><i class="bx bx-detail"></i></a>' +
                '<a href="'+path +'/view/'+full['view']+'" class="btn btn-sm btn-icon item-edit"><i class="bx bx-download"></i></a>' +
                '<div class="d-inline-block">' +
                '<a href="javascript:;" class="btn btn-sm btn-icon dropdown-toggle hide-arrow" data-bs-toggle="dropdown"><i class="bx bx-dots-vertical-rounded"></i></a>' +
                '<ul class="dropdown-menu dropdown-menu-end m-0">' +
                '<div class="dropdown-divider"></div>' +
                '<li><a href="javascript:;" class="dropdown-item text-danger delete-record">Delete</a></li>' +
                '</ul>' +
                '</div>' 
              );
            }
          }
        ],
        order: [[3, 'asc']],
        dom: `
        <"row"<"col-sm-12 col-md-6"l>
        <"col-sm-12 col-md-6 d-flex justify-content-center justify-content-md-end"fB>>
        t
        <"row"<"col-sm-12 col-md-6"i><"col-sm-12 col-md-6"p>
        >`,
        buttons: [
          {
            extend: 'collection',
            filename: `Settings_${timestampnow}`,
            className: 'btn btn-sm btn-label-primary dropdown-toggle ms-3 mt-3 mb-0',
            text: '<i class="bx bx-export me-sm-1"></i> <span class="d-none d-sm-inline-block">Export</span>',
            buttons: [
              {
                extend: 'csv',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-file me-1" ></i>Csv',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
                extend: 'copy',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-copy me-1" ></i>Copy',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
              }
            ]
          },
        ],
        responsive: {
          details: {
            display: $.fn.dataTable.Responsive.display.modal({
              header: function (row) {
                var data = row.data();
                return 'Details of ' + data['full_name'];
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
        }
      });

    }
    var dt_uplink_system_log = $('.dt_uplink_system_log');
    if (dt_uplink_system_log.length) {
      var dt_basic = dt_uplink_system_log.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
      pageLength: 5, // Set the default page length
        ajax: webPath + "tab-settings/system_log/log_uplink_dir/table",
        columns: [
          { data: '' },
          { data: 'name' },
          { data: 'name' },
          { data: 'name' },
          { data: 'size' },
          { data: 'modTime' },
          { data: '' }
        ],
        columnDefs: [
          {
            // For Responsive
            className: 'control',
            orderable: false,
            searchable: false,
            responsivePriority: 1,
            targets: 0,
            render: function (data, type, full, meta) {
              return '';
            }
          },
          {
            // For Checkboxes
            targets: 1,
            orderable: false,
            searchable: false,
            responsivePriority: 2,
            checkboxes: true,
            render: function () {
              return '<input type="checkbox" class="dt-checkboxes form-check-input">';
            },
            checkboxes: {
              selectAllRender: '<input type="checkbox" class="form-check-input">'
            }
          },
          {
            targets: 2,
            searchable: false,
            visible: false
          },
          {
            // Avatar image/badge, Name and post
            targets: 3,
            responsivePriority: 3,
            render: function (data, type, full, meta) {
            //   var $user_img = full['avatar'],
              var  $name = full['name'];
                // $post = full['post'];
                var $output = '';
            var stateNum = Math.floor(Math.random() * 6);
            var states = ['success', 'danger', 'warning', 'info', 'dark', 'primary', 'secondary'];
            var $state = states[stateNum];
            $output =
            "<i class='bx bx-file text-"+ $state+"' style='font-size: 30px;'></i>";
              // Creates full output for row
              var $row_output =
                '<div class="d-flex justify-content-start align-items-center user-name">' +
                '<div class="avatar-wrapper">' +
                '<div class="avatar me-2">' +
                $output +
                '</div>' +
                '</div>' +
                '<div class="d-flex flex-column">' +
                '<span class="emp_name text-truncate">' +
                $name +
                '</span>' +
                '</div>' +
                '</div>';
              return $row_output;
            }
          },
          {
            // Actions
            targets: -1,
            title: 'Actions',
            orderable: false,
            searchable: false,
            render: function (data, type, full, meta) {
                const path = meta.settings.ajax;
              return (
                '<a href="javascript:;" class="btn btn-sm btn-icon item-edit"><i class="bx bx-detail"></i></a>' +
                '<a href="'+path +'/view/'+full['view']+'" class="btn btn-sm btn-icon item-edit"><i class="bx bx-download"></i></a>' +
                '<div class="d-inline-block">' +
                '<a href="javascript:;" class="btn btn-sm btn-icon dropdown-toggle hide-arrow" data-bs-toggle="dropdown"><i class="bx bx-dots-vertical-rounded"></i></a>' +
                '<ul class="dropdown-menu dropdown-menu-end m-0">' +
                '<div class="dropdown-divider"></div>' +
                '<li><a href="javascript:;" class="dropdown-item text-danger delete-record">Delete</a></li>' +
                '</ul>' +
                '</div>' 
              );
            }
          }
        ],
        order: [[3, 'asc']],
        dom: `
        <"row"<"col-sm-12 col-md-6"l>
        <"col-sm-12 col-md-6 d-flex justify-content-center justify-content-md-end"fB>>
        t
        <"row"<"col-sm-12 col-md-6"i><"col-sm-12 col-md-6"p>
        >`,
        buttons: [
          {
            extend: 'collection',
            filename: `Settings_${timestampnow}`,
            className: 'btn btn-sm btn-label-primary dropdown-toggle ms-3 mt-3 mb-0',
            text: '<i class="bx bx-export me-sm-1"></i> <span class="d-none d-sm-inline-block">Export</span>',
            buttons: [
              {
                extend: 'csv',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-file me-1" ></i>Csv',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
                extend: 'copy',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-copy me-1" ></i>Copy',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
              }
            ]
          },
        ],
        responsive: {
          details: {
            display: $.fn.dataTable.Responsive.display.modal({
              header: function (row) {
                var data = row.data();
                return 'Details of ' + data['full_name'];
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
        }
      });

    }
    var dt_settlement_system_log = $('.dt_settlement_system_log');
    if (dt_settlement_system_log.length) {
      var dt_basic = dt_settlement_system_log.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
      pageLength: 5, // Set the default page length
        ajax: webPath + "tab-settings/system_log/log_settlement_dir/table",
        columns: [
          { data: '' },
          { data: 'name' },
          { data: 'name' },
          { data: 'name' },
          { data: 'size' },
          { data: 'modTime' },
          { data: '' }
        ],
        columnDefs: [
          {
            // For Responsive
            className: 'control',
            orderable: false,
            searchable: false,
            responsivePriority: 1,
            targets: 0,
            render: function (data, type, full, meta) {
              return '';
            }
          },
          {
            // For Checkboxes
            targets: 1,
            orderable: false,
            searchable: false,
            responsivePriority: 2,
            checkboxes: true,
            render: function () {
              return '<input type="checkbox" class="dt-checkboxes form-check-input">';
            },
            checkboxes: {
              selectAllRender: '<input type="checkbox" class="form-check-input">'
            }
          },
          {
            targets: 2,
            searchable: false,
            visible: false
          },
          {
            // Avatar image/badge, Name and post
            targets: 3,
            responsivePriority: 3,
            render: function (data, type, full, meta) {
            //   var $user_img = full['avatar'],
              var  $name = full['name'];
                // $post = full['post'];
                var $output = '';
            var stateNum = Math.floor(Math.random() * 6);
            var states = ['success', 'danger', 'warning', 'info', 'dark', 'primary', 'secondary'];
            var $state = states[stateNum];
            $output =
            "<i class='bx bx-file text-"+ $state+"' style='font-size: 30px;'></i>";
              // Creates full output for row
              var $row_output =
                '<div class="d-flex justify-content-start align-items-center user-name">' +
                '<div class="avatar-wrapper">' +
                '<div class="avatar me-2">' +
                $output +
                '</div>' +
                '</div>' +
                '<div class="d-flex flex-column">' +
                '<span class="emp_name text-truncate">' +
                $name +
                '</span>' +
                '</div>' +
                '</div>';
              return $row_output;
            }
          },
          {
            // Actions
            targets: -1,
            title: 'Actions',
            orderable: false,
            searchable: false,
            render: function (data, type, full, meta) {
                const path = meta.settings.ajax;
              return (
                '<a href="javascript:;" class="btn btn-sm btn-icon item-edit"><i class="bx bx-detail"></i></a>' +
                '<a href="'+path +'/view/'+full['view']+'" class="btn btn-sm btn-icon item-edit"><i class="bx bx-download"></i></a>' +
                '<div class="d-inline-block">' +
                '<a href="javascript:;" class="btn btn-sm btn-icon dropdown-toggle hide-arrow" data-bs-toggle="dropdown"><i class="bx bx-dots-vertical-rounded"></i></a>' +
                '<ul class="dropdown-menu dropdown-menu-end m-0">' +
                '<div class="dropdown-divider"></div>' +
                '<li><a href="javascript:;" class="dropdown-item text-danger delete-record">Delete</a></li>' +
                '</ul>' +
                '</div>' 
              );
            }
          }
        ],
        order: [[3, 'asc']],
        dom: `
        <"row"<"col-sm-12 col-md-6"l>
        <"col-sm-12 col-md-6 d-flex justify-content-center justify-content-md-end"fB>>
        t
        <"row"<"col-sm-12 col-md-6"i><"col-sm-12 col-md-6"p>
        >`,
        buttons: [
          {
            extend: 'collection',
            filename: `Settings_${timestampnow}`,
            className: 'btn btn-sm btn-label-primary dropdown-toggle ms-3 mt-3 mb-0',
            text: '<i class="bx bx-export me-sm-1"></i> <span class="d-none d-sm-inline-block">Export</span>',
            buttons: [
              {
                extend: 'csv',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-file me-1" ></i>Csv',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
                extend: 'copy',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-copy me-1" ></i>Copy',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
              }
            ]
          },
        ],
        responsive: {
          details: {
            display: $.fn.dataTable.Responsive.display.modal({
              header: function (row) {
                var data = row.data();
                return 'Details of ' + data['full_name'];
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
        }
      });

    }
    var dt_trx_request_system_log = $('.dt_trx_request_system_log');
    if (dt_trx_request_system_log.length) {
      var dt_basic = dt_trx_request_system_log.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
      pageLength: 5, // Set the default page length
        ajax: webPath + "tab-settings/system_log/log_trx_request_dir/table",
        columns: [
          { data: '' },
          { data: 'name' },
          { data: 'name' },
          { data: 'name' },
          { data: 'size' },
          { data: 'modTime' },
          { data: '' }
        ],
        columnDefs: [
          {
            // For Responsive
            className: 'control',
            orderable: false,
            searchable: false,
            responsivePriority: 1,
            targets: 0,
            render: function (data, type, full, meta) {
              return '';
            }
          },
          {
            // For Checkboxes
            targets: 1,
            orderable: false,
            searchable: false,
            responsivePriority: 2,
            checkboxes: true,
            render: function () {
              return '<input type="checkbox" class="dt-checkboxes form-check-input">';
            },
            checkboxes: {
              selectAllRender: '<input type="checkbox" class="form-check-input">'
            }
          },
          {
            targets: 2,
            searchable: false,
            visible: false
          },
          {
            // Avatar image/badge, Name and post
            targets: 3,
            responsivePriority: 3,
            render: function (data, type, full, meta) {
            //   var $user_img = full['avatar'],
              var  $name = full['name'];
                // $post = full['post'];
                var $output = '';
            var stateNum = Math.floor(Math.random() * 6);
            var states = ['success', 'danger', 'warning', 'info', 'dark', 'primary', 'secondary'];
            var $state = states[stateNum];
            $output =
            "<i class='bx bx-file text-"+ $state+"' style='font-size: 30px;'></i>";
              // Creates full output for row
              var $row_output =
                '<div class="d-flex justify-content-start align-items-center user-name">' +
                '<div class="avatar-wrapper">' +
                '<div class="avatar me-2">' +
                $output +
                '</div>' +
                '</div>' +
                '<div class="d-flex flex-column">' +
                '<span class="emp_name text-truncate">' +
                $name +
                '</span>' +
                '</div>' +
                '</div>';
              return $row_output;
            }
          },
          {
            // Actions
            targets: -1,
            title: 'Actions',
            orderable: false,
            searchable: false,
            render: function (data, type, full, meta) {
                const path = meta.settings.ajax;
              return (
                '<a href="javascript:;" class="btn btn-sm btn-icon item-edit"><i class="bx bx-detail"></i></a>' +
                '<a href="'+path +'/view/'+full['view']+'" class="btn btn-sm btn-icon item-edit"><i class="bx bx-download"></i></a>' +
                '<div class="d-inline-block">' +
                '<a href="javascript:;" class="btn btn-sm btn-icon dropdown-toggle hide-arrow" data-bs-toggle="dropdown"><i class="bx bx-dots-vertical-rounded"></i></a>' +
                '<ul class="dropdown-menu dropdown-menu-end m-0">' +
                '<div class="dropdown-divider"></div>' +
                '<li><a href="javascript:;" class="dropdown-item text-danger delete-record">Delete</a></li>' +
                '</ul>' +
                '</div>' 
              );
            }
          }
        ],
        order: [[3, 'asc']],
        dom: `
        <"row"<"col-sm-12 col-md-6"l>
        <"col-sm-12 col-md-6 d-flex justify-content-center justify-content-md-end"fB>>
        t
        <"row"<"col-sm-12 col-md-6"i><"col-sm-12 col-md-6"p>
        >`,
        buttons: [
          {
            extend: 'collection',
            filename: `Settings_${timestampnow}`,
            className: 'btn btn-sm btn-label-primary dropdown-toggle ms-3 mt-3 mb-0',
            text: '<i class="bx bx-export me-sm-1"></i> <span class="d-none d-sm-inline-block">Export</span>',
            buttons: [
              {
                extend: 'csv',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-file me-1" ></i>Csv',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
                extend: 'copy',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-copy me-1" ></i>Copy',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
              }
            ]
          },
        ],
        responsive: {
          details: {
            display: $.fn.dataTable.Responsive.display.modal({
              header: function (row) {
                var data = row.data();
                return 'Details of ' + data['full_name'];
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
        }
      });

    }
    var dt_fds_ml_system_log = $('.dt_fds_ml_system_log');
    if (dt_fds_ml_system_log.length) {
      var dt_basic = dt_fds_ml_system_log.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
      pageLength: 5, // Set the default page length
        ajax: webPath + "tab-settings/system_log/log_fds_ml_dir/table",
        columns: [
          { data: '' },
          { data: 'name' },
          { data: 'name' },
          { data: 'name' },
          { data: 'size' },
          { data: 'modTime' },
          { data: '' }
        ],
        columnDefs: [
          {
            // For Responsive
            className: 'control',
            orderable: false,
            searchable: false,
            responsivePriority: 1,
            targets: 0,
            render: function (data, type, full, meta) {
              return '';
            }
          },
          {
            // For Checkboxes
            targets: 1,
            orderable: false,
            searchable: false,
            responsivePriority: 2,
            checkboxes: true,
            render: function () {
              return '<input type="checkbox" class="dt-checkboxes form-check-input">';
            },
            checkboxes: {
              selectAllRender: '<input type="checkbox" class="form-check-input">'
            }
          },
          {
            targets: 2,
            searchable: false,
            visible: false
          },
          {
            // Avatar image/badge, Name and post
            targets: 3,
            responsivePriority: 3,
            render: function (data, type, full, meta) {
            //   var $user_img = full['avatar'],
              var  $name = full['name'];
                // $post = full['post'];
                var $output = '';
            var stateNum = Math.floor(Math.random() * 6);
            var states = ['success', 'danger', 'warning', 'info', 'dark', 'primary', 'secondary'];
            var $state = states[stateNum];
            $output =
            "<i class='bx bx-file text-"+ $state+"' style='font-size: 30px;'></i>";
              // Creates full output for row
              var $row_output =
                '<div class="d-flex justify-content-start align-items-center user-name">' +
                '<div class="avatar-wrapper">' +
                '<div class="avatar me-2">' +
                $output +
                '</div>' +
                '</div>' +
                '<div class="d-flex flex-column">' +
                '<span class="emp_name text-truncate">' +
                $name +
                '</span>' +
                '</div>' +
                '</div>';
              return $row_output;
            }
          },
          {
            // Actions
            targets: -1,
            title: 'Actions',
            orderable: false,
            searchable: false,
            render: function (data, type, full, meta) {
                const path = meta.settings.ajax;
              return (
                '<a href="javascript:;" class="btn btn-sm btn-icon item-edit"><i class="bx bx-detail"></i></a>' +
                '<a href="'+path +'/view/'+full['view']+'" class="btn btn-sm btn-icon item-edit"><i class="bx bx-download"></i></a>' +
                '<div class="d-inline-block">' +
                '<a href="javascript:;" class="btn btn-sm btn-icon dropdown-toggle hide-arrow" data-bs-toggle="dropdown"><i class="bx bx-dots-vertical-rounded"></i></a>' +
                '<ul class="dropdown-menu dropdown-menu-end m-0">' +
                '<div class="dropdown-divider"></div>' +
                '<li><a href="javascript:;" class="dropdown-item text-danger delete-record">Delete</a></li>' +
                '</ul>' +
                '</div>' 
              );
            }
          }
        ],
        order: [[3, 'asc']],
        dom: `
        <"row"<"col-sm-12 col-md-6"l>
        <"col-sm-12 col-md-6 d-flex justify-content-center justify-content-md-end"fB>>
        t
        <"row"<"col-sm-12 col-md-6"i><"col-sm-12 col-md-6"p>
        >`,
        buttons: [
          {
            extend: 'collection',
            filename: `Settings_${timestampnow}`,
            className: 'btn btn-sm btn-label-primary dropdown-toggle ms-3 mt-3 mb-0',
            text: '<i class="bx bx-export me-sm-1"></i> <span class="d-none d-sm-inline-block">Export</span>',
            buttons: [
              {
                extend: 'csv',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-file me-1" ></i>Csv',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
                extend: 'copy',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-copy me-1" ></i>Copy',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
              }
            ]
          },
        ],
        responsive: {
          details: {
            display: $.fn.dataTable.Responsive.display.modal({
              header: function (row) {
                var data = row.data();
                return 'Details of ' + data['full_name'];
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
        }
      });

    }
    var dt_system_services = $('.dt_system_services');
    if (dt_system_services.length) {
      var dt_basic = dt_system_services.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
      pageLength: 100, // Set the default page length
        ajax: webPath + 'tab-settings/systems/services/table',
        columns: [
          { data: '' },
          { data: 'unit' },
          { data: 'unit' },
          { data: 'load' },
          { data: 'active' },
          { data: 'sub' },
          { data: 'desc' },
        ],
        columnDefs: [
          {
            // For Responsive
            className: 'control',
            orderable: false,
            searchable: false,
            responsivePriority: 1,
            targets: 0,
            render: function (data, type, full, meta) {
              return '';
            }
          },
          {
            targets: 1,
            responsivePriority: 1,
            render: function (data, type, full, meta) {
              var btn = ""
              if (full["start"] == "1" ){
                btn = `<button type="button" onclick="restartService('${data}')" class="btn btn-sm btn-outline-info p-1" title="Restart"><span class="tf-icons bx bx-power-off bx-flashing-hover"></span></button>`;
              }
              return btn;
            }
          },
          {
            targets: 2,
            responsivePriority: 1,
            render: function (data, type, full, meta) {
              return (`<b>${data}</b>`);
            }
          },
          {
            targets: 3,
            responsivePriority: 2,
            render: function (data, type, full, meta) {
              var badge = "bg-label-white"
              if (data == "loaded") {
                badge = "bg-label-dark"
              }
              return (`<span class="badge ${badge}">${data}</span>`);
            }
          },
          {
            targets: 4, // Target column for ACTIVE
            responsivePriority: 3,
            render: function (data, type, full, meta) {
              var badge;
              switch (data) {
                case 'active':
                  badge = 'bg-label-success';
                  break;
                case 'activating':
                  badge = 'bg-label-warning';
                  break;
                case 'failed':
                  badge = 'bg-label-danger';
                  break;
                case 'exited':
                  badge = 'bg-label-dark';
                  break;
                default:
                  badge = 'bg-label-white';
                  break;
              }
              return `<span class="badge ${badge}">${data}</span>`;
            }
          },
          {
            targets: 5, // Target column for SUB
            responsivePriority: 4,
            render: function (data, type, full, meta) {
              var badge;
              switch (data) {
                case 'running':
                  badge = 'bg-label-success';
                  break;
                case 'auto-restart':
                  badge = 'bg-label-warning';
                  break;
                case 'dead':
                  badge = 'bg-label-danger';
                  break;
                case 'exited':
                  badge = 'bg-label-dark';
                  break;
                case 'waiting':
                  badge = 'bg-label-info';
                  break;
                default:
                  badge = 'bg-label-white';
                  break;
              }
              return `<span class="badge ${badge}">${data}</span>`;
            },
          }
        ],
        order: [[3, 'asc']],
        dom: `
        <"row"<"col-sm-12 col-md-6"l>
        <"col-sm-12 col-md-6 d-flex justify-content-center justify-content-md-end"fB>>
        t
        <"row"<"col-sm-12 col-md-6"i><"col-sm-12 col-md-6"p>
        >`,
        buttons: [
          {
            extend: 'collection',
            filename: `Settings_${timestampnow}`,
            className: 'btn btn-sm btn-label-primary dropdown-toggle ms-3 mt-3 mb-0',
            text: '<i class="bx bx-export me-sm-1"></i> <span class="d-none d-sm-inline-block">Export</span>',
            buttons: [
              {
                extend: 'csv',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-file me-1" ></i>Csv',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
                extend: 'copy',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-copy me-1" ></i>Copy',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
              }
            ]
          },
        ],
        responsive: {
          details: {
            display: $.fn.dataTable.Responsive.display.modal({
              header: function (row) {
                var data = row.data();
                return 'Details of ' + data['full_name'];
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
        }
      });

    }
    var dt_system_server_status = $('.dt_system_server_status');
    if (dt_system_server_status.length) {
      var dt_basic = dt_system_server_status.DataTable({
      lengthMenu: [5, 10, 25, 50, 100, 200, 500, 1000], 
      pageLength: 100, // Set the default page length
        ajax: webPath + 'tab-settings/systems/stat/table',
        columns: [
          { data: '' },
          { data: 'col0' },
          { data: 'col1' },
          { data: 'col2' },
          { data: 'col3' },
          { data: 'col4' },
          { data: 'col5' },
          { data: 'col6' },
          { data: 'col7' },
          { data: 'col8' },
          { data: 'col9' },
          { data: 'col10' },
          { data: 'col11' },
          
        ],
        columnDefs: [
          {
            // For Responsive
            className: 'control',
            orderable: false,
            searchable: false,
            responsivePriority: 1,
            targets: 0,
            render: function (data, type, full, meta) {
              return '';
            }
          },
        ],
        createdRow: function(row, data, dataIndex) {
          $(row).find('td').addClass('p-1');
        },
        order: [[10, 'desc']],
        dom: `
        <"row"<"col-sm-12 col-md-6"l>
        <"col-sm-12 col-md-6 d-flex justify-content-center justify-content-md-end"fB>>
        t
        <"row"<"col-sm-12 col-md-6"i><"col-sm-12 col-md-6"p>
        >`,
        buttons: [
          {
            extend: 'collection',
            filename: `Settings_${timestampnow}`,
            className: 'btn btn-sm btn-label-primary dropdown-toggle ms-3 mt-3 mb-0',
            text: '<i class="bx bx-export me-sm-1"></i> <span class="d-none d-sm-inline-block">Export</span>',
            buttons: [
              {
                extend: 'csv',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-file me-1" ></i>Csv',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
                extend: 'copy',
                filename: `Settings_${timestampnow}`,
                text: '<i class="bx bx-copy me-1" ></i>Copy',
                className: 'dropdown-item',
                exportOptions: {
                  columns: [3, 4, 5],
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
              }
            ]
          },
          {
            text: '<i class="bx bx-refresh"></i>',
            className: 'btn btn-sm btn-label-info ms-3 mt-3 mb-0',
            action: function () {
              getSystemStat (); 
            }
          }
        ],
        responsive: {
          details: {
            display: $.fn.dataTable.Responsive.display.modal({
              header: function (row) {
                var data = row.data();
                return 'Details of ' + data['full_name'];
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
        initComplete: function () {
          setInterval(()=>{
            getSystemStat()
          },30000)
        }
      });

    }

})();

function restartService(data) {
  // Show confirmation alert before proceeding
  Swal.fire({
    title: 'Are you sure?',
    text: "Do you really want to restart the service?",
    icon: 'warning',
    showCancelButton: true,
    confirmButtonColor: '#3085d6',
    cancelButtonColor: '#d33',
    confirmButtonText: 'Yes, restart it!'
  }).then((result) => {
    if (result.isConfirmed) {
      // User confirmed, proceed with fetch operation
      fetch(webPath + 'tab-settings/systems/services/restart?s=' + data)
        .then(response => {
          // Check if the response is OK
          if (!response.ok) {
            throw new Error('Network response was not ok');
          }
          return response.text();
        })
        .then(responseData => {
          // Print the response data
          console.log(responseData);
          // Show success alert
          Swal.fire(
            'Restarted!',
            'The service has been restarted.',
            'success'
          );
        })
        .catch(error => {
          // Handle any errors that occurred during fetch
          console.error('There has been a problem with your fetch operation:', error);
          // Show error alert
          Swal.fire(
            'Error!',
            'There was a problem restarting the service.',
            'error'
          );
        });
    }
  });
}


function getSystemStat (){
  fetch(webPath + 'tab-settings/systems/stat')
    .then(response => {
      // Memastikan respons dari server dalam format teks
      if (!response.ok) {
        throw new Error('Network response was not ok');
      }
      return response.text();
    })
    .then(data => {
      // Mencetak hasil teks yang didapat dari URL
      document.getElementById("system-log").innerText = data
    })
    .catch(error => {
      // Menangani kesalahan jika fetch gagal
      console.error('There has been a problem with your fetch operation:', error);
    });
    $('.dt_system_server_status').DataTable().ajax.reload();
}

