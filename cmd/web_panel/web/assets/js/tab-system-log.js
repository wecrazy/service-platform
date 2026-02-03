/**
 * App eCommerce Category List
 */

'use strict';

// Comment editor
$(function () {
  var log_file = $('#system-log-file');
  if (log_file.length) {

    // Fetch data from the Go Gin endpoint
    $.ajax({
      url: webPath+"tab-system-log/system/log/file",
      type: "GET",
      success: function(response) {
        // Clear existing options
        $("#system-log-file").empty();
        $("#system-log-file").append('<option value="apps.log">apps.log</option>');
        // Add new options based on the response
        response.data.forEach(function(file) {
          if(file !== "apps.log"){
            $("#system-log-file").append('<option value="' + file + '" disabled>' + file + '</option>');
          }
        });
        
        // Trigger select2 to update
        $("#system-log-file").trigger('change');
      },
      error: function(xhr, status, error) {
        console.error("Error fetching data:", error);
      }
    });
  }

  // Variable declaration for category list table
  var dt_category_list_table = $('.system-log-list');
  if (dt_category_list_table.length) {
    var dt_category = dt_category_list_table.DataTable({
      lengthMenu: [10, 25, 50, 100, 200, 500, 1000], 
			pageLength: 50, // Set the default page length
      scrollX:true,
      ajax: webPath+'tab-system-log/table?v='+$("#system-log-file").val(), // JSON file to add data
      columns: [
        { data: 'l' }
      ],
      columnDefs: [
        {
          targets: 0,
          render: function (data, type, full, meta) {
            return full['l'];
          }
        }
      ],
      order: [[0, 'desc']],
      dom:
        '<"card-header d-flex flex-wrap py-0"' +
        '<"me-5 ms-n2 pe-5"f>' +
        '<"d-flex justify-content-start justify-content-md-end align-items-baseline"<"dt-action-buttons d-flex align-items-start align-items-md-center justify-content-sm-center mb-3 mb-sm-0 gap-3"lB>>' +
        '>t' +
        '<"row mx-2"' +
        '<"col-sm-12 col-md-6"i>' +
        '<"col-sm-12 col-md-6"p>' +
        '>',
      lengthMenu: [10, 20, 50, 70, 100], //for length of menu
      language: {
        sLengthMenu: '_MENU_',
        search: '',
        searchPlaceholder: 'Search Log'
      },
      buttons: [
        {
          extend: 'collection',
          className: 'btn btn-label-secondary dropdown-toggle ms-2 me-3',
          text: '<i class="bx bx-export me-1"></i>Export',
          buttons: [
            {
              text: '<i class="bx bx-data"></i> Log File',
              className: 'dropdown-item',
              action: function (e, dt, button, config) {
                fetch(webPath+'tab-system-log/table.csv?v='+$("#system-log-file").val(), {
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
                  const filename = `System_${$("#system-log-file").val()}_${timestampnow}.csv`;
              
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
  var select2 = $('.select2');
  
  if (select2.length) {
    select2.each(function () {
      var $this = $(this);
      $this.wrap('<div class="position-relative mx-0 h3"></div>').select2({
        placeholder: 'Select value',
        dropdownParent: $this.parent()
      });
    });
    var select2selection = $('.select2-selection');
    select2selection.each(function () {
      var $this = $(this);
      $this.addClass('border-0');
    });

  }
  $("#system-log-file").change(function() {
    // Get the selected option value
    var selectedValue = $(this).val();
    // Perform any actions based on the selected value
    console.log("Selected value:", selectedValue);
    $('.system-log-list').DataTable().ajax.reload();
  });

});
