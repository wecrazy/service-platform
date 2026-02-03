package webgui

import (
	"fmt"
	"html/template"
)

func stringSelectableSrcEditForm(colHeader template.HTML, selectableSrc template.URL, edit_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
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
				  `, colHeader, edit_id, colData, i, colHeader, i-1, selectableSrc, edit_id, edit_id, edit_id)
	return template.HTML(html)
}

func stringSelectableSrcFilter(colHeader template.HTML, selectableSrc template.URL, filter_id string, i int) template.HTML {
	html := fmt.Sprintf(`
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
				  `, colHeader, filter_id, i, colHeader, i-1, selectableSrc, filter_id, filter_id, filter_id)
	return template.HTML(html)
}

func stringSelectableSrcInsertField(colHeader template.HTML, selectableSrc template.URL, insert_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
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
				  `, colHeader, insert_id, colData, i, colHeader, i-1, selectableSrc, insert_id, insert_id, insert_id)
	return template.HTML(html)
}

func stringInsertField(colHeader template.HTML, insert_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
					<label class="form-label">%s:</label>
					<input
					id="%s"
					name="%s"
					type="text"
					class="form-control"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, colHeader, insert_id, colData, i, colHeader, i-1)
	return template.HTML(html)
}

func stringEditForm(colHeader template.HTML, edit_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
					<input
					id="%s"
					type="text"
					class="form-control"
					name="%s"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, colHeader, edit_id, colData, i, colHeader, i-1)
	return template.HTML(html)
}

func stringFilter(colHeader template.HTML, filter_id string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
					<input
					id="%s"
					type="text"
					class="form-control dt-input dt-full-name"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, colHeader, filter_id, i, colHeader, i-1)
	return template.HTML(html)
}

func stringColumnConfig(className, returnValue string, visible, orderable bool, i int) template.JS {
	js := fmt.Sprintf(
		`{
					className: '%s',
					targets: %d,
					visible: %t,
					orderable: %t,
					render: function (data, type, full, meta) {
					var extract_data = extractTxt_HTML(data);
					return '%s';
					}
				},`, className, i, visible, orderable, returnValue)
	return template.JS(js)
}

// func imageFilter // ADD soon coz its not implemented yet
func imageEditForm(colHeader template.HTML, edit_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
			<input
			  id="%s"
			  type="file"
			  class="form-control"
			  name="%s"
			  data-column="%d"
			  placeholder="Upload %s Image"
			  accept=".jpg, .jpeg, .png"
			  data-column-index="%d" />`, colHeader, edit_id, colData, i, colHeader, i-1)

	return template.HTML(html)
}

func imageInsertField(colHeader template.HTML, insert_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
			<label class="form-label">%s:</label>
			<input
			  id="%s"
			  name="%s"
			  type="file"
			  class="form-control"
			  data-column="%d"
			  placeholder="Upload %s Image"
			  accept=".jpg, .jpeg, .png"
			  data-column-index="%d" />`, colHeader, insert_id, colData, i, colHeader, i-1)
	return template.HTML(html)
}

func imageTableColumnConfig(className, returnValue string, visible, orderable bool, i int) template.JS {
	js := fmt.Sprintf(
		`{
					className: '%s',
					targets: %d,
					visible: %t,
					orderable: %t,
					render: function (data, type, full, meta) {
					return '<div style="width: 50px;height: 50px;overflow: hidden;">%s</div>';
					}
				},`, className, i, visible, orderable, returnValue)
	return template.JS(js)
}

func timeTimeFilter(colHeader template.HTML, filter_id, table_name string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
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
			</div>`, colHeader, filter_id, i, i-1, table_name, i, i-1, table_name, i, i-1)
	return template.HTML(html)
}

func timeTimeEditForm(colHeader template.HTML, edit_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
			<input
			  id="%s"
			  type="number"
			  class="form-control flatpickr-datetime"
			  name="%s"
			  data-column="%d"
			  placeholder="%s YYYY-MM-DD HH:MM"
			  data-column-index="%d" />`, colHeader, edit_id, colData, i, colHeader, i-1)
	return template.HTML(html)
}

func timeTimeInsertField(colHeader template.HTML, insert_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
			<label class="form-label">%s:</label>
			<input
			  id="%s"
			  name="%s"
			  type="text"
			  class="form-control flatpickr-datetime"
			  data-column="%d"
			  placeholder="%s YYYY-MM-DD HH:MM"
			  data-column-index="%d" />`, colHeader, insert_id, colData, i, colHeader, i-1)
	return template.HTML(html)
}

func intSelectableSrcEditForm(colHeader template.HTML, selectableSrc template.URL, edit_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
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
				  `, colHeader, edit_id, colData, i, colHeader, i-1, selectableSrc, edit_id, edit_id, edit_id)
	return template.HTML(html)
}

