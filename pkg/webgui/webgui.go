// Package webgui provides server-side rendered HTML components for DataTable-based web GUIs,
// including filterable, editable, and insertable fields with export options.
package webgui

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"service-platform/internal/config"
	"sync"
)

// Constants for export types
const (
	ExportCopy  = "COPY"
	ExportPrint = "PRINT"
	ExportCSV   = "CSV"
	ExportPdf   = "PDF"
	ExportAll   = "ALL"
)

var (
	tmplCache  *template.Template
	staticPath string
	mu         sync.Mutex // For thread safety
)

// Column defines the structure for a DataTable column, including its data source, type, edit form configuration, header, filter configuration, insert field configuration, column configuration for DataTables initialization, and visibility/orderability settings. This struct is used to dynamically generate HTML for DataTable columns based on the provided configurations.
type Column struct {
	Data, Type, EditID                    string
	Header, Filter, EditForm, InsertField template.HTML
	ColumnConfig                          template.JS
	Visible, Orderable, Filterable        bool
	Editable, Insertable                  bool
	Passwordable                          bool
	SelectableSrc                         template.URL
}

// loadTemplates loads and parses the templates each time it's called.
func loadTemplates() (*template.Template, error) {
	var err error

	// Get the absolute path of the static directory
	staticPath = config.ServicePlatform.Get().App.StaticDir
	staticPath, err = filepath.Abs(staticPath)
	if err != nil {
		fmt.Println("Error getting absolute path:", err)
		return nil, err
	}

	// Lock mutex to ensure thread safety
	mu.Lock()
	defer mu.Unlock()

	// Parse the templates each time
	tmplCache, err = template.ParseGlob(filepath.Join(staticPath, "**/*.html"))
	if err != nil {
		return nil, err
	}
	return tmplCache, nil
}

// RenderTemplateToString renders a template to a string
func RenderTemplateToString(templateName string, data interface{}) (string, error) {
	tmpl, err := loadTemplates()
	if err != nil {
		return "", err
	}

	// Create a buffer to capture the template output
	var renderedTemplate bytes.Buffer

	// Execute the template
	err = tmpl.ExecuteTemplate(&renderedTemplate, templateName, data)
	if err != nil {
		return "", err
	}

	return renderedTemplate.String(), nil
}

/*
	HTML Build
*/
// stringSelectableSrcEditForm generates HTML for a text input field with typeahead functionality in the edit form of a DataTable. It creates an input element and includes JavaScript to fetch options from a specified URL, initialize the Bloodhound suggestion engine, and set up Typeahead for autocomplete functionality. The function takes parameters for the column header, element ID, column data name, selectable source URL, and column index to properly configure the input field and its associated label.
func stringSelectableSrcEditForm(colHeader template.HTML, selectableSrc template.URL, editID, colData string, i int) template.HTML {
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
				  `, colHeader, editID, colData, i, colHeader, i-1, selectableSrc, editID, editID, editID)
	return template.HTML(html)
}

// stringSelectableSrcFilter generates HTML for a text input field with typeahead functionality in the filter section of a DataTable. It creates an input element and includes JavaScript to fetch options from a specified URL, initialize the Bloodhound suggestion engine, and set up Typeahead for autocomplete functionality. The function takes parameters for the column header, element ID, selectable source URL, and column index to properly configure the input field and its associated label.
func stringSelectableSrcFilter(colHeader template.HTML, selectableSrc template.URL, filterID string, i int) template.HTML {
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
				  `, colHeader, filterID, i, colHeader, i-1, selectableSrc, filterID, filterID, filterID)
	return template.HTML(html)
}

