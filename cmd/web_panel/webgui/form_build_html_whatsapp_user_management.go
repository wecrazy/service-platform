package webgui

import (
	"fmt"
	"html/template"
)

func booleanEditFormWhatsappUserManagement(colHeader template.HTML, edit_id, colData, yesNoOptionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
			<label class="form-label">%s</label>
			<select
				id="%s"
				name="%s"
				class="form-select"
				data-column="%d"
				data-column-index="%d">
				%s
			</select>`, colHeader, edit_id, colData, i, i-1, yesNoOptionsHTML)

	return template.HTML(html)
}

func booleanFilterWhatsappUserManagement(colHeader template.HTML, filter_id, filterOptionsHTML string, i int) template.HTML {
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
		colHeader, filter_id, i, i-1, filterOptionsHTML, filter_id,
	)

	return template.HTML(html)
}

func booleanInsertFieldWhatsappUserManagement(colHeader template.HTML, insert_id, colData, yesNoOptionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
			<select
				id="%s"
				name="%s"
				class="form-select"
				data-column="%d"
				data-column-index="%d">
				%s
			</select>`, colHeader, insert_id, colData, i, i-1, yesNoOptionsHTML)

	return template.HTML(html)
}

func waUserTypeFilterWhatsappUserManagement(colHeader template.HTML, filter_id, optionsHTML string, i int) template.HTML {
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
		colHeader, filter_id, i, i-1, optionsHTML, filter_id)

	return template.HTML(html)
}

func waUserTypeEditFormWhatsappUserManagement(colHeader template.HTML, edit_id, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
		<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
			<option value="">Select</option>
			%s
		</select>`,
		colHeader, edit_id, colData, i, i-1, optionsHTML)

	return template.HTML(html)
}

func waUserTypeInsertFieldWhatsappUserManagement(colHeader template.HTML, insert_id, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
		<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
			%s
		</select>`,
		colHeader, insert_id, colData, i, i-1, optionsHTML)
	return template.HTML(html)
}

// Filter: multi-select filter using select2 (with regex search)
func allowedTypesFilterWhatsappUserManagement(colHeader template.HTML, filter_id, optionsHTML string, i int) template.HTML {
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
		colHeader, filter_id, i, i-1, optionsHTML, filter_id, i, i)
	return template.HTML(html)
}

// Edit form: multi-select, correct name="%s[]"
func allowedTypesEditFormWhatsappUserManagement(colHeader template.HTML, edit_id, colData, optionsHTML string, i int) template.HTML {
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
		edit_id,     // id of select
		colData,     // name attribute
		i,           // data-column
		i-1,         // data-column-index (adjust if needed)
		optionsHTML, // your <option> list
		edit_id)     // id used in JS)
	return template.HTML(html)
}

// Insert form: multi-select, correct name="%s[]"
func allowedTypesInsertFieldWhatsappUserManagement(colHeader template.HTML, insert_id, colData, optionsHTML string, i int) template.HTML {
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
		colHeader, insert_id, colData, i, i-1, optionsHTML, insert_id)
	return template.HTML(html)
}

func allowedChatFilterWhatsappUserManagement(colHeader template.HTML, filter_id, optionsHTML string, i int) template.HTML {
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
		colHeader, filter_id, i, i-1, optionsHTML, filter_id)
	return template.HTML(html)
}

func allowedChatEditFormWhatsappUserManagement(colHeader template.HTML, edit_id, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s</label>
				<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
					<option value="" disabled>--- Select an option ---</option>
					%s
				</select>`,
		colHeader, edit_id, colData, i, i-1, optionsHTML)
	return template.HTML(html)
}

func allowedChatInsertFieldWhatsappUserManagement(colHeader template.HTML, insert_id, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
					%s
				</select>`,
		colHeader, insert_id, colData, i, i-1, optionsHTML)
	return template.HTML(html)
}

func userOfFilterWhatsappUserManagement(colHeader template.HTML, filter_id, optionsHTML string, i int) template.HTML {
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
		colHeader, filter_id, i, i-1, optionsHTML, filter_id)
	return template.HTML(html)
}

func userOfEditFormWhatsappUserManagement(colHeader template.HTML, edit_id, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s</label>
				<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
					<option value="" disabled>--- Select an option ---</option>
					%s
				</select>`,
		colHeader, edit_id, colData, i, i-1, optionsHTML)
	return template.HTML(html)
}

func userOfInsertFieldWhatsappUserManagement(colHeader template.HTML, insert_id, colData, optionsHTML string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<select id="%s" name="%s" class="form-select" data-column="%d" data-column-index="%d">
					%s
				</select>`,
		colHeader, insert_id, colData, i, i-1, optionsHTML)
	return template.HTML(html)
}
