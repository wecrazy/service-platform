package webgui

import (
	"fmt"
	"html/template"
)

// booleanEditFormWhatsappUserManagement generates HTML for a boolean field in the edit form of the WhatsApp User Management DataTable. It creates a select dropdown with "Yes" and "No" options, allowing users to edit boolean values in a user-friendly way. The function takes parameters for the column header, element ID, column data name, options HTML, and column index to properly configure the select element and its associated label.
func booleanEditFormWhatsappUserManagement(colHeader template.HTML, editID, colData, yesNoOptionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
			<label class="form-label">%s</label>
			<select
				id="%s"
				name="%s"
				class="form-select"
				data-column="%d"
				data-column-index="%d">
				%s
			</select>`, colHeader, editID, colData, i, i-1, yesNoOptionsHTML)

	return template.HTML(html)
}

// booleanInsertFieldWhatsappUserManagement generates HTML for a boolean field in the insert form of the WhatsApp User Management DataTable. Similar to the edit form, it creates a select dropdown with "Yes" and "No" options for users to input boolean values when adding new entries. The function takes parameters for the column header, element ID, column data name, options HTML, and column index to properly configure the select element and its associated label.
func booleanFilterWhatsappUserManagement(colHeader template.HTML, filterID, filterOptionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
			<select
				id="%s"
				class="form-control dt-input"
				data-column="%d"
				data-column-index="%d">
				%s
			</select>

			<script>
				$('#%s').on('change', function() {
					var columnIdx = $(this).data('column');
					var value = $(this).val();

					// Try to find the first DataTable on the page
					var table = $('.dataTable').DataTable();

					if (value === '') {
						table.column(columnIdx).search('').draw();
					} else {
						// Exact match true/false
						table.column(columnIdx).search('^' + value + '$', true, false).draw();
					}
				});
			</script>`,
		colHeader, filterID, i, i-1, filterOptionsHTML, filterID,
	)

	return template.HTML(html)
}

// booleanInsertFieldWhatsappUserManagement generates HTML for a boolean field in the insert form of the WhatsApp User Management DataTable. Similar to the edit form, it creates a select dropdown with "Yes" and "No" options for users to input boolean values when adding new entries. The function takes parameters for the column header, element ID, column data name, options HTML, and column index to properly configure the select element and its associated label.
func booleanInsertFieldWhatsappUserManagement(colHeader template.HTML, insertID, colData, yesNoOptionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
			<select
				id="%s"
				name="%s"
				class="form-select"
				data-column="%d"
				data-column-index="%d">
				%s
			</select>`, colHeader, insertID, colData, i, i-1, yesNoOptionsHTML)

	return template.HTML(html)
}

// waUserTypeFilterWhatsappUserManagement generates HTML for a user type filter in the WhatsApp User Management DataTable. It creates a select dropdown with options for different user types, allowing users to filter the table based on the selected type. The function takes parameters for the column header, element ID, options HTML, and column index to properly configure the select element and its associated label.
func waUserTypeFilterWhatsappUserManagement(colHeader template.HTML, filterID, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
		<label class="form-label">%s:</label>
		<select id="%s" class="form-control dt-input" data-column="%d" data-column-index="%d">
			<option value="">All</option>
			%s
		</select>
		<script>
			$('#%s').on('change', function() {
				var columnIdx = $(this).data('column');
				var value = $(this).val();
				var table = $('.dataTable').DataTable();
				if (value === '') {
					table.column(columnIdx).search('').draw();
				} else {
					table.column(columnIdx).search('^' + value + '$', true, false).draw();
				}
			});
		</script>`,
		colHeader, filterID, i, i-1, optionsHTML, filterID)

	return template.HTML(html)
}