// stringSelectableSrcInsertField generates HTML for a text input field with typeahead functionality in the insert form of a DataTable. It creates an input element and includes JavaScript to fetch options from a specified URL, initialize the Bloodhound suggestion engine, and set up Typeahead for autocomplete functionality. The function takes parameters for the column header, element ID, column data name, selectable source URL, and column index to properly configure the input field and its associated label.
func stringSelectableSrcInsertField(colHeader template.HTML, selectableSrc template.URL, insertID, colData string, i int) template.HTML {
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
				  `, colHeader, insertID, colData, i, colHeader, i-1, selectableSrc, insertID, insertID, insertID)
	return template.HTML(html)
}

// stringInsertField generates HTML for a text input field in the insert form of a DataTable. It creates an input element and includes JavaScript to handle user input and filtering. The function takes parameters for the column header, element ID, column data name, and column index to properly configure the input field and its associated label.
func stringInsertField(colHeader template.HTML, insertID, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
					<label class="form-label">%s:</label>
					<input
					id="%s"
					name="%s"
					type="text"
					class="form-control"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, colHeader, insertID, colData, i, colHeader, i-1)
	return template.HTML(html)
}

// stringEditForm generates HTML for a text input field in the edit form of a DataTable. It creates an input element and includes JavaScript to handle user input and filtering. The function takes parameters for the column header, element ID, column data name, and column index to properly configure the input field and its associated label.
func stringEditForm(colHeader template.HTML, editID, colData string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
					<input
					id="%s"
					type="text"
					class="form-control"
					name="%s"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, colHeader, editID, colData, i, colHeader, i-1)
	return template.HTML(html)
}

// stringFilter generates HTML for a text input field in the filter section of a DataTable. It creates an input element and includes JavaScript to handle user input and filtering. The function takes parameters for the column header, element ID, and column index to properly configure the input field and its associated label.
func stringFilter(colHeader template.HTML, filterID string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
					<input
					id="%s"
					type="text"
					class="form-control dt-input dt-full-name"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, colHeader, filterID, i, colHeader, i-1)
	return template.HTML(html)
}

// stringColumnConfig returns a JavaScript configuration object for a DataTable column that renders string data. It takes parameters for the CSS class name, return value template, visibility, orderability, and column index to properly configure the column's appearance and behavior in the DataTable.
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

// imageFilter generates HTML for an image input field in the filter section of a DataTable. It creates an input element and includes JavaScript to handle user input and filtering. The function takes parameters for the column header, element ID, and column index to properly configure the input field and its associated label.
func imageFilter(colHeader template.HTML, filterID string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
					<input
					id="%s"
					type="text"
					class="form-control dt-input dt-full-name"
					data-column="%d"
					placeholder="Search %s by filename"
					data-column-index="%d" />`, colHeader, filterID, i, colHeader, i-1)
	return template.HTML(html)
}

// imageEditForm generates HTML for an image input field in the edit form of a DataTable. It creates an input element and includes JavaScript to handle file uploads and filtering. The function takes parameters for the column header, element ID, column data name, and column index to properly configure the input field and its associated label.
func imageEditForm(colHeader template.HTML, editID, colData string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
			<input
			  id="%s"
			  type="file"
			  class="form-control"
			  name="%s"
			  data-column="%d"
			  placeholder="Upload %s Image"
			  accept=".jpg, .jpeg, .png"
			  data-column-index="%d" />`, colHeader, editID, colData, i, colHeader, i-1)

	return template.HTML(html)
}

// imageInsertField generates HTML for an image input field in the insert form of a DataTable. It creates an input element and includes JavaScript to handle file uploads and filtering. The function takes parameters for the column header, element ID, column data name, and column index to properly configure the input field and its associated label.
func imageInsertField(colHeader template.HTML, insertID, colData string, i int) template.HTML {
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
			  data-column-index="%d" />`, colHeader, insertID, colData, i, colHeader, i-1)
	return template.HTML(html)
}

// imageTableColumnConfig returns a JavaScript configuration object for a DataTable column that renders image data. It takes parameters for the CSS class name, return value template, visibility, orderability, and column index to properly configure the column's appearance and behavior in the DataTable, specifically for rendering images within the table cells.
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

// timeTimeFilter generates HTML for a date range input field in the filter section of a DataTable. It creates input elements for selecting a start and end date, and includes JavaScript to handle user input and filtering based on the selected date range. The function takes parameters for the column header, element ID, table name, and column index to properly configure the input fields and their associated labels.
func timeTimeFilter(colHeader template.HTML, filterID, tableName string, i int) template.HTML {
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
			</div>`, colHeader, filterID, i, i-1, tableName, i, i-1, tableName, i, i-1)
	return template.HTML(html)
}

// timeTimeEditForm generates HTML for a date-time input field in the edit form of a DataTable. It creates an input element with a flatpickr date-time picker and includes JavaScript to handle user input and filtering based on the selected date and time. The function takes parameters for the column header, element ID, column data name, and column index to properly configure the input field and its associated label.
func timeTimeEditForm(colHeader template.HTML, editID, colData string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
			<input
			  id="%s"
			  type="number"
			  class="form-control flatpickr-datetime"
			  name="%s"
			  data-column="%d"
			  placeholder="%s YYYY-MM-DD HH:MM"
			  data-column-index="%d" />`, colHeader, editID, colData, i, colHeader, i-1)
	return template.HTML(html)
}

