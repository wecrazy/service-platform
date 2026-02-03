package webguibuilder

import (
	"html/template"
	"strings"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

func TABLE_WHATSAPP_BOT_LOG_MSG_RECEIVED(session string, redisDB *redis.Client, db *gorm.DB) template.HTML {
	var b strings.Builder
	b.WriteString(`
	<div class="card">
		<h3 class="card-header"><i class="fa-brands fa-square-whatsapp me-2"></i>Log Message</h3>
		<div class="card-datatable table-responsive">
			<div class="row mb-2 align-items-center p-2 m-0">
				<div class="col-md-6 d-flex align-items-center">
					<input type="text" id="waMsgReceivedLogSearch" class="form-control me-2" placeholder="Search..." style="max-width: 300px;">
				</div>
				<div class="col-md-6 text-end">
					<div class="btn-group" id="waMsgReceivedLogExportBtns"></div>
				</div>
			</div>
			<table class="table border-top table-striped" id="waMsgReceivedLog">
				<thead>
					<tr>
						<th>Level</th>
						<th>Date in Log</th>
						<th>Event</th>
						<th>Message ID</th>
						<th>From</th>
						<th>Name</th>
						<th>Sender</th>
						<th>Message Sent</th>
						<th>IsGroup</th>
						<th>Message</th>
					</tr>
				</thead>
			</table>
		</div>
	</div>

	<script>
		let endpoint = $('#endpointWaLogMsgReceived').data('endpoint');

		let isScrollableY = false;
		let isScrollableX = false;

		function initLogWaMsgReceivedTable(scrollY, scrollX) {
			let table = $('#waMsgReceivedLog').DataTable({
				destroy: true,
				serverSide: true,
				processing: true,
				scrollY: scrollY ? '400px' : '',
				scrollX: scrollX,
				ajax: {
					url: endpoint,
					type: 'POST'
				},
				columnDefs: [
					{
						targets: 0,
						render: function (data, type, row) {
							let badgeClass = 'bg-secondary';
							switch (data.toUpperCase()) {
								case 'FATAL':
									badgeClass = 'bg-danger text-white fw-bold';
									break;
								case 'ERROR':
									badgeClass = 'bg-label-danger';
									break;
								case 'WARN':
								case 'WARNING':
									badgeClass = 'bg-warning';
									break;
								case 'INFO':
									badgeClass = 'bg-info';
									break;
								case 'DEBUG':
									badgeClass = 'bg-dark';
									break;
								case 'TRACE':
									badgeClass = 'bg-primary';
									break;
							}
							return '<span class="badge ' + badgeClass + '">' + data + '</span>';
						}
					}
				],
				buttons: [
					{
						extend: 'excel',
						className: 'btn btn-outline-success mx-1 p-2',
						text: '<i class="bx bx-file"></i> Excel',
						title: 'Whatsapp Log Message Received',
						filename: 'Whatsapp_Log_Message_Received',
						exportOptions: {
						columns: ':visible'
						}
					},
					{
						extend: 'csv',
						className: 'btn btn-outline-primary mx-1 p-2',
						text: '<i class="bx bx-file"></i> CSV',
						title: 'Whatsapp Log Message Received',
						filename: 'Whatsapp_Log_Message_Received',
						exportOptions: {
						columns: ':visible'
						}
					},
					{
						extend: 'copy',
						className: 'btn btn-outline-secondary mx-1 p-2',
						text: '<i class="bx bx-copy"></i> Copy',
						title: 'Whatsapp Log Message Received',
						filename: 'Whatsapp_Log_Message_Received',
						exportOptions: {
						columns: ':visible'
						}
					},
					{
						text: scrollY ? '<i class="bx bx-expand-vertical"></i>' : '<i class="bx bx-collapse-vertical"></i>',
						className: 'btn mx-1 p-2 ' + (!scrollY ? 'btn-dark' : 'btn-label-dark'),
						action: function () {
						isScrollableY = !isScrollableY;
						initLogWaMsgReceivedTable(isScrollableY, isScrollableX);
						}
					},
					{
						text: scrollX ? '<i class="bx bx-expand-horizontal"></i>' : '<i class="bx bx-collapse-horizontal"></i>',
						className: 'btn mx-1 p-2 ' + (!scrollX ? 'btn-dark' : 'btn-label-dark'),
						action: function () {
						isScrollableX = !isScrollableX;
						initLogWaMsgReceivedTable(isScrollableY, isScrollableX);
						}
					}
				],
				dom: '<"row mb-2 align-items-center"<"col-md-6 d-flex align-items-center"l><"col-md-6 text-end"B>>rt<"row"<"col-md-6"i><"col-md-6"p>>'
			});

			// Move export buttons to custom container
			table.buttons([0,1,2]).containers().appendTo('#waMsgReceivedLogExportBtns');
		}

		$(document).ready(function () {
			initLogWaMsgReceivedTable(isScrollableY, isScrollableX);

			// Custom search input
			$('#waMsgReceivedLogSearch').on('keyup', function () {
				$('#waMsgReceivedLog').DataTable().search(this.value).draw();
			});
		});
	</script>
	`)
	return template.HTML(b.String())
}
