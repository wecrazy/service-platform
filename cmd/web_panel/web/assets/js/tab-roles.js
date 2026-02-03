/**
 * App user list
 */

'use strict';

// Datatable (jquery)
$(function () {
  
  var dtUserTableRoles = $('.datatables-users-roles');

  // Users List datatable
  if (dtUserTableRoles.length) {
    dtUserTableRoles.DataTable({
      lengthMenu: [10, 25, 50, 100, 200, 500, 1000], 
      pageLength: 50,
      ajax: webPath+'tab-roles/admins/table',//assetsPath + 'json/user-list.json', // JSON file to add data
      columns: [
        // columns according to JSON
        { data: '' },
        { data: 'full_name' },
        { data: 'username' },
        { data: 'phone' },
        { data: 'role' },
        { data: 'status' },
        { data: '' }
      ],
      columnDefs: [
        {
          // For Responsive
          className: 'control',
          orderable: false,
          searchable: false,
          responsivePriority: 2,
          targets: 0,
          render: function (data, type, full, meta) {
            return '';
          }
        },
        {
          // User full name and email
          targets: 1,
          responsivePriority: 1,
          render: function (data, type, full, meta) {
            var editable = ""
            if(full["updating"] == "1"){
              editable = "editable"
            }
            var $name = full['full_name'],
              $email = full['email'],
              $image = full['avatar'];
            if ($image) {
              // For Avatar image
              var $output =
                '<img src="'+ $image + '" alt="Avatar" class="avatar-image rounded-circle">';
            } else {
              // For Avatar badge
              var stateNum = Math.floor(Math.random() * 6) + 1;
              var states = ['success', 'danger', 'warning', 'info', 'dark', 'primary', 'secondary'];
              var $state = states[stateNum],
                $name = full['full_name'],
                $initials = $name.match(/\b\w/g) || [];
              $initials = (($initials.shift() || '') + ($initials.pop() || '')).toUpperCase();
              $output = '<span class="avatar-initial rounded-circle bg-label-' + $state + '">' + $initials + '</span>';
            }
            const parts = meta.settings.ajax.split('/');
            // Creates full output for row
            var $row_output =
              '<div class="d-flex justify-content-left align-items-center">' +
              '<div class="avatar-wrapper">' +
              '<div class="avatar avatar-sm me-3">' +
              $output +
              '</div>' +
              '</div>' +
              '<div class="d-flex flex-column">' +
              '<p class="'+editable+' mb-0" patch="'+webPath+'tab-roles/${parts[parts.length - 2]}" field="fullname" point="'+full['id']+'" >'+$name+'</p>'+
              '<small class="text-muted '+editable+'" patch="'+webPath+'tab-roles/${parts[parts.length - 2]}" field="email" point="'+full['id']+'" >' +
              $email +
              '</small>' +
              '</div>' +
              '</div>';
            return $row_output;
          }
        },
        {
          // User full name and email
          targets: 3,
          responsivePriority: 1,
          render: function (data, type, full, meta) {
            var editable = ""
            if(full["updating"] == "1"){
              editable = "editable"
            }
            const parts = meta.settings.ajax.split('/');
            return ('<p class="'+editable+` mb-0" patch="${webPath}tab-roles/${parts[parts.length - 2]}" field="phone" point="`+full['id']+'" >'+full['phone']+'</p>');
          }
        },
        {
          // User Role
          targets: 4,
          render: function (data, type, full, meta) {
            var selectablechoice = ""
            if(full["updating"] == "1"){
              selectablechoice = "selectable-choice"
            }
            var $role = full['role_id'];
            var $roles = full['roles'];
            var title = "Unknown"
            var classname = "btn-label-dark"
            var choices = ""

            for (let i = 0; i < $roles.length; i++) {
              if ($role == $roles[i].id){
                title = $roles[i].title
                classname = $roles[i].class_name
              }
              choices += $roles[i].title
              choices += (i!=($roles.length-1))?",":"";
            }
            const parts = meta.settings.ajax.split('/');
            return (`<p class="${classname} ${selectablechoice}" tab="${meta.settings.sTableId}" patch="${webPath}tab-roles/${parts[parts.length - 2]}" field="role" point="${full['id']}" origin="${title}" choices="${choices}">${title}</p>`)
          }
        },
        {
          // User Status
          targets: 5,
          render: function (data, type, full, meta) {
            var selectablechoice = ""
            if(full["updating"] == "1"){
              selectablechoice = "selectable-choice"
            }
            var $status = full['status'];
            var $statuses = full['statuses'];
            var title = "Unknown"
            var classname = "btn-label-dark"
            var choices = ""

            for (let i = 0; i < $statuses.length; i++) {
              if ($status == $statuses[i].id){
                title = $statuses[i].title
                classname = $statuses[i].class_name
              }
              choices += $statuses[i].title
              choices += (i!=($statuses.length-1))?",":"";
            }
            const parts = meta.settings.ajax.split('/');
            return (`<p class="${classname} ${selectablechoice}" tab="${meta.settings.sTableId}" patch="${webPath}tab-roles/${parts[parts.length - 2]}" field="status" point="${full['id']}" origin="${title}" choices="${choices}">${title}</p>`)
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
                    '<button class="btn dropdown-item text-danger deleteable" tab="'+tableId+'" delete="'+webPath+'tab-roles/'+parts[parts.length - 2]+'/'+full['id']+'">Remove</button>' +
                  '</div>' +
                '</div>' +
              '</div>'
              );
          }
        }
      ],
      order: [[1, 'desc']],
      dom:
        '<"row mx-2"' +
        '<"col-sm-12 col-md-1 col-lg-1" l>' +
        // '<"col-sm-12 col-md-4 col-lg-4 d-flex align-items-center justify-content-md-end justify-content-center align-items-center"B>' +
        '<"col-sm-12 col-md-11 col-lg-11"<"dt-action-buttons text-xl-end text-lg-start text-md-end text-start d-flex align-items-center justify-content-md-end justify-content-center align-items-center flex-sm-nowrap flex-wrap me-1"<"me-3"f><"user_role w-px-200 pb-3 pb-sm-0">B>>'+
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search User..'
      },
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle ms-2 me-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              extend: 'csv',
              text: '<i class="bx bx-file me-2" ></i>Csv',
              className: 'dropdown-item',
              filename: `Admin_${timestampnow}`,
              exportOptions: {
                columns: [1, 2, 3, 4, 5],
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
          ]
        },
        {
          text: '<i class="bx bxs-user-plus"></i><span class="d-none d-sm-inline-block"></span>',
          className: 'add-new btn btn-outline-info py-1 px-2',
          attr: {
            'data-bs-toggle': 'offcanvas',
            'data-bs-target': '#offcanvasAddUser'
          }
        },
      ],
      // For responsive popup
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
            setTimeout(()=>{
              enableEditableCells();
            },300)
            return data ? $('<table class="table"/><tbody />').append(data) : false;
          }
        }
      },
      initComplete: function () {
        // Adding role filter once table initialized
        this.api()
          .columns(4)
          .every(function () {
            var column = this;
            var select = $(
              '<select id="UserRole" class="form-select text-capitalize"><option value=""> All Role </option></select>'
            )
              .appendTo('.user_role')
              .on('change', function () {
                var val = $.fn.dataTable.util.escapeRegex($(this).val());
                column.search(val ? '^' + val + '$' : '', true, false).draw();
              });

            column
              .data()
              .unique()
              .sort()
              .each(function (d, j) {
                select.append('<option value="' + d + '" class="text-capitalize">' + d + '</option>');
              });
          });
          // Add editable functionality after the table is initialized
          enableEditableCells();
      }
    }).on('draw.dt', function() {
      enableEditableCells();
    });
  }

  // Filter form control to default size
  // ? setTimeout used for multilingual table initialization
  setTimeout(() => {
    $('.dataTables_filter .form-control').removeClass('form-control-sm');
    $('.dataTables_length .form-select').removeClass('form-select-sm');
  }, 300);
});

(function () {
  // On edit role click, update text
  var roleEditList = document.querySelectorAll('.role-edit-modal'),
    roleAdd = document.querySelector('.add-new-role'),
    roleTitle = document.querySelector('.role-title');

  // roleAdd.onclick = function () {
  //   roleTitle.innerHTML = 'Add New Role'; // reset text
  // };
  if (roleEditList) {
    roleEditList.forEach(function (roleEditEl) {
      roleEditEl.onclick = function () {
        // var uuid = roleEditEl.getAttribute('uuid');
        // console.log('ajsbka')
        // console.log(uuid)
        
        roleTitle.innerHTML = 'Edit Role'; // reset text
      };
    });
  }
})();