// timeTimeInsertField generates HTML for a date-time input field in the insert form of a DataTable. It creates an input element with a flatpickr date-time picker and includes JavaScript to handle user input and filtering based on the selected date and time. The function takes parameters for the column header, element ID, column data name, and column index to properly configure the input field and its associated label.
func timeTimeInsertField(colHeader template.HTML, insertID, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
			<label class="form-label">%s:</label>
			<input
			  id="%s"
			  name="%s"
			  type="text"
			  class="form-control flatpickr-datetime"
			  data-column="%d"
			  placeholder="%s YYYY-MM-DD HH:MM"
			  data-column-index="%d" />`, colHeader, insertID, colData, i, colHeader, i-1)
	return template.HTML(html)
}

// intSelectableSrcEditForm generates HTML for a number input field with typeahead functionality in the edit form of a DataTable. It creates an input element and includes JavaScript to fetch options from a specified URL, initialize the Bloodhound suggestion engine, and set up Typeahead for autocomplete functionality. The function takes parameters for the column header, element ID, column data name, selectable source URL, and column index to properly configure the input field and its associated label.
func intSelectableSrcEditForm(colHeader template.HTML, selectableSrc template.URL, editID, colData string, i int) template.HTML {
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
				  `, colHeader, editID, colData, i, colHeader, i-1, selectableSrc, editID, editID, editID)
	return template.HTML(html)
}

// intSelectableSrcFilter generates HTML for a number input field with typeahead functionality in the filter section of a DataTable. It creates an input element and includes JavaScript to fetch options from a specified URL, initialize the Bloodhound suggestion engine, and set up Typeahead for autocomplete functionality. The function takes parameters for the column header, element ID, selectable source URL, and column index to properly configure the input field and its associated label.
func intSelectableSrcFilter(colHeader template.HTML, selectableSrc template.URL, filterID string, i int) template.HTML {
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
				  `, colHeader, filterID, i, colHeader, i-1, selectableSrc, filterID, filterID, filterID)
	return template.HTML(html)
}

// intSelectableSrcInsertField generates HTML for a number input field with typeahead functionality in the insert form of a DataTable. It creates an input element and includes JavaScript to fetch options from a specified URL, initialize the Bloodhound suggestion engine, and set up Typeahead for autocomplete functionality. The function takes parameters for the column header, element ID, column data name, selectable source URL, and column index to properly configure the input field and its associated label.
func intSelectableSrcInsertField(colHeader template.HTML, selectableSrc template.URL, insertID, colData string, i int) template.HTML {
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
				  `, colHeader, insertID, colData, i, colHeader, i-1, selectableSrc, insertID, insertID, insertID)
	return template.HTML(html)
}

// intInsertField generates HTML for a number input field in the insert form of a DataTable. It creates an input element and includes JavaScript to handle user input and filtering. The function takes parameters for the column header, element ID, column data name, and column index to properly configure the input field and its associated label.
func intInsertField(colHeader template.HTML, insertID, colData string, i int) template.HTML {
	html := fmt.Sprintf(`
				<label class="form-label">%s:</label>
				<input
				  id="%s"
				  name="%s"
				  type="number"
				  class="form-control"
				  data-column="%d"
				  placeholder="%s number"
				  data-column-index="%d" />`, colHeader, insertID, colData, i, colHeader, i-1)
	return template.HTML(html)
}

// intFilter generates HTML for a number input field in the filter section of a DataTable. It creates an input element and includes JavaScript to handle user input and filtering. The function takes parameters for the column header, element ID, and column index to properly configure the input field and its associated label.
func intFilter(colHeader template.HTML, filterID string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
				  <input
					id="%s"
					type="number"
					class="form-control dt-input dt-full-name"
					data-column="%d"
					placeholder="%s Text"
					data-column-index="%d" />`, colHeader, filterID, i, colHeader, i-1)
	return template.HTML(html)
}

// intEditForm generates HTML for a number input field in the edit form of a DataTable. It creates an input element and includes JavaScript to handle user input and filtering. The function takes parameters for the column header, element ID, column data name, and column index to properly configure the input field and its associated label.
func intEditForm(colHeader template.HTML, editID, colData string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
				<input
				  id="%s"
				  type="number"
				  class="form-control"
				  name="%s"
				  data-column="%d"
				  placeholder="%s Number"
				  data-column-index="%d" />`, colHeader, editID, colData, i, colHeader, i-1)
	return template.HTML(html)
}