// waUserTypeEditFormWhatsappUserManagement generates HTML for a user type field in the edit form of the WhatsApp User Management DataTable. It creates a select dropdown with options for different user types, allowing users to edit the type of a user in a user-friendly way. The function takes parameters for the column header, element ID, column data name, options HTML, and column index to properly configure the select element and its associated label.
func waUserTypeEditFormWhatsappUserManagement(colHeader template.HTML, editID, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
		<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
			<option value="">Select</option>
			%s
		</select>`,
		colHeader, editID, colData, i, i-1, optionsHTML)

	return template.HTML(html)
}

// waUserTypeInsertFieldWhatsappUserManagement generates HTML for a user type field in the insert form of the WhatsApp User Management DataTable. Similar to the edit form, it creates a select dropdown with options for different user types, allowing users to select the type of a user when adding new entries. The function takes parameters for the column header, element ID, column data name, options HTML, and column index to properly configure the select element and its associated label.
func waUserTypeInsertFieldWhatsappUserManagement(colHeader template.HTML, insertID, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
		<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
			%s
		</select>`,
		colHeader, insertID, colData, i, i-1, optionsHTML)
	return template.HTML(html)
}

// allowedTypesFilterWhatsappUserManagement generates HTML for a multi-select filter for allowed types in the WhatsApp User Management DataTable. It creates a select element with the "multiple" attribute, allowing users to select multiple types to filter the table. The function includes JavaScript to handle the change event and apply the appropriate filtering logic to the DataTable based on the selected options.
func allowedTypesFilterWhatsappUserManagement(colHeader template.HTML, filterID, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
	<select id="%s" multiple class="form-select dt-input" data-column="%d" data-column-index="%d">
		%s
	</select>
	<script>
	(function(){
		var el = $('#%s');
		if (el.length) {
			el.select2({
				placeholder: "Filter types",
				allowClear: true,
				width: 'resolve'
			});
			el.on('change', function() {
				var table = $('.dataTable').DataTable();
				var selected = $(this).val();
				if (!selected || selected.length === 0) {
					table.column(%d).search('').draw();
				} else {
					var regex = selected.join('|'); // text|image|video
					table.column(%d).search(regex, true, false).draw();
				}
			});
		}
	})();
	</script>`,
		colHeader, filterID, i, i-1, optionsHTML, filterID, i, i)
	return template.HTML(html)
}

// allowedTypesEditFormWhatsappUserManagement generates HTML for a multi-select field for allowed types in the edit form of the WhatsApp User Management DataTable. It creates a select element with the "multiple" attribute, allowing users to select multiple types when editing an entry. The function includes JavaScript to initialize the Select2 plugin for enhanced usability and to pre-select existing values based on a data attribute.
func allowedTypesEditFormWhatsappUserManagement(colHeader template.HTML, editID, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
<label class="form-label">%s</label>
<select id="%s" name="%s[]" multiple class="form-select" data-column="%d" data-column-index="%d">
  %s
</select>
<script>
(function(){
  var el = $('#%s');
  function initSelect2(){
    if (el.length) {
      el.select2({
        placeholder: "Edit types",
        allowClear: true,
        width: 'resolve'
      });

      // Pre-select values if data-value exists
      var existing = el.data('value');
      if (existing) {
        try {
          var selected = JSON.parse(existing);
          el.val(selected).trigger('change');
        } catch(e) {
          console.error('Invalid JSON in data-value:', existing);
        }
      }
    }
  }
  initSelect2();

  // Re-init on DataTable draw (in case table redraws destroy the select2)
  $('.dataTable').on('draw.dt', function(){
    initSelect2();
  });
})();
</script>`,
		colHeader,   // label
		editID,     // id of select
		colData,     // name attribute
		i,           // data-column
		i-1,         // data-column-index (adjust if needed)
		optionsHTML, // your <option> list
		editID)     // id used in JS)
	return template.HTML(html)
}