func intSelectableSrcFilter(colHeader template.HTML, selectableSrc template.URL, filter_id string, i int) template.HTML {
	html := fmt.Sprintf(`
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
				  `, colHeader, filter_id, i, colHeader, i-1, selectableSrc, filter_id, filter_id, filter_id)
	return template.HTML(html)
}

func intSelectableSrcInsertField(colHeader template.HTML, selectableSrc template.URL, insert_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
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
				  `, colHeader, insert_id, colData, i, colHeader, i-1, selectableSrc, insert_id, insert_id, insert_id)
	return template.HTML(html)
}

func intInsertField(colHeader template.HTML, insert_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<input
				  id="%s"
				  name="%s"
				  type="number"
				  class="form-control"
				  data-column="%d"
				  placeholder="%s number"
				  data-column-index="%d" />`, colHeader, insert_id, colData, i, colHeader, i-1)
	return template.HTML(html)
}

func intFilter(colHeader template.HTML, filter_id string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
				  <input
					id="%s"
					type="number"
					class="form-control dt-input dt-full-name"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, colHeader, filter_id, i, colHeader, i-1)
	return template.HTML(html)
}

func intEditForm(colHeader template.HTML, edit_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
				<input
				  id="%s"
				  type="number"
				  class="form-control"
				  name="%s"
				  data-column="%d"
				  placeholder="%s Number"
				  data-column-index="%d" />`, colHeader, edit_id, colData, i, colHeader, i-1)
	return template.HTML(html)
}

func intColumnConfig(className, returnValue string, visible, orderable bool, i int) template.JS {
	js := fmt.Sprintf(
		`{
						className: '%s',
						targets: %d,
						visible: %t,
						orderable: %t,
						render: function (data, type, full, meta) {
						return '%s';
						}
					},`, className, i, visible, orderable, returnValue)
	return template.JS(js)
}

// ADD this if needed
//
//	func boolColumnConfig(className, returnValue string, visible, orderable bool, i int) template.JS {
//		js := fmt.Sprintf(
//			`{
//					className: '%s',
//					targets: %d,
//					visible: %t,
//					orderable: %t,
//					render: function (data, type, full, meta) {
//						return '%s';
//					}
//				},`, className, i, visible, orderable, returnValue)
//		return template.JS(js)
//	}
func floatSelectableSrcEditForm(colHeader template.HTML, selectableSrc template.URL, edit_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s</label>
				<input
					id="%s"
					type="number"
					step="0.01"
					class="form-control"
					name="%s"
					data-column="%d"
					placeholder="%s Float"
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
				  `, colHeader, edit_id, colData, i, colHeader, i-1, selectableSrc, edit_id, edit_id, edit_id)
	return template.HTML(html)
}

func floatSelectableSrcFilter(colHeader template.HTML, selectableSrc template.URL, filter_id string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<input
					id="%s"
					type="number"
					step="0.01"
					class="form-control dt-input dt-full-name typeahead-input"
					data-column="%d"
					placeholder="%s Float"
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
				  `, colHeader, filter_id, i, colHeader, i-1, selectableSrc, filter_id, filter_id, filter_id)
	return template.HTML(html)
}

func floatSelectableSrcInsertField(colHeader template.HTML, selectableSrc template.URL, insert_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<input
					id="%s"
					type="number"
					step="0.01"
					name="%s"
					class="form-control"
					data-column="%d"
					placeholder="%s Float"
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
				  `, colHeader, insert_id, colData, i, colHeader, i-1, selectableSrc, insert_id, insert_id, insert_id)
	return template.HTML(html)
}

func floatInsertField(colHeader template.HTML, insert_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<input
				  id="%s"
				  name="%s"
				  type="number"
				  step="0.01"
				  class="form-control"
				  data-column="%d"
				  placeholder="%s float"
				  data-column-index="%d" />`, colHeader, insert_id, colData, i, colHeader, i-1)
	return template.HTML(html)
}

func floatFilter(colHeader template.HTML, filter_id string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
				  <input
					id="%s"
					type="number"
					step="0.01"
					class="form-control dt-input dt-full-name"
					data-column="%d"
					placeholder="%s Float"
					data-column-index="%d" />`, colHeader, filter_id, i, colHeader, i-1)
	return template.HTML(html)
}

func floatEditForm(colHeader template.HTML, edit_id, colData string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
				<input
				  id="%s"
				  type="number"
				  step="0.01"
				  class="form-control"
				  name="%s"
				  data-column="%d"
				  placeholder="%s Float"
				  data-column-index="%d" />`, colHeader, edit_id, colData, i, colHeader, i-1)
	return template.HTML(html)
}

func floatColumnConfig(className, returnValue string, visible, orderable bool, i int) template.JS {
	js := fmt.Sprintf(
		`{
						className: '%s',
						targets: %d,
						visible: %t,
						orderable: %t,
						render: function (data, type, full, meta) {
						return '%s';
						}
					},`, className, i, visible, orderable, returnValue)
	return template.JS(js)
}