// intColumnConfig returns a JavaScript configuration object for a DataTable column that renders integer data. It takes parameters for the CSS class name, return value template, visibility, orderability, and column index to properly configure the column's appearance and behavior in the DataTable.
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

// // boolColumnConfig returns a JavaScript configuration object for a DataTable column that renders boolean data. It takes parameters for the CSS class name, return value template, visibility, orderability, and column index to properly configure the column's appearance and behavior in the DataTable, specifically for rendering boolean values within the table cells.
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

// floatSelectableSrcEditForm generates HTML for a number input field with typeahead functionality in the edit form of a DataTable, specifically for floating-point numbers. It creates an input element and includes JavaScript to fetch options from a specified URL, initialize the Bloodhound suggestion engine, and set up Typeahead for autocomplete functionality. The function takes parameters for the column header, element ID, column data name, selectable source URL, and column index to properly configure the input field and its associated label.
func floatSelectableSrcEditForm(colHeader template.HTML, selectableSrc template.URL, editID, colData string, i int) template.HTML {
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
				  `, colHeader, editID, colData, i, colHeader, i-1, selectableSrc, editID, editID, editID)
	return template.HTML(html)
}

// floatSelectableSrcFilter generates HTML for a number input field with typeahead functionality in the filter section of a DataTable, specifically for floating-point numbers. It creates an input element and includes JavaScript to fetch options from a specified URL, initialize the Bloodhound suggestion engine, and set up Typeahead for autocomplete functionality. The function takes parameters for the column header, element ID, selectable source URL, and column index to properly configure the input field and its associated label.
func floatSelectableSrcFilter(colHeader template.HTML, selectableSrc template.URL, filterID string, i int) template.HTML {
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
				  `, colHeader, filterID, i, colHeader, i-1, selectableSrc, filterID, filterID, filterID)
	return template.HTML(html)
}

// floatSelectableSrcInsertField generates HTML for a number input field with typeahead functionality in the insert form of a DataTable, specifically for floating-point numbers. It creates an input element and includes JavaScript to fetch options from a specified URL, initialize the Bloodhound suggestion engine, and set up Typeahead for autocomplete functionality. The function takes parameters for the column header, element ID, column data name, selectable source URL, and column index to properly configure the input field and its associated label.
func floatSelectableSrcInsertField(colHeader template.HTML, selectableSrc template.URL, insertID, colData string, i int) template.HTML {
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
				  `, colHeader, insertID, colData, i, colHeader, i-1, selectableSrc, insertID, insertID, insertID)
	return template.HTML(html)
}

// floatInsertField generates HTML for a number input field in the insert form of a DataTable, specifically for floating-point numbers. It creates an input element and includes JavaScript to handle user input and filtering. The function takes parameters for the column header, element ID, column data name, and column index to properly configure the input field and its associated label.
func floatInsertField(colHeader template.HTML, insertID, colData string, i int) template.HTML {
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
				  data-column-index="%d" />`, colHeader, insertID, colData, i, colHeader, i-1)
	return template.HTML(html)
}

// floatFilter generates HTML for a number input field in the filter section of a DataTable, specifically for floating-point numbers. It creates an input element and includes JavaScript to handle user input and filtering. The function takes parameters for the column header, element ID, and column index to properly configure the input field and its associated label.
func floatFilter(colHeader template.HTML, filterID string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s:</label>
				  <input
					id="%s"
					type="number"
					step="0.01"
					class="form-control dt-input dt-full-name"
					data-column="%d"
					placeholder="%s Float"
					data-column-index="%d" />`, colHeader, filterID, i, colHeader, i-1)
	return template.HTML(html)
}

// floatEditForm generates HTML for a number input field in the edit form of a DataTable, specifically for floating-point numbers. It creates an input element and includes JavaScript to handle user input and filtering. The function takes parameters for the column header, element ID, column data name, and column index to properly configure the input field and its associated label.
func floatEditForm(colHeader template.HTML, editID, colData string, i int) template.HTML {
	html := fmt.Sprintf(`<label class="form-label">%s</label>
				<input
				  id="%s"
				  type="number"
				  step="0.01"
				  class="form-control"
				  name="%s"
				  data-column="%d"
				  placeholder="%s Float"
				  data-column-index="%d" />`, colHeader, editID, colData, i, colHeader, i-1)
	return template.HTML(html)
}

// floatColumnConfig returns a JavaScript configuration object for a DataTable column that renders floating-point number data. It takes parameters for the CSS class name, return value template, visibility, orderability, and column index to properly configure the column's appearance and behavior in the DataTable.
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