// allowedTypesInsertFieldWhatsappUserManagement generates HTML for a multi-select field for allowed types in the insert form of the WhatsApp User Management DataTable. It creates a select element with the "multiple" attribute, allowing users to select multiple types when adding a new entry. The function includes JavaScript to initialize the Select2 plugin for enhanced usability.
func allowedTypesInsertFieldWhatsappUserManagement(colHeader template.HTML, insertID, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
	<select id="%s" name="%s[]" multiple class="form-select" data-column="%d" data-column-index="%d">
		%s
	</select>
	<script>
	(function(){
		var el = $('#%s');
		if (el.length) {
			el.select2({
				placeholder: "Select types to insert",
				allowClear: true,
				width: 'resolve'
			});
		}
	})();
	</script>`,
		colHeader, insertID, colData, i, i-1, optionsHTML, insertID)
	return template.HTML(html)
}

// allowedChatFilterWhatsappUserManagement generates HTML for a chat filter in the WhatsApp User Management DataTable. It creates a select dropdown with options for different chat types, allowing users to filter the table based on the selected chat type. The function takes parameters for the column header, element ID, options HTML, and column index to properly configure the select element and its associated label.
func allowedChatFilterWhatsappUserManagement(colHeader template.HTML, filterID, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<select id="%s" class="form-control dt-input" data-column="%d" data-column-index="%d">
					<option value="">All</option>
					%s
				</select>
				<script>
					$('#%s').on('change', function() {
						var columnIdx = $(this).data('column');
						var value = $(this).val();
						var table = $('.dataTable').DataTable();
						if (value === '') {
							table.column(columnIdx).search('').draw();
						} else {
							table.column(columnIdx).search('^' + value + '$', true, false).draw();
						}
					});
				</script>`,
		colHeader, filterID, i, i-1, optionsHTML, filterID)
	return template.HTML(html)
}

// allowedChatEditFormWhatsappUserManagement generates HTML for a chat field in the edit form of the WhatsApp User Management DataTable. It creates a select dropdown with options for different chat types, allowing users to edit the chat type of an entry in a user-friendly way. The function takes parameters for the column header, element ID, column data name, options HTML, and column index to properly configure the select element and its associated label.
func allowedChatEditFormWhatsappUserManagement(colHeader template.HTML, editID, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s</label>
				<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
					<option value="" disabled>--- Select an option ---</option>
					%s
				</select>`,
		colHeader, editID, colData, i, i-1, optionsHTML)
	return template.HTML(html)
}

// allowedChatInsertFieldWhatsappUserManagement generates HTML for a chat field in the insert form of the WhatsApp User Management DataTable. Similar to the edit form, it creates a select dropdown with options for different chat types, allowing users to select the chat type when adding new entries. The function takes parameters for the column header, element ID, column data name, options HTML, and
func allowedChatInsertFieldWhatsappUserManagement(colHeader template.HTML, insertID, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
					%s
				</select>`,
		colHeader, insertID, colData, i, i-1, optionsHTML)
	return template.HTML(html)
}

// userOfFilterWhatsappUserManagement generates HTML for a user filter in the WhatsApp User Management DataTable. It creates a select dropdown with options for different users, allowing users to filter the table based on the selected user. The function takes parameters for the column header, element ID, options HTML, and column index to properly configure the select element and its associated label.
func userOfFilterWhatsappUserManagement(colHeader template.HTML, filterID, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<select id="%s" class="form-control dt-input" data-column="%d" data-column-index="%d">
					<option value="">All</option>
					%s
				</select>
				<script>
					$('#%s').on('change', function() {
						var columnIdx = $(this).data('column');
						var value = $(this).val();
						var table = $('.dataTable').DataTable();
						if (value === '') {
							table.column(columnIdx).search('').draw();
						} else {
							table.column(columnIdx).search('^' + value + '$', true, false).draw();
						}
					});
				</script>`,
		colHeader, filterID, i, i-1, optionsHTML, filterID)
	return template.HTML(html)
}

// userOfEditFormWhatsappUserManagement generates HTML for a user field in the edit form of the WhatsApp User Management DataTable. It creates a select dropdown with options for different users, allowing users to edit the associated user of an entry in a user-friendly way. The function takes parameters for the column header, element ID, column data name, options HTML, and column index to properly configure the select element and its associated label.
func userOfEditFormWhatsappUserManagement(colHeader template.HTML, editID, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s</label>
				<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
					<option value="" disabled>--- Select an option ---</option>
					%s
				</select>`,
		colHeader, editID, colData, i, i-1, optionsHTML)
	return template.HTML(html)
}

// userOfInsertFieldWhatsappUserManagement generates HTML for a user field in the insert form of the WhatsApp User Management DataTable. Similar to the edit form, it creates a select dropdown with options for different users, allowing users to select the associated user when adding new entries. The function takes parameters for the column header, element ID, column data name, options HTML, and column index to properly configure the select element and its associated label.
func userOfInsertFieldWhatsappUserManagement(colHeader template.HTML, insertID, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
					%s
				</select>`,
		colHeader, insertID, colData, i, i-1, optionsHTML)
	return template.HTML(html)
}
