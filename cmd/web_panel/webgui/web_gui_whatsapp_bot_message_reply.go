package webgui

import (
	"fmt"
	"html/template"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"

	"gorm.io/gorm"
)

func RenderDataTableServerSideWhatsappBotMessageReply(title, table_name, endpoint string, page_length int, length_menu []int, order []any, table_columns []Column, insertable, editable, deletable, hideHeader, passwordable bool, scrollUpDown, scrollLeftRight bool, exportType []string, db *gorm.DB) template.HTML {
	var column_array []int
	for i, col := range table_columns {
		if col.Visible {
			column_array = append(column_array, i)
		}

		table_columns[i].Filterable = false
		table_columns[i].Insertable = false

		switch col.Type {
		case "string":
			if title == "Whatsapp Bot Message Reply" {

				allowedCols := map[string]bool{
					// "keywords":   true,
					"reply_text": true,
				}

				allowedInsertCols := map[string]bool{
					"language":   true,
					"keywords":   true,
					"reply_text": true,
				}

				if allowedCols[col.Data] {
					table_columns[i].Filterable = true
				}

				if allowedInsertCols[col.Data] {
					table_columns[i].Insertable = true
				}
			}

			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data
			table_columns[i].EditId = edit_id
			if table_columns[i].SelectableSrc != "" {
				table_columns[i].EditForm = template.HTML(fmt.Sprintf(`
				<label class="form-label">%s</label>
				<input
					id="%s"
					type="text"
					class="form-control"
					name="%s"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />

				<script>
				fetch('%s')
					.then(response => response.json())
					.then(data => {
						var prefetchExample = new Bloodhound({
							datumTokenizer: Bloodhound.tokenizers.whitespace,
							queryTokenizer: Bloodhound.tokenizers.whitespace,
							local: data // Use fetched data directly as the suggestion source
						});

						// Function to render default suggestions or search results
						function renderDefaults(q, sync) {
							if (q === '') {
								sync(prefetchExample.all()); // Show all suggestions when the query is empty
							} else {
								prefetchExample.search(q, sync); // Search based on the query
							}
						}

						// Initialize Typeahead on the input field
						$('#%s').typeahead(
							{
								hint: true,
								highlight: true,
								minLength: 0
							},
							{
								name: 'options',
								source: renderDefaults
							}
						);

						// Show all options when the input is focused and empty
						$('#%s').on('focus', function() {
							if (this.value === '') {
								$(this).typeahead('val', ''); // Clear the input to trigger default suggestions
								$(this).typeahead('open'); // Open the dropdown with all suggestions
							}
						});
						// Trigger a function when an option is selected from the dropdown
						$('#%s').on('typeahead:select', function(ev, suggestion) {
							// Perform an action here, e.g., trigger a keyup event, call a function, etc.
							$(this).trigger('keyup'); // Example: Trigger the keyup event
							filterColumn($(this).attr('data-column'), $(this).val()); // Example: Trigger your filtering function
						});
					})
					.catch(error => console.error('Error fetching options data:', error));
				</script>
				  `, col.Header, edit_id, col.Data, i, col.Header, i-1, table_columns[i].SelectableSrc, edit_id, edit_id, edit_id))

				table_columns[i].Filter = template.HTML(fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<input
					id="%s"
					type="text"
					class="form-control dt-input dt-full-name typeahead-input"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />

				<script>
				fetch('%s')
					.then(response => response.json())
					.then(data => {
						var prefetchExample = new Bloodhound({
							datumTokenizer: Bloodhound.tokenizers.whitespace,
							queryTokenizer: Bloodhound.tokenizers.whitespace,
							local: data // Use fetched data directly as the suggestion source
						});

						// Function to render default suggestions or search results
						function renderDefaults(q, sync) {
							if (q === '') {
								sync(prefetchExample.all()); // Show all suggestions when the query is empty
							} else {
								prefetchExample.search(q, sync); // Search based on the query
							}
						}

						// Initialize Typeahead on the input field
						$('#%s').typeahead(
							{
								hint: true,
								highlight: true,
								minLength: 0
							},
							{
								name: 'options',
								source: renderDefaults
							}
						);

						// Show all options when the input is focused and empty
						$('#%s').on('focus', function() {
							if (this.value === '') {
								$(this).typeahead('val', ''); // Clear the input to trigger default suggestions
								$(this).typeahead('open'); // Open the dropdown with all suggestions
							}
						});
						// Trigger a function when an option is selected from the dropdown
						$('#%s').on('typeahead:select', function(ev, suggestion) {
							// Perform an action here, e.g., trigger a keyup event, call a function, etc.
							$(this).trigger('keyup'); // Example: Trigger the keyup event
							filterColumn($(this).attr('data-column'), $(this).val()); // Example: Trigger your filtering function
						});
					})
					.catch(error => console.error('Error fetching options data:', error));
				</script>
				  `, col.Header, filter_id, i, col.Header, i-1, table_columns[i].SelectableSrc, filter_id, filter_id, filter_id))
				table_columns[i].InsertField = template.HTML(fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<input
					id="%s"
					type="text"
					name="%s"
					class="form-control"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />

				<script>
				fetch('%s')
					.then(response => response.json())
					.then(data => {
						var prefetchExample = new Bloodhound({
							datumTokenizer: Bloodhound.tokenizers.whitespace,
							queryTokenizer: Bloodhound.tokenizers.whitespace,
							local: data
						});
						function renderDefaults(q, sync) {
							if (q === '') {
								sync(prefetchExample.all());
							} else {
								prefetchExample.search(q, sync);
							}
						}

						// Initialize Typeahead on the input field
						$('#%s').typeahead(
							{
								hint: true,
								highlight: true,
								minLength: 0
							},
							{
								name: 'options',
								source: renderDefaults
							}
						);

						$('#%s').on('focus', function() {
							if (this.value === '') {
								$(this).typeahead('val', '');
								$(this).typeahead('open');
							}
						});
						$('#%s').on('typeahead:select', function(ev, suggestion) {
							$(this).trigger('keyup');
							filterColumn($(this).attr('data-column'), $(this).val());
						});
					})
					.catch(error => console.error('Error fetching options data:', error));
				</script>
				  `, col.Header, insert_id, col.Data, i, col.Header, i-1, table_columns[i].SelectableSrc, insert_id, insert_id, insert_id))

			} else {
				switch table_columns[i].Data {
				case "language":
					var languages []model.Language
					if err := db.Find(&languages).Error; err != nil {
						fmt.Println("Error fetching TicketType data:", err)
						return template.HTML(fmt.Sprintf("<p>Error fetching TicketType data: %v</p>", err))
					}

					// Build the options HTML string
					optionsHTML := `<option value="" disabled selected hidden>-- Select Language --</option>`
					for _, dbData := range languages {
						optionsHTML += fmt.Sprintf(`<option value="%d">%s</option>`, dbData.ID, dbData.Name)
					}

					table_columns[i].InsertField = template.HTML(fmt.Sprintf(
						`<label class="form-label">%s:</label>
						<select
							id="%s"
							name="%s"
							class="form-control"
							data-column="%d"
							data-column-index="%d"
							required>
							%s
						</select>`,
						col.Header, insert_id, col.Data, i, i-1, optionsHTML))
				case "keywords":
					// InsertField (already provided by you)
					table_columns[i].InsertField = template.HTML(fmt.Sprintf(`
						<label class="form-label">%s:</label>
						<div id="%s_keywords_container">
							<div class="input-group mb-2">
								<input type="text" class="form-control" placeholder="Enter keyword (can contain comma)" data-keyword-input="true" />
								<button class="btn btn-outline-primary" type="button" onclick="addKeywordField_%s()">+</button>
							</div>
						</div>
						<input type="hidden" id="%s" name="%s" />

						<script>
						$(document).ready(function () {
							var container = $('#%s_keywords_container');
							var hiddenInput = $('#%s');

							function updateKeywords_%s() {
								var keywords = [];
								container.find('input[data-keyword-input="true"]').each(function() {
									var val = $(this).val().trim();
									if(val !== '') {
										keywords.push(val);
									}
								});
								hiddenInput.val(keywords.join('%s'));
							}

							window.addKeywordField_%s = function () {
								var newGroup = $('<div class="input-group mb-2">'
									+ '<input type="text" class="form-control" placeholder="Enter keyword (can contain comma)" data-keyword-input="true" />'
									+ '<button class="btn btn-outline-danger" type="button">-</button>'
									+ '</div>');
								newGroup.find('input').on('input', updateKeywords_%s);
								newGroup.find('button').on('click', function () {
									newGroup.remove();
									updateKeywords_%s();
								});
								container.append(newGroup);
								updateKeywords_%s();
							};

							container.on('input', 'input[data-keyword-input="true"]', updateKeywords_%s);

							// Ensure keywords are updated on form submit
							container.closest('form').on('submit', function() {
								updateKeywords_%s();
							});
						});
						</script>
					`,
						col.Header, // 1
						insert_id,  // 2
						insert_id,  // 3
						insert_id,  // 4
						col.Data,   // 5
						insert_id,  // 6
						insert_id,  // 7
						insert_id,  // 8
						config.GetConfig().Whatsmeow.KeywordSeparator, // 9 (e.g. "|||")
						insert_id, // 10
						insert_id, // 11
						insert_id, // 12
						insert_id, // 13
						insert_id, // 14
						insert_id, // 15
					))

					// EditForm
					table_columns[i].EditForm = template.HTML(fmt.Sprintf(`
						<label class="form-label">%s</label>
						<div id="%s_keywords_edit_container"></div>
						<input type="hidden" id="%s" name="%s" />

						<script>
						function renderEditKeywords_%s(initialValue) {
							var container = $('#%s_keywords_edit_container');
							var hiddenInput = $('#%s');
							container.empty();

							var sep = '%s';
							var keywords = [];
							if (initialValue) {
								keywords = initialValue.split(sep).map(function(k) { return k.trim(); }).filter(function(k) { return k.length > 0; });
							}

							function updateEditKeywords_%s() {
								var vals = [];
								container.find('input[data-keyword-input="true"]').each(function() {
									var val = $(this).val().trim();
									if(val !== '') {
										vals.push(val);
									}
								});
								hiddenInput.val(vals.join(sep));
							}

							function addEditKeywordField_%s(value) {
								var newGroup = $('<div class="input-group mb-2">'
									+ '<input type="text" class="form-control" placeholder="Enter keyword (can contain comma)" data-keyword-input="true" />'
									+ '<button class="btn btn-outline-danger" type="button">-</button>'
									+ '</div>');
								if (value) newGroup.find('input').val(value);
								newGroup.find('input').on('input', updateEditKeywords_%s);
								newGroup.find('button').on('click', function () {
									newGroup.remove();
									updateEditKeywords_%s();
								});
								container.append(newGroup);
							}

							// Add existing keywords
							if (keywords.length > 0) {
								keywords.forEach(function(k) {
									addEditKeywordField_%s(k);
								});
							} else {
								addEditKeywordField_%s('');
							}

							// Add "+" button
							var addBtn = $('<button class="btn btn-outline-primary mb-2" type="button">+</button>');
							addBtn.on('click', function() {
								addEditKeywordField_%s('');
								updateEditKeywords_%s();
							});
							container.append(addBtn);

							container.on('input', 'input[data-keyword-input="true"]', updateEditKeywords_%s);

							// Initial update
							updateEditKeywords_%s();
						}

						$(document).ready(function () {
							var initialValue = $('#%s').val();
							renderEditKeywords_%s(initialValue);

							// If value is changed externally, re-render
							$('#%s').on('change', function() {
								renderEditKeywords_%s($(this).val());
							});
						});
						</script>
					`,
						col.Header, // 1
						edit_id,    // 2
						edit_id,    // 3
						col.Data,   // 4
						edit_id,    // 5 (function suffix)
						edit_id,    // 6 (container)
						edit_id,    // 7 (hidden input)
						config.GetConfig().Whatsmeow.KeywordSeparator, // 8
						edit_id, // 9 (update function)
						edit_id, // 10 (add field function)
						edit_id, // 11 (update function)
						edit_id, // 12 (update function)
						edit_id, // 13 (add field for each keyword)
						edit_id, // 14 (add empty field if none)
						edit_id, // 15 (add "+" button)
						edit_id, // 16 (update on add)
						edit_id, // 17 (update on input)
						edit_id, // 18 (initial update)
						edit_id, // 19 (get initial value)
						edit_id, // 20 (render on ready)
						edit_id, // 21 (on change)
						edit_id, // 22 (render on change)
					))

				default:
					table_columns[i].InsertField = template.HTML(fmt.Sprintf(`
					<label class="form-label">%s:</label>
					<input
					id="%s"
					name="%s"
					type="text"
					class="form-control"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, col.Header, insert_id, col.Data, i, col.Header, i-1))

					table_columns[i].EditForm = template.HTML(fmt.Sprintf(`<label class="form-label">%s</label>
					<input
					id="%s"
					type="text"
					class="form-control"
					name="%s"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, col.Header, edit_id, col.Data, i, col.Header, i-1))

					table_columns[i].Filter = template.HTML(fmt.Sprintf(`<label class="form-label">%s:</label>
					<input
					id="%s"
					type="text"
					class="form-control dt-input dt-full-name"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, col.Header, filter_id, i, col.Header, i-1))
				}
			}

			className := "control"
			returnValue := ""
			if i > 0 {
				className = ""
				if editable {
					if table_columns[i].Editable {
						pass := ""
						if table_columns[i].Passwordable {
							pass = `pass="true"`
						}
						if table_columns[i].SelectableSrc != "" {
							returnValue = `<p class="selectable-suggestion" data-origin="'+extract_data+'" patch="` + endpoint + `" field="` + col.Data + `" select-option="` + string(table_columns[i].SelectableSrc) + `" point="'+full['id']+'" ` + pass + `>'+data+'</p>`
						} else {
							returnValue = `<p class="editable" data-origin="'+extract_data+'" patch="` + endpoint + `" field="` + col.Data + `" point="'+full['id']+'" ` + pass + `>'+data+'</p>`
						}
					} else {
						returnValue = `<p>'+data+'</p>`

					}
				} else {
					returnValue = `<p>'+data+'</p>`
				}
			}

			table_columns[i].ColumnConfig = template.JS(fmt.Sprintf(
				`{
					className: '%s',
					targets: %d,
					visible: %t,
					orderable: %t,
					render: function (data, type, full, meta) {
					var extract_data = extractTxt_HTML(data);
					return '%s';
					}
				},`, className, i, table_columns[i].Visible, table_columns[i].Orderable, returnValue))
		case "image":
			// filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data
			table_columns[i].EditId = edit_id
			// fmt.Println(filter_id)
			table_columns[i].InsertField = template.HTML(fmt.Sprintf(`
			<label class="form-label">%s:</label>
			<input
			  id="%s"
			  name="%s"
			  type="file"
			  class="form-control"
			  data-column="%d"
			  placeholder="Upload %s Image"
			  accept=".jpg, .jpeg, .png"
			  data-column-index="%d" />`, col.Header, insert_id, col.Data, i, col.Header, i-1))

			table_columns[i].Orderable = false
			table_columns[i].Filterable = false
			table_columns[i].Filter = ""

			table_columns[i].EditForm = template.HTML(fmt.Sprintf(`<label class="form-label">%s</label>
			<input
			  id="%s"
			  type="file"
			  class="form-control"
			  name="%s"
			  data-column="%d"
			  placeholder="Upload %s Image"
			  accept=".jpg, .jpeg, .png"
			  data-column-index="%d" />`, col.Header, edit_id, col.Data, i, col.Header, i-1))
			className := "control"
			returnValue := ""
			if i > 0 {
				className = ""
				if editable {
					if table_columns[i].Editable {
						returnValue = `<img src="'+data+'" alt="Image" style="width: 100%%;height:auto;" class="editable-image" data-origin="'+data+'" patch="` + endpoint + `" field="` + col.Data + `" point="'+full['id']+'" /> `
					} else {
						returnValue = `<img src="'+data+'" alt="Image" style="width: 100%% ; height: auto;"/>`

					}
				} else {
					returnValue = `<img src="'+data+'" alt="Image" style="width: 100%% ; height: auto;"/>`
				}
			}

			table_columns[i].ColumnConfig = template.JS(fmt.Sprintf(
				`{
					className: '%s',
					targets: %d,
					visible: %t,
					orderable: %t,
					render: function (data, type, full, meta) {
					return '<div style="width: 50px;height: 50px;overflow: hidden;">%s</div>';
					}
				},`, className, i, table_columns[i].Visible, table_columns[i].Orderable, returnValue))
		case "time.Time", "*time.Time":
			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data
			table_columns[i].EditId = edit_id
			table_columns[i].InsertField = template.HTML(fmt.Sprintf(`
			<label class="form-label">%s:</label>
			<input
			  id="%s"
			  name="%s"
			  type="text"
			  class="form-control flatpickr-datetime"
			  data-column="%d"
			  placeholder="%s YYYY-MM-DD HH:MM"
			  data-column-index="%d" />`, col.Header, insert_id, col.Data, i, col.Header, i-1))

			table_columns[i].EditForm = template.HTML(fmt.Sprintf(`<label class="form-label">%s</label>
			<input
			  id="%s"
			  type="number"
			  class="form-control flatpickr-datetime"
			  name="%s"
			  data-column="%d"
			  placeholder="%s YYYY-MM-DD HH:MM"
			  data-column-index="%d" />`, col.Header, edit_id, col.Data, i, col.Header, i-1))

			table_columns[i].Filter = template.HTML(fmt.Sprintf(`<label class="form-label">%s:</label>
			<div class="mb-0">
			  <input
			  	id="%s"
				type="text"
				class="form-control dt-date flatpickr-range dt-input"
				data-column="%d"
				placeholder="StartDate to EndDate"
				data-column-index="%d"
				name="dt_date" />
			  <input
				type="hidden"
				class="form-control dt-date start_date_%s dt-input"
				data-column="%d"
				data-column-index="%d"
				name="value_from_start_date" />
			  <input
				type="hidden"
				class="form-control dt-date end_date_%s dt-input"
				name="value_from_end_date"
				data-column="%d"
				data-column-index="%d" />
			</div>`, col.Header, filter_id, i, i-1, table_name, i, i-1, table_name, i, i-1))
		case "int", "int8", "int16", "int32", "uint", "int64":
			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data
			table_columns[i].EditId = edit_id
			if table_columns[i].SelectableSrc != "" {
				table_columns[i].EditForm = template.HTML(fmt.Sprintf(`
				<label class="form-label">%s</label>
				<input
					id="%s"
					type="number"
					class="form-control"
					name="%s"
					data-column="%d"
					placeholder="%s Number"
					data-column-index="%d" />

				<script>
				fetch('%s')
					.then(response => response.json())
					.then(data => {
						var prefetchExample = new Bloodhound({
							datumTokenizer: Bloodhound.tokenizers.whitespace,
							queryTokenizer: Bloodhound.tokenizers.whitespace,
							local: data // Use fetched data directly as the suggestion source
						});

						// Function to render default suggestions or search results
						function renderDefaults(q, sync) {
							if (q === '') {
								sync(prefetchExample.all()); // Show all suggestions when the query is empty
							} else {
								prefetchExample.search(q, sync); // Search based on the query
							}
						}

						// Initialize Typeahead on the input field
						$('#%s').typeahead(
							{
								hint: true,
								highlight: true,
								minLength: 0
							},
							{
								name: 'options',
								source: renderDefaults
							}
						);

						// Show all options when the input is focused and empty
						$('#%s').on('focus', function() {
							if (this.value === '') {
								$(this).typeahead('val', ''); // Clear the input to trigger default suggestions
								$(this).typeahead('open'); // Open the dropdown with all suggestions
							}
						});
						// Trigger a function when an option is selected from the dropdown
						$('#%s').on('typeahead:select', function(ev, suggestion) {
							// Perform an action here, e.g., trigger a keyup event, call a function, etc.
							$(this).trigger('keyup'); // Example: Trigger the keyup event
							filterColumn($(this).attr('data-column'), $(this).val()); // Example: Trigger your filtering function
						});
					})
					.catch(error => console.error('Error fetching options data:', error));
				</script>
				  `, col.Header, edit_id, col.Data, i, col.Header, i-1, table_columns[i].SelectableSrc, edit_id, edit_id, edit_id))

				table_columns[i].Filter = template.HTML(fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<input
					id="%s"
					type="number"
					class="form-control dt-input dt-full-name typeahead-input"
					data-column="%d"
					placeholder="%s Number"
					data-column-index="%d" />

				<script>
				fetch('%s')
					.then(response => response.json())
					.then(data => {
						var prefetchExample = new Bloodhound({
							datumTokenizer: Bloodhound.tokenizers.whitespace,
							queryTokenizer: Bloodhound.tokenizers.whitespace,
							local: data // Use fetched data directly as the suggestion source
						});

						// Function to render default suggestions or search results
						function renderDefaults(q, sync) {
							if (q === '') {
								sync(prefetchExample.all()); // Show all suggestions when the query is empty
							} else {
								prefetchExample.search(q, sync); // Search based on the query
							}
						}

						// Initialize Typeahead on the input field
						$('#%s').typeahead(
							{
								hint: true,
								highlight: true,
								minLength: 0
							},
							{
								name: 'options',
								source: renderDefaults
							}
						);

						// Show all options when the input is focused and empty
						$('#%s').on('focus', function() {
							if (this.value === '') {
								$(this).typeahead('val', ''); // Clear the input to trigger default suggestions
								$(this).typeahead('open'); // Open the dropdown with all suggestions
							}
						});
						// Trigger a function when an option is selected from the dropdown
						$('#%s').on('typeahead:select', function(ev, suggestion) {
							// Perform an action here, e.g., trigger a keyup event, call a function, etc.
							$(this).trigger('keyup'); // Example: Trigger the keyup event
							filterColumn($(this).attr('data-column'), $(this).val()); // Example: Trigger your filtering function
						});
					})
					.catch(error => console.error('Error fetching options data:', error));
				</script>
				  `, col.Header, filter_id, i, col.Header, i-1, table_columns[i].SelectableSrc, filter_id, filter_id, filter_id))
				table_columns[i].InsertField = template.HTML(fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<input
					id="%s"
					type="number"
					name="%s"
					class="form-control"
					data-column="%d"
					placeholder="%s Number"
					data-column-index="%d" />

				<script>
				fetch('%s')
					.then(response => response.json())
					.then(data => {
						var prefetchExample = new Bloodhound({
							datumTokenizer: Bloodhound.tokenizers.whitespace,
							queryTokenizer: Bloodhound.tokenizers.whitespace,
							local: data
						});
						function renderDefaults(q, sync) {
							if (q === '') {
								sync(prefetchExample.all());
							} else {
								prefetchExample.search(q, sync);
							}
						}

						// Initialize Typeahead on the input field
						$('#%s').typeahead(
							{
								hint: true,
								highlight: true,
								minLength: 0
							},
							{
								name: 'options',
								source: renderDefaults
							}
						);

						$('#%s').on('focus', function() {
							if (this.value === '') {
								$(this).typeahead('val', '');
								$(this).typeahead('open');
							}
						});
						$('#%s').on('typeahead:select', function(ev, suggestion) {
							$(this).trigger('keyup');
							filterColumn($(this).attr('data-column'), $(this).val());
						});
					})
					.catch(error => console.error('Error fetching options data:', error));
				</script>
				  `, col.Header, insert_id, col.Data, i, col.Header, i-1, table_columns[i].SelectableSrc, insert_id, insert_id, insert_id))

			} else {
				table_columns[i].InsertField = template.HTML(fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<input
				  id="%s"
				  name="%s"
				  type="number"
				  class="form-control"
				  data-column="%d"
				  placeholder="%s number"
				  data-column-index="%d" />`, col.Header, insert_id, col.Data, i, col.Header, i-1))

				table_columns[i].Filter = template.HTML(fmt.Sprintf(`<label class="form-label">%s:</label>
				  <input
					id="%s"
					type="number"
					class="form-control dt-input dt-full-name"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, col.Header, filter_id, i, col.Header, i-1))

				table_columns[i].EditForm = template.HTML(fmt.Sprintf(`<label class="form-label">%s</label>
				<input
				  id="%s"
				  type="number"
				  class="form-control"
				  name="%s"
				  data-column="%d"
				  placeholder="%s Number"
				  data-column-index="%d" />`, col.Header, edit_id, col.Data, i, col.Header, i-1))

			}

			className := "control"
			returnValue := ""
			if i > 0 {
				className = ""
				if editable {
					if table_columns[i].Editable {
						if table_columns[i].SelectableSrc != "" {
							returnValue = `<p class="selectable-suggestion" data-origin="'+data+'" patch="` + endpoint + `" field="` + col.Data + `" select-option="` + string(table_columns[i].SelectableSrc) + `" point="'+full['id']+'" >'+data+'</p>`
						} else {
							returnValue = `<p class="editable" data-origin="'+data+'" patch="` + endpoint + `" field="` + col.Data + `" point="'+full['id']+'" >'+data+'</p>`
						}
					} else {
						returnValue = `<p>'+data+'</p>`

					}
				} else {
					returnValue = `<p>'+data+'</p>`
				}
			}

			table_columns[i].ColumnConfig = template.JS(fmt.Sprintf(
				`{
						className: '%s',
						targets: %d,
						visible: %t,
						orderable: %t,
						render: function (data, type, full, meta) {
						return '%s';
						}
					},`, className, i, table_columns[i].Visible, table_columns[i].Orderable, returnValue))
		case "model.WAUserType":
			allowedCols := map[string]bool{
				"for_user_type": true,
			}
			if allowedCols[col.Data] {
				table_columns[i].Filterable = true
				table_columns[i].Insertable = true
				table_columns[i].Editable = false
			}
			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data

			// Build options
			optionsHTML := ""
			for _, t := range model.AllWAUserTypes {
				optionsHTML += fmt.Sprintf(`<option value="%s">%s</option>`, t, fun.SnakeToCapitalized(string(t)))
			}

			// Form build
			table_columns[i].Filter = waUserTypeFilterWhatsappUserManagement(col.Header, filter_id, optionsHTML, i)
			table_columns[i].EditForm = waUserTypeEditFormWhatsappUserManagement(col.Header, edit_id, col.Data, optionsHTML, i)
			table_columns[i].InsertField = waUserTypeInsertFieldWhatsappUserManagement(col.Header, insert_id, col.Data, optionsHTML, i)

		case "model.WAUserOf":
			allowedCols := map[string]bool{
				"user_of": true,
			}
			if allowedCols[col.Data] {
				table_columns[i].Filterable = true
				table_columns[i].Insertable = true
			}
			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data

			// Build options
			optionsHTML := ""
			for _, mode := range model.AllUserOf {
				var optLabel string
				if mode == model.UserOfCSNA {
					optLabel = config.GetConfig().Default.PT
				} else {
					optLabel = fun.SnakeToCapitalized(string(mode))
				}
				optionsHTML += fmt.Sprintf(`<option value="%s">%s</option>`, mode, optLabel)
			}

			table_columns[i].Filter = userOfFilterWhatsappUserManagement(col.Header, filter_id, optionsHTML, i)
			table_columns[i].EditForm = userOfEditFormWhatsappUserManagement(col.Header, edit_id, col.Data, optionsHTML, i)
			table_columns[i].InsertField = userOfInsertFieldWhatsappUserManagement(col.Header, insert_id, col.Data, optionsHTML, i)

		default:
			table_columns[i].Filterable = false
		}

	}
	actionable := ""
	if editable || deletable {
		table_columns = append(table_columns, Column{Data: "", Header: template.HTML("<i class='bx bx-run'></i>"), Type: "", Editable: false})
		actionable = "orderable"
	}

	// fmt.Println("show_header")
	// fmt.Println(!hideHeader)
	export_copy := false
	export_print := false
	export_pdf := false
	export_csv := false
	export_all_csv := false
	for _, export_type := range exportType {
		switch export_type {
		case EXPORT_COPY:
			export_copy = true
		case EXPORT_PRINT:
			export_print = true
		case EXPORT_CSV:
			export_csv = true
		case EXPORT_PDF:
			export_csv = true
		case EXPORT_ALL:
			export_all_csv = true
		}
	}
	passtrue := ""
	if passwordable {
		passtrue = `pass="true"`

	}
	renderedHTML, err := RenderTemplateToString("gui_server_table_whatsapp_bot_message_reply.html", map[string]any{
		"title":           template.HTML(title),
		"table_name":      table_name,
		"endpoint":        template.URL(endpoint),
		"table_columns":   table_columns,
		"actionable":      actionable,
		"insertable":      insertable,
		"page_length":     page_length,
		"length_menu":     length_menu,
		"order":           order,
		"hide_header":     hideHeader,
		"passwordable":    passwordable,
		"passtrue":        passtrue,
		"export_copy":     export_copy,
		"export_print":    export_print,
		"export_pdf":      export_pdf,
		"export_csv":      export_csv,
		"export_all_csv":  export_all_csv,
		"scrollUpDown":    scrollUpDown,
		"scrollLeftRight": scrollLeftRight,
		"column_array":    column_array,
	})
	if err != nil {
		fmt.Println("Error rendering template:", err)
		return template.HTML("Error rendering template")
	}

	return template.HTML(renderedHTML)
}
