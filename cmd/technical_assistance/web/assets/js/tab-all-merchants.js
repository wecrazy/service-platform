/**
 * Page User List
 */

'use strict';

// Datatable (jquery)
$(function () {
  let borderColor, bodyBg, headingColor;
  if (isDarkStyle) {
    borderColor = config.colors_dark.borderColor;
    bodyBg = config.colors_dark.bodyBg;
    headingColor = config.colors_dark.headingColor;
  } else {
    borderColor = config.colors.borderColor;
    bodyBg = config.colors.bodyBg;
    headingColor = config.colors.headingColor;
  }

  // Variable declaration for table
  var dt_user_table = $('.datatables-merchant-list'),
    select2 = $('.select2'),
    userView = 'app-user-view-account.html',
    statusObj = {
      2: { title: 'Pending', class: 'bg-label-warning' },
      1: { title: 'Active', class: 'bg-label-success' },
      0: { title: 'Inactive', class: 'bg-label-secondary' }
    };

  // Users datatable
  if (dt_user_table.length) {
    var dt_user = dt_user_table.DataTable({
      lengthMenu: [10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 50, // Set the default page length
      // ajax: webPath+'tab-all-merchants/table',
      serverSide: true,
      ajax: {
				url: webPath+'tab-all-merchants/table',
				type: 'POST',
      },
      columns: [
        { data:"no"},                  
        { data:"no"},
        { data:"registration_name"},
        { data:"main_registration_name"},
        { data:"business_contact_name"},
        { data:"business_contact_mobile_no"},
        { data:"business_contact_email"},
        { data:"business_contact_tel"},
        { data:"technical_contact_name"},
        { data:"technical_contact_mobile_no"},
        { data:"technical_contact_email"},
        { data:"technical_contact_tel"},
        { data:"registration_address"},
        { data:"area_code"},
        { data:"category_code"},
        { data:"credit_level"},
        { data:"valid_start_date"},
        { data:"stop_trx_date"},
        { data:"is_monitored"},
        { data:"amount_type"},
        { data:"is_credit_checked"},
        { data:"business_start_time"},
        { data:"business_end_time"},
        { data:"pay_cycle"},
        { data:"status"},
        { data:"card_type"},
        { data:"create_at"},
        { data:"update_at"},
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
        // {
        //   // User full name and email
        //   targets: 1,
        //   responsivePriority: 4,
        //   render: function (data, type, full, meta) {
        //     var $name = full['registration_name'],
        //       $email = full['business_contact_email'],
        //       $image = full['avatar'];
        //     if ($image) {
        //       // For Avatar image
        //       var $output =
        //         '<img src="' + assetsPath + 'img/avatars/' + $image + '" alt="Avatar" class="rounded-circle">';
        //     } else {
        //       // For Avatar badge
        //       var stateNum = Math.floor(Math.random() * 6);
        //       var states = ['success', 'danger', 'warning', 'info', 'dark', 'primary', 'secondary'];
        //       var $state = states[stateNum],
        //         $name = full['full_name'],
        //         $initials = $name.match(/\b\w/g) || [];
        //       $initials = (($initials.shift() || '') + ($initials.pop() || '')).toUpperCase();
        //       $output = '<span class="avatar-initial rounded-circle bg-label-' + $state + '">' + $initials + '</span>';
        //       // $('#mainAvatar').html($output);
        //       // $('.avatar img').attr('src', $output);
        //       // $('.avatar').html($output);
        //     }
        //     // Creates full output for row
        //     var $row_output =
        //       '<div class="d-flex justify-content-start align-items-center user-name">' +
        //       '<div class="avatar-wrapper">' +
        //       '<div class="avatar avatar-sm me-3">' +
        //       $output +
        //       '</div>' +
        //       '</div>' +
        //       '<div class="d-flex flex-column">' +
        //       '<span class="text-body text-truncate"><span class="fw-medium">' +
        //       $name +
        //       '</span></span>' +
        //       '<small class="text-muted">' +
        //       $email +
        //       '</small>' +
        //       '</div>' +
        //       '</div>';
        //     return $row_output;
        //   }
        // },
        {
          // Actions
          targets: -1,
          title: 'Actions',
          searchable: false,
          orderable: false,
          render: function (data, type, full, meta) {
            return (
              '<div class="d-inline-block text-nowrap">' +
              '<div class="dropdown">' +
              '<button class="btn btn-sm btn-icon dropdown-toggle hide-arrow" type="button" id="actionsDropdown" data-bs-toggle="dropdown" aria-haspopup="true" aria-expanded="false">' +
              '<i class="bx bx-dots-vertical-rounded me-2"></i>' +
              '</button>' +
              '<div class="dropdown-menu dropdown-menu-end" aria-labelledby="actionsDropdown">' +
              '<a class="dropdown-item" href="#">Disable</a>' +
              '<a class="dropdown-item" href="#">Remove</a>' +
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
          className: 'btn btn-label-secondary dropdown-toggle ms-2 me-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'print',
              text: '<i class="bx bx-printer me-2" ></i>Print',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7],
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
              filename: `Merchant_${timestampnow}`,
              text: '<i class="bx bx-file me-2" ></i>Csv',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7],
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
              filename: `Merchant_${timestampnow}`,
              text: '<i class="bx bxs-file-export me-2"></i>Excel',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7],
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
              filename: `Merchant_${timestampnow}`,
              text: '<i class="bx bxs-file-pdf me-2"></i>Pdf',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7],
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
              text: '<i class="bx bx-copy me-2" ></i>Copy',
              className: 'dropdown-item',
              exportOptions: {
                columns: [1, 2, 3, 4, 5, 6, 7],
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
                fetch(webPath + 'tab-all-merchants/table.csv', {
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
                  const filename = `Merchant_Report_${timestampnow}.csv`;
              
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
                fetch(webPath + 'tab-all-merchants/table2.csv', {
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
                  const filename = `All_Simple_Merchant_Report_${timestampnow}.csv`;
              
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
      // initComplete: function () {
      //   // Adding role filter once table initialized
      //   this.api()
      //     .columns(2)
      //     .every(function () {
      //       var column = this;
      //       var select = $(
      //         '<select id="UserRole" class="form-select text-capitalize"><option value=""> Select Role </option></select>'
      //       )
      //         .appendTo('.user_role')
      //         .on('change', function () {
      //           var val = $.fn.dataTable.util.escapeRegex($(this).val());
      //           column.search(val ? '^' + val + '$' : '', true, false).draw();
      //         });

      //       column
      //         .data()
      //         .unique()
      //         .sort()
      //         .each(function (d, j) {
      //           select.append('<option value="' + d + '">' + d + '</option>');
      //         });
      //     });
      // }
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

// Validation & Phone mask
(function () {
  const phoneMaskList = document.querySelectorAll('.phone-mask'),
    addNewUserForm = document.getElementById('addNewUserForm');

  // Phone Number
  if (phoneMaskList) {
    phoneMaskList.forEach(function (phoneMask) {
      new Cleave(phoneMask, {
        phone: true,
        phoneRegionCode: 'US'
      });
    });
  }
  // Add New User Form Validation
  const fv = FormValidation.formValidation(addNewUserForm, {
    fields: {
      userFullname: {
        validators: {
          notEmpty: {
            message: 'Please enter fullname '
          }
        }
      },
      userEmail: {
        validators: {
          notEmpty: {
            message: 'Please enter your email'
          },
          emailAddress: {
            message: 'The value is not a valid email address'
          }
        }
      }
    },
    plugins: {
      trigger: new FormValidation.plugins.Trigger(),
      bootstrap5: new FormValidation.plugins.Bootstrap5({
        // Use this for enabling/changing valid/invalid class
        eleValidClass: '',
        rowSelector: function (field, ele) {
          // field is the field name & ele is the field element
          return '.mb-3';
        }
      }),
      submitButton: new FormValidation.plugins.SubmitButton(),
      // Submit the form when all fields are valid
      // defaultSubmit: new FormValidation.plugins.DefaultSubmit(),
      autoFocus: new FormValidation.plugins.AutoFocus()
    }
  });
})();
