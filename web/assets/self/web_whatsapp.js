/**
 * WhatsApp Tab Management
 * Handles all functionality for WhatsApp bot management and user management tabs
 */

// Helper function to get i18n translations
function getI18n(key) {
  // Check for i18next (the actual i18n library being used)
  if (typeof i18next !== 'undefined' && i18next && typeof i18next.t === 'function' && i18next.isInitialized) {
    const translation = i18next.t(key);
    // If translation is the same as key, it means key doesn't exist
    if (translation && translation !== key) {
      return translation;
    }
  }

  // Check for window.i18n as fallback
  if (typeof window.i18n !== 'undefined' && window.i18n && typeof window.i18n.t === 'function') {
    return window.i18n.t(key);
  }

  // If no translation found, return the key itself for debugging
  return key;
}

// WhatsApp Bot Management Module
const WhatsAppBotManager = (function () {
  // Private variables
  let connectionStatusInterval;
  let connectModal, autoReplyModal, groupDetailsModal;
  let RANDOM_ACCESS = '';
  let DATA_SEPARATOR = '|'; // Default separator

  // Initialize the WhatsApp Bot tab
  function init() {
    // Get RANDOM_ACCESS from window if available
    RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    if (!RANDOM_ACCESS) {
      console.error('RANDOM_ACCESS not available yet, will retry on tab load');
      return;
    }

    // Load data separator from backend
    loadDataSeparator();

    // Log current language for debugging
    if (typeof i18next !== 'undefined' && i18next) {
      //   console.log('Current i18next language:', i18next.language);
    }

    // Initialize modals
    const connectModalEl = document.getElementById('connectModal');
    const autoReplyModalEl = document.getElementById('autoReplyModal');
    const groupDetailsModalEl = document.getElementById('groupDetailsModal');
    const imageViewerModalEl = document.getElementById('imageViewerModal');

    if (connectModalEl) connectModal = new bootstrap.Modal(connectModalEl);
    if (autoReplyModalEl) autoReplyModal = new bootstrap.Modal(autoReplyModalEl);
    if (groupDetailsModalEl) groupDetailsModal = new bootstrap.Modal(groupDetailsModalEl);

    // Check connection status immediately
    checkConnectionStatus();

    // Clear existing interval if any
    if (connectionStatusInterval) {
      clearInterval(connectionStatusInterval);
    }
    // Start auto-refresh every 10 seconds
    connectionStatusInterval = setInterval(checkConnectionStatus, 10000);

    // Load initial data
    loadStatistics();
    initializeMessagesTable();
    initializeIncomingTable();
    refreshGroupsTable();
    initializeAutoReplyTable();

    // Form submission handler
    const sendForm = document.getElementById('send-message-form');
    if (sendForm) {
      sendForm.removeEventListener('submit', handleSendMessage);
      sendForm.addEventListener('submit', handleSendMessage);
    }

    // Connection method toggle
    document.querySelectorAll('input[name="connection-method"]').forEach(radio => {
      radio.addEventListener('change', function () {
        const pairingField = document.getElementById('pairing-phone-field');
        if (pairingField) {
          pairingField.style.display = this.value === 'pairing' ? 'block' : 'none';
        }
      });
    });
  }

  // Check connection status
  async function checkConnectionStatus() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';
    if (!RANDOM_ACCESS) return;

    try {
      const response = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/status', {
        headers: { 'Content-Type': 'application/json' }
      });
      const data = await response.json();

      const statusBadge = document.getElementById('connection-status');
      const accountInfo = document.getElementById('account-info');
      const noConnectionInfo = document.getElementById('no-connection-info');
      const btnConnect = document.getElementById('btn-connect');
      const btnDisconnect = document.getElementById('btn-disconnect');
      const btnLogout = document.getElementById('btn-logout');

      if (!statusBadge) return; // Tab not loaded yet

      if (data.connected) {
        statusBadge.innerHTML = '<i class="fas fa-check-circle me-1"></i>Connected';
        statusBadge.className = 'badge bg-success';

        if (accountInfo) accountInfo.style.display = 'block';
        if (noConnectionInfo) noConnectionInfo.style.display = 'none';

        // Show connection info panel on right side
        const connectionInfoPanel = document.getElementById('connection-info-panel');
        if (connectionInfoPanel) {
          connectionInfoPanel.style.display = 'block';
        }

        if (data.account) {
          const nameEl = document.getElementById('account-name');
          const phoneEl = document.getElementById('account-phone');
          const statusTextEl = document.getElementById('connection-status-text');
          const connectionTimeEl = document.getElementById('connection-time');
          const profilePicEl = document.getElementById('profile-pic');
          const deviceInfoEl = document.getElementById('device-info');
          const platformInfoEl = document.getElementById('platform-info');
          const connectedSinceEl = document.getElementById('connected-since');

          if (nameEl) nameEl.textContent = data.account.name || 'WhatsApp User';
          if (phoneEl) {
            const phoneDisplay = data.account.phone || data.account.jid || 'No phone number';
            phoneEl.textContent = phoneDisplay;
          }
          if (statusTextEl) {
            const deviceInfo = data.account.device ? ` - ${data.account.device}` : '';
            statusTextEl.textContent = `Active & Ready${deviceInfo}`;
          }
          if (connectionTimeEl) {
            const now = new Date();
            connectionTimeEl.textContent = `Connected at ${now.toLocaleTimeString()}`;
          }
          if (profilePicEl && data.account.profile_pic_url) {
            profilePicEl.src = data.account.profile_pic_url;
          }

          // Update right panel info
          if (deviceInfoEl) deviceInfoEl.textContent = data.account.device || 'Unknown';
          if (platformInfoEl) platformInfoEl.textContent = data.account.platform || 'Unknown';
          if (connectedSinceEl) {
            const now = new Date();
            connectedSinceEl.textContent = now.toLocaleTimeString();
          }
        }

        if (btnConnect) btnConnect.disabled = true;
        if (btnDisconnect) btnDisconnect.disabled = false;
        if (btnLogout) btnLogout.disabled = false;
      } else {
        statusBadge.innerHTML = '<i class="fas fa-times-circle me-1"></i>Disconnected';
        statusBadge.className = 'badge bg-danger';

        if (accountInfo) accountInfo.style.display = 'none';
        if (noConnectionInfo) noConnectionInfo.style.display = 'block';

        // Hide connection info panel
        const connectionInfoPanel = document.getElementById('connection-info-panel');
        if (connectionInfoPanel) connectionInfoPanel.style.display = 'none';

        if (btnConnect) btnConnect.disabled = false;
        if (btnDisconnect) btnDisconnect.disabled = true;
        if (btnLogout) btnLogout.disabled = true;
      }
    } catch (error) {
      console.error('Failed to check connection status:', error);
      const statusBadge = document.getElementById('connection-status');
      if (statusBadge) {
        statusBadge.innerHTML = '<i class="fas fa-exclamation-triangle me-1"></i>Error';
        statusBadge.className = 'badge bg-warning';
      }
    }
  }

  // Load data separator from backend
  async function loadDataSeparator() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';
    if (!RANDOM_ACCESS) return;

    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp/data-separator`);
      const data = await response.json();

      if (data.success && data.separator) {
        DATA_SEPARATOR = data.separator;

        // Update hint text in the keywords input
        const keywordsInput = document.getElementById('auto-reply-keywords');
        if (keywordsInput) {
          const hintElement = keywordsInput.parentElement.querySelector('.form-text');
          if (hintElement) {
            hintElement.textContent = `Separate multiple keywords with "${DATA_SEPARATOR}"`;
          }
        }
      }
    } catch (error) {
      console.error('Failed to load data separator:', error);
    }
  }

  function showConnectModal() {
    if (!connectModal) {
      const modalEl = document.getElementById('connectModal');
      if (modalEl) connectModal = new bootstrap.Modal(modalEl);
    }
    if (connectModal) connectModal.show();
  }

  async function connectWhatsApp() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    const method = document.querySelector('input[name="connection-method"]:checked')?.value;
    const phone = method === 'pairing' ? document.getElementById('pairing-phone')?.value : '';

    if (method === 'pairing' && !phone) {
      Swal.fire({
        icon: 'warning',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.phoneRequired')
      });
      return;
    }

    try {
      const response = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/connect', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method, phone })
      });

      const data = await response.json();

      if (data.success) {
        if (connectModal) connectModal.hide();

        if (method === 'qr' && data.qr_code) {
          const qrContainer = document.getElementById('qr-container');
          const qrCode = document.getElementById('qr-code');
          if (qrContainer && qrCode) {
            qrCode.innerHTML = '';
            QRCode.toCanvas(qrCode, data.qr_code, { width: 256 });
            qrContainer.style.display = 'block';
          }
        } else if (method === 'pairing' && data.pairing_code) {
          const pairingContainer = document.getElementById('pairing-container');
          const pairingCodeEl = document.getElementById('pairing-code');
          if (pairingContainer && pairingCodeEl) {
            pairingCodeEl.textContent = data.pairing_code;
            pairingContainer.style.display = 'block';
          }
        }

        Swal.fire({
          icon: 'success',
          title: getI18n('table.successTitle'),
          text: getI18n('whatsapp.connectSuccess'),
          timer: 2000
        });

        checkConnectionStatus();
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('table.errorTitle'),
          text: data.message || getI18n('whatsapp.connectFailed')
        });
      }
    } catch (error) {
      console.error('Failed to connect:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.connectFailed')
      });
    }
  }

  async function disconnectWhatsApp() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    const result = await Swal.fire({
      icon: 'warning',
      title: getI18n('whatsapp.confirmDelete'),
      text: getI18n('whatsapp.confirmDisconnect'),
      showCancelButton: true,
      confirmButtonText: getI18n('whatsapp.confirmDisconnectButton'),
      cancelButtonText: getI18n('table.cancelButton'),
      confirmButtonColor: '#d33'
    });

    if (!result.isConfirmed) return;

    try {
      const response = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/disconnect', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });

      const data = await response.json();

      if (data.success) {
        Swal.fire({
          icon: 'success',
          title: getI18n('table.successTitle'),
          text: getI18n('whatsapp.disconnectSuccess'),
          timer: 2000
        });
        checkConnectionStatus();

        const qrContainer = document.getElementById('qr-container');
        const pairingContainer = document.getElementById('pairing-container');
        if (qrContainer) qrContainer.style.display = 'none';
        if (pairingContainer) pairingContainer.style.display = 'none';
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('table.errorTitle'),
          text: data.message || getI18n('whatsapp.disconnectFailed')
        });
      }
    } catch (error) {
      console.error('Failed to disconnect:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.disconnectFailed')
      });
    }
  }

  async function logoutWhatsApp() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';
    const logoutResult = await Swal.fire({
      icon: 'warning',
      title: getI18n('whatsapp.confirmLogout'),
      text: getI18n('whatsapp.confirmLogoutText'),
      showCancelButton: true,
      confirmButtonText: getI18n('whatsapp.confirmLogoutButton'),
      cancelButtonText: getI18n('table.cancelButton'),
      confirmButtonColor: '#d33'
    });
    if (!logoutResult.isConfirmed) return;

    try {
      const response = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/logout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });

      const data = await response.json();

      if (data.success) {
        Swal.fire({
          icon: 'success',
          title: getI18n('table.successTitle'),
          text: getI18n('whatsapp.logoutSuccess'),
          timer: 2000
        });
        checkConnectionStatus();
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('table.errorTitle'),
          text: data.message || getI18n('whatsapp.logoutFailed')
        });
      }
    } catch (error) {
      console.error('Failed to logout:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.logoutFailed')
      });
    }
  }

  async function refreshQRCode() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    try {
      const response = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/refresh_qr', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });

      const data = await response.json();

      if (data.success && data.qr_code) {
        const qrCode = document.getElementById('qr-code');
        if (qrCode) {
          qrCode.innerHTML = '';
          QRCode.toCanvas(qrCode, data.qr_code, { width: 256 });
        }
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('table.errorTitle'),
          text: getI18n('whatsapp.refreshQRFailed')
        });
      }
    } catch (error) {
      console.error('Failed to refresh QR code:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.refreshQRFailed')
      });
    }
  }

  function toggleMessageFields() {
    const messageType = document.getElementById('message-type')?.value;
    const textMessageField = document.getElementById('text-message-field');
    const mediaFields = document.getElementById('media-fields');
    const locationFields = document.getElementById('location-fields');
    const contactFields = document.getElementById('contact-fields');
    const reactionFields = document.getElementById('reaction-fields');
    const mediaDataInput = document.getElementById('media-data');
    const mediaMimetypeInput = document.getElementById('media-mimetype');

    // Hide all fields first
    [mediaFields, locationFields, contactFields, reactionFields].forEach(field => {
      if (field) field.style.display = 'none';
    });

    // Show relevant fields and update placeholders
    if (messageType === 'text') {
      if (textMessageField) textMessageField.style.display = 'block';
    } else if (['image', 'video', 'document', 'audio', 'sticker'].includes(messageType)) {
      if (mediaFields) mediaFields.style.display = 'block';
      if (textMessageField) textMessageField.style.display = 'block';

      // Update placeholders based on type
      if (mediaDataInput) {
        const placeholders = {
          'image': 'https://example.com/photo.jpg or base64 data',
          'video': 'https://example.com/video.mp4 or base64 data',
          'document': 'https://example.com/document.pdf or base64 data',
          'audio': 'https://example.com/audio.mp3 or base64 data',
          'sticker': 'https://example.com/sticker.webp or base64 data'
        };
        mediaDataInput.placeholder = placeholders[messageType] || 'URL or base64 data';
      }

      if (mediaMimetypeInput) {
        const mimetypes = {
          'image': 'image/jpeg',
          'video': 'video/mp4',
          'document': 'application/pdf',
          'audio': 'audio/mpeg',
          'sticker': 'image/webp'
        };
        mediaMimetypeInput.placeholder = mimetypes[messageType] || 'mime/type';
      }
    } else if (messageType === 'location') {
      if (locationFields) locationFields.style.display = 'block';
      if (textMessageField) textMessageField.style.display = 'none';
    } else if (messageType === 'contact') {
      if (contactFields) contactFields.style.display = 'block';
      if (textMessageField) textMessageField.style.display = 'none';
    } else if (messageType === 'reaction') {
      if (reactionFields) reactionFields.style.display = 'block';
      if (textMessageField) textMessageField.style.display = 'none';
    }

    // Update filename placeholder based on media type
    const mediaFilenameInput = document.getElementById('media-filename');
    if (mediaFilenameInput && ['image', 'video', 'document', 'audio', 'sticker'].includes(messageType)) {
      const filenames = {
        'image': 'photo.jpg',
        'video': 'video.mp4',
        'document': 'document.pdf',
        'audio': 'audio.mp3',
        'sticker': 'sticker.webp'
      };
      mediaFilenameInput.placeholder = filenames[messageType] || 'filename';
    }
  }

  function toggleGroupMode() {
    const sendTo = document.getElementById('send-to')?.value;
    const groupJidField = document.getElementById('group-jid-field');

    if (groupJidField) {
      groupJidField.style.display = sendTo === 'group' ? 'block' : 'none';
    }
  }

  async function handleSendMessage(e) {
    e.preventDefault();
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    const formData = {
      to: document.getElementById('recipient-number')?.value,
      type: document.getElementById('message-type')?.value,
      text: document.getElementById('message-text')?.value,
      media_url: document.getElementById('media-url')?.value,
      caption: document.getElementById('media-caption')?.value,
      latitude: document.getElementById('location-lat')?.value,
      longitude: document.getElementById('location-lng')?.value,
      location_name: document.getElementById('location-name')?.value,
      live_location: document.getElementById('live-location')?.checked || false,
      contact_name: document.getElementById('contact-name')?.value,
      contact_number: document.getElementById('contact-number')?.value,
      reaction_emoji: document.getElementById('reaction-emoji')?.value,
      message_id: document.getElementById('reaction-message-id')?.value,
      send_to: document.getElementById('send-to')?.value,
      group_jid: document.getElementById('group-jid')?.value
    };

    try {
      const response = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/send_message', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData)
      });

      const data = await response.json();

      if (data.success) {
        Swal.fire({
          icon: 'success',
          title: getI18n('table.successTitle'),
          text: getI18n('whatsapp.messageSentSuccess'),
          timer: 2000
        });
        e.target.reset();
        refreshMessagesTable();
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('table.errorTitle'),
          text: data.message || getI18n('whatsapp.messageSentFailed')
        });
      }
    } catch (error) {
      console.error('Failed to send message:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.messageSentFailed')
      });
    }
  }

  function initializeMessagesTable() {
    const table = $('#messages-table');
    if (!table.length) return;

    if ($.fn.DataTable.isDataTable('#messages-table')) {
      table.DataTable().destroy();
    }

    window.messagesDataTable = table.DataTable({
      serverSide: true,
      processing: true,
      ajax: {
        url: '/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/messages',
        type: 'GET',
        data: function (d) {
          // DataTables params to API params
          return {
            page: Math.floor(d.start / d.length) + 1,
            limit: d.length
          };
        },
        dataSrc: function (json) {
          json.recordsTotal = json.pagination?.total_items || 0;
          json.recordsFiltered = json.pagination?.total_items || 0;
          return json.data || [];
        }
      },
      columns: [
        { data: 'ID', title: 'ID', width: '60px', className: 'text-center' },
        {
          data: 'whatsapp_message_sent_to',
          title: 'To',
          width: '150px',
          render: function (data, type, row) {
            if (!data || data === '-') return '<span class="text-muted">-</span>';
            const truncated = data.length > 20 ? data.substring(0, 20) + '...' : data;
            return `<span title="${data}" data-bs-toggle="tooltip">${truncated}</span>`;
          }
        },
        {
          data: 'whatsapp_message_body',
          title: 'Type',
          width: '100px',
          render: function (data, type, row) {
            // Since whatsapp_message_type is not in JSON, infer from body or default
            const msgType = 'text'; // Default since JSON field is hidden
            return `<span class="badge bg-${getMessageTypeColor(msgType)}">${msgType}</span>`;
          }
        },
        {
          data: 'whatsapp_message_body',
          title: 'Message',
          render: function (data, type, row) {
            if (!data || data === '-') return '<span class="text-muted">-</span>';
            const truncated = data.length > 30 ? data.substring(0, 30) + '...' : data;
            return `<span title="${data.replace(/"/g, '&quot;')}" data-bs-toggle="tooltip">${truncated}</span>`;
          }
        },
        {
          data: 'whatsapp_msg_status',
          title: 'Status',
          width: '100px',
          render: function (data, type, row) {
            return `<span class="badge bg-${getStatusColor(data)}">${data || '-'}</span>`;
          }
        },
        {
          data: 'whatsapp_sent_at',
          title: 'Sent At',
          width: '150px',
          render: function (data, type, row) {
            return formatDate(data || row.CreatedAt);
          }
        }
      ],
      order: [[0, 'desc']],
      pageLength: 10,
      lengthMenu: [[10, 25, 50, 100], [10, 25, 50, 100]],
      language: {
        processing: '<i class="fas fa-spinner fa-spin text-primary"></i> Loading...',
        emptyTable: '<div class="text-center p-3"><i class="fas fa-comments fa-2x text-muted mb-2"></i><p class="text-muted mb-0">No messages found</p></div>',
        zeroRecords: '<div class="text-center p-3"><i class="fas fa-search fa-2x text-muted mb-2"></i><p class="text-muted mb-0">No matching messages</p></div>',
        lengthMenu: '_MENU_ entries per page',
        paginate: {
          first: '<i class="fas fa-angle-double-left"></i>',
          last: '<i class="fas fa-angle-double-right"></i>',
          next: '<i class="fas fa-angle-right"></i>',
          previous: '<i class="fas fa-angle-left"></i>'
        }
      },
      drawCallback: function () {
        $('[data-bs-toggle="tooltip"]').tooltip();
      }
    });
  }

  function refreshMessagesTable() {
    if (window.messagesDataTable) {
      window.messagesDataTable.ajax.reload();
    } else {
      initializeMessagesTable();
    }
  }

  function initializeIncomingTable() {
    const table = $('#incoming-table');
    if (!table.length) return;

    if ($.fn.DataTable.isDataTable('#incoming-table')) {
      table.DataTable().destroy();
    }

    window.incomingDataTable = table.DataTable({
      serverSide: true,
      processing: true,
      ajax: {
        url: '/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/incoming',
        type: 'GET',
        data: function (d) {
          return {
            page: Math.floor(d.start / d.length) + 1,
            limit: d.length
          };
        },
        dataSrc: function (json) {
          json.recordsTotal = json.pagination?.total_items || 0;
          json.recordsFiltered = json.pagination?.total_items || 0;
          return json.data || [];
        }
      },
      columns: [
        { data: 'ID', title: 'ID', width: '60px', className: 'text-center' },
        {
          data: 'whatsapp_sender_jid',
          title: 'From',
          width: '150px',
          render: function (data, type, row) {
            if (!data || data === '-') return '<span class="text-muted">-</span>';
            const truncated = data.length > 20 ? data.substring(0, 20) + '...' : data;
            return `<span title="${data}" data-bs-toggle="tooltip">${truncated}</span>`;
          }
        },
        {
          data: 'whatsapp_message_type',
          title: 'Type',
          width: '100px',
          render: function (data, type, row) {
            return `<span class="badge bg-${getMessageTypeColor(data)}">${data || 'text'}</span>`;
          }
        },
        {
          data: 'whatsapp_message_body',
          title: 'Message',
          render: function (data, type, row) {
            if (!data || data === '-') return '<span class="text-muted">-</span>';
            const truncated = data.length > 40 ? data.substring(0, 40) + '...' : data;
            return `<span title="${data.replace(/"/g, '&quot;')}" data-bs-toggle="tooltip">${truncated}</span>`;
          }
        },
        {
          data: 'whatsapp_received_at',
          title: 'Received At',
          width: '150px',
          render: function (data, type, row) {
            return formatDate(data || row.CreatedAt);
          }
        }
      ],
      order: [[0, 'desc']],
      pageLength: 10,
      lengthMenu: [[10, 25, 50, 100], [10, 25, 50, 100]],
      language: {
        processing: '<i class="fas fa-spinner fa-spin text-primary"></i> Loading...',
        emptyTable: '<div class="text-center p-3"><i class="fas fa-inbox fa-2x text-muted mb-2"></i><p class="text-muted mb-0">No incoming messages</p></div>',
        zeroRecords: '<div class="text-center p-3"><i class="fas fa-search fa-2x text-muted mb-2"></i><p class="text-muted mb-0">No matching messages</p></div>',
        lengthMenu: '_MENU_ entries per page',
        paginate: {
          first: '<i class="fas fa-angle-double-left"></i>',
          last: '<i class="fas fa-angle-double-right"></i>',
          next: '<i class="fas fa-angle-right"></i>',
          previous: '<i class="fas fa-angle-left"></i>'
        }
      },
      drawCallback: function () {
        $('[data-bs-toggle="tooltip"]').tooltip();
      }
    });
  }

  function refreshIncomingTable() {
    if (window.incomingDataTable) {
      window.incomingDataTable.ajax.reload();
    } else {
      initializeIncomingTable();
    }
  }

  function initializeGroupsTable() {
    const table = $('#groups-table');
    if (!table.length) return;

    if ($.fn.DataTable.isDataTable('#groups-table')) {
      table.DataTable().destroy();
    }

    window.groupsDataTable = table.DataTable({
      serverSide: true,
      processing: true,
      ajax: {
        url: '/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/groups/datatable',
        type: 'POST',
        data: function (d) {
          return JSON.stringify(d);
        },
        contentType: 'application/json',
        error: function (xhr, error, code) {
          console.error('Groups DataTable Error:', error, code);
          console.error('Response:', xhr.responseText);
        }
      },
      columns: [
        { data: 'name' },
        { data: 'jid' },
        {
          data: 'participants',
          render: function (data) {
            return Array.isArray(data) ? data.length : 0;
          },
          orderable: false
        },
        {
          data: 'created_at',
          render: function (data) {
            return formatDate(data);
          }
        },
        {
          data: 'jid',
          render: function (data) {
            return `<button class="btn btn-sm btn-info" onclick="WhatsAppBotManager.showGroupDetails('${data}')" title="${getI18n('whatsapp.groupDetails')}">
              <i class="fas fa-eye"></i>
            </button>`;
          },
          orderable: false
        }
      ],
      order: [[3, 'desc']], // Order by CreatedAt descending
      pageLength: 20,
      lengthMenu: [[10, 20, 50, 100], [10, 20, 50, 100]],
      responsive: true,
      dom: 'lrtip',
      language: {
        processing: '<i class="fa fa-spinner fa-spin"></i> ' + getI18n('common.loading'),
        lengthMenu: '_MENU_ ' + (getI18n('whatsapp.entriesPerPage') || 'entries per page'),
        zeroRecords: getI18n('whatsapp.noGroupsFound') || 'No groups found',
        info: getI18n('whatsapp.showingGroups') || 'Showing _START_ to _END_ of _TOTAL_ groups',
        infoEmpty: getI18n('whatsapp.noGroupsAvailable') || 'No groups available',
        infoFiltered: '(' + (getI18n('whatsapp.filteredFrom') || 'filtered from') + ' _MAX_ ' + (getI18n('whatsapp.totalGroups') || 'total groups') + ')'
      }
    });
  }

  async function refreshGroupsTable() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';
    if (!RANDOM_ACCESS) {
      console.warn('RANDOM_ACCESS not available for refreshGroupsTable');
      return;
    }

    // Initialize or refresh the DataTable
    if (window.groupsDataTable && $.fn.DataTable.isDataTable('#groups-table')) {
      window.groupsDataTable.draw();
    } else {
      initializeGroupsTable();
    }
  }

  function initializeAutoReplyTable() {
    const table = $('#auto-reply-table');
    if (!table.length) return;

    if ($.fn.DataTable.isDataTable('#auto-reply-table')) {
      table.DataTable().destroy();
    }

    window.autoReplyDataTable = table.DataTable({
      serverSide: true,
      processing: true,
      ajax: {
        url: '/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/auto-reply',
        type: 'GET',
        data: function (d) {
          return {
            page: Math.floor(d.start / d.length) + 1,
            limit: d.length
          };
        },
        dataSrc: function (json) {
          json.recordsTotal = json.pagination?.total_items || 0;
          json.recordsFiltered = json.pagination?.total_items || 0;
          return json.data || [];
        }
      },
      columns: [
        { data: 'id', title: 'ID', width: '60px', className: 'text-center' },
        {
          data: 'keywords',
          title: 'Keywords',
          width: '180px',
          render: function (data, type, row) {
            if (!data || data === '-') return '<span class="text-muted">-</span>';
            const truncated = data.length > 25 ? data.substring(0, 25) + '...' : data;
            return `<span title="${data.replace(/"/g, '&quot;')}" data-bs-toggle="tooltip">${truncated}</span>`;
          }
        },
        {
          data: 'reply_text',
          title: 'Reply',
          width: '200px',
          render: function (data, type, row) {
            if (!data || data === '-') return '<span class="text-muted">-</span>';
            const truncated = data.length > 35 ? data.substring(0, 35) + '...' : data;
            return `<span title="${data.replace(/"/g, '&quot;')}" data-bs-toggle="tooltip">${truncated}</span>`;
          }
        },
        {
          data: 'language',
          title: 'Language',
          width: '100px',
          render: function (data, type, row) {
            return `<span class="badge bg-primary">${data || '-'}</span>`;
          }
        },
        {
          data: 'lang_code',
          title: 'Lang Code',
          width: '90px',
          className: 'text-center',
          render: function (data, type, row) {
            return `<code>${data || '-'}</code>`;
          }
        },
        {
          data: 'for_user_type',
          title: 'User Type',
          width: '120px',
          render: function (data, type, row) {
            return `<span class="badge bg-secondary">${data || '-'}</span>`;
          }
        },
        {
          data: 'user_of',
          title: 'User Of',
          width: '140px',
          render: function (data, type, row) {
            return `<span class="badge bg-info">${data || '-'}</span>`;
          }
        },
        {
          data: null,
          title: 'Actions',
          orderable: false,
          searchable: false,
          width: '120px',
          className: 'text-center',
          render: function (data, type, row) {
            return `
              <div class="btn-group btn-group-sm">
                <button class="btn btn-warning" onclick="WhatsAppBotManager.editAutoReply(${row.id})" title="Edit">
                  <i class="fas fa-edit"></i>
                </button>
                <button class="btn btn-danger" onclick="WhatsAppBotManager.deleteAutoReply(${row.id})" title="Delete">
                  <i class="fas fa-trash"></i>
                </button>
              </div>
            `;
          }
        }
      ],
      order: [[0, 'desc']],
      pageLength: 10,
      lengthMenu: [[10, 25, 50, 100], [10, 25, 50, 100]],
      language: {
        processing: '<i class="fas fa-spinner fa-spin text-primary"></i> Loading...',
        emptyTable: '<div class="text-center p-3"><i class="fas fa-robot fa-2x text-muted mb-2"></i><p class="text-muted mb-0">No auto-reply rules found</p></div>',
        zeroRecords: '<div class="text-center p-3"><i class="fas fa-search fa-2x text-muted mb-2"></i><p class="text-muted mb-0">No matching rules</p></div>',
        lengthMenu: '_MENU_ entries per page',
        paginate: {
          first: '<i class="fas fa-angle-double-left"></i>',
          last: '<i class="fas fa-angle-double-right"></i>',
          next: '<i class="fas fa-angle-right"></i>',
          previous: '<i class="fas fa-angle-left"></i>'
        }
      },
      drawCallback: function () {
        $('[data-bs-toggle="tooltip"]').tooltip();
      }
    });
  }

  function refreshAutoReplyTable() {
    if (window.autoReplyDataTable) {
      window.autoReplyDataTable.ajax.reload();
    } else {
      initializeAutoReplyTable();
    }
  }

  async function loadLanguagesForAutoReply() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp/languages`);

      if (!response.ok) {
        const errorData = await response.json();
        console.error('Failed to load languages - Status:', response.status, 'Error:', errorData);
        return;
      }

      const result = await response.json();
      const languages = result.data || result.languages || [];

      const select = document.getElementById('auto-reply-language');
      if (select) {
        // Clear existing options except the placeholder
        select.innerHTML = '<option value="">-- Select Language --</option>';

        // Add language options
        languages.forEach(lang => {
          const option = document.createElement('option');
          option.value = lang.id;
          option.textContent = lang.name || lang.language_name || 'Unknown';
          select.appendChild(option);
        });
      }
    } catch (error) {
      console.error('Failed to load languages:', error);
    }
  }

  function showAddAutoReplyModal() {
    const form = document.getElementById('auto-reply-form');
    if (form) {
      form.reset();
      form.dataset.ruleId = '';
    }
    const modalTitle = document.getElementById('autoReplyModalTitle');
    if (modalTitle) modalTitle.textContent = 'Add Auto Reply Rule';

    if (!autoReplyModal) {
      const modalEl = document.getElementById('autoReplyModal');
      if (modalEl) autoReplyModal = new bootstrap.Modal(modalEl);
    }

    // Load languages before showing modal
    loadLanguagesForAutoReply();

    if (autoReplyModal) autoReplyModal.show();
  }

  async function editAutoReply(id) {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    // Show confirmation dialog
    const confirmResult = await Swal.fire({
      icon: 'question',
      title: getI18n('whatsapp.confirmEdit'),
      text: getI18n('whatsapp.confirmEditRule'),
      showCancelButton: true,
      confirmButtonText: getI18n('whatsapp.confirmEditButton'),
      cancelButtonText: getI18n('table.cancelButton'),
      confirmButtonColor: '#3085d6'
    });

    if (!confirmResult.isConfirmed) return;

    try {
      // Load languages first
      await loadLanguagesForAutoReply();

      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp/auto-reply/${id}`);
      const result = await response.json();
      const rule = result.data || result;

      document.getElementById('auto-reply-keywords').value = rule.keywords || '';
      document.getElementById('auto-reply-text').value = rule.reply_text || '';
      document.getElementById('auto-reply-user-type').value = rule.for_user_type || '';
      document.getElementById('auto-reply-user-of').value = rule.user_of || '';
      document.getElementById('auto-reply-language').value = rule.language_id || '';

      const form = document.getElementById('auto-reply-form');
      if (form) form.dataset.ruleId = id;

      const modalTitle = document.getElementById('autoReplyModalTitle');
      if (modalTitle) modalTitle.textContent = 'Edit Auto Reply Rule';

      if (!autoReplyModal) {
        const modalEl = document.getElementById('autoReplyModal');
        if (modalEl) autoReplyModal = new bootstrap.Modal(modalEl);
      }
      if (autoReplyModal) autoReplyModal.show();
    } catch (error) {
      console.error('Failed to load auto-reply rule:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.ruleLoadFailed')
      });
    }
  }

  async function saveAutoReply() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    const form = document.getElementById('auto-reply-form');
    const id = form?.dataset.ruleId;

    const keywords = document.getElementById('auto-reply-keywords')?.value?.trim();
    const replyText = document.getElementById('auto-reply-text')?.value?.trim();
    const languageId = document.getElementById('auto-reply-language')?.value?.trim();

    // Validate required fields
    if (!keywords) {
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: 'Keywords are required!'
      });
      return;
    }

    if (!replyText) {
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: 'Reply message is required!'
      });
      return;
    }

    if (!languageId) {
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: 'Language is required!'
      });
      return;
    }

    const ruleData = {
      keywords: keywords,
      reply_text: replyText,
      language_id: parseInt(languageId),
      for_user_type: document.getElementById('auto-reply-user-type')?.value,
      user_of: document.getElementById('auto-reply-user-of')?.value
    };

    try {
      const url = id ? `/api/v1/${RANDOM_ACCESS}/tab-whatsapp/auto-reply/${id}` : `/api/v1/${RANDOM_ACCESS}/tab-whatsapp/auto-reply`;
      const method = id ? 'PUT' : 'POST';

      const response = await fetch(url, {
        method: method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(ruleData)
      });

      const data = await response.json();

      if (data.success) {
        Swal.fire({
          icon: 'success',
          title: getI18n('table.successTitle'),
          text: getI18n('whatsapp.ruleSaveSuccess'),
          timer: 2000
        });
        if (autoReplyModal) autoReplyModal.hide();
        if (window.autoReplyDataTable) {
          window.autoReplyDataTable.ajax.reload();
        } else {
          refreshAutoReplyTable();
        }
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('table.errorTitle'),
          text: data.message || getI18n('whatsapp.ruleSaveFailed')
        });
      }
    } catch (error) {
      console.error('Failed to save auto-reply rule:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.ruleSaveFailed')
      });
    }
  }

  async function deleteAutoReply(id) {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    // Show confirmation dialog
    const confirmResult = await Swal.fire({
      icon: 'warning',
      title: getI18n('whatsapp.confirmDelete'),
      text: getI18n('whatsapp.confirmDeleteText'),
      showCancelButton: true,
      confirmButtonText: getI18n('whatsapp.confirmDeleteButton'),
      cancelButtonText: getI18n('table.cancelButton'),
      confirmButtonColor: '#d33'
    });

    if (!confirmResult.isConfirmed) return;

    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp/auto-reply/${id}`, {
        method: 'DELETE'
      });

      const data = await response.json();

      if (data.success) {
        Swal.fire({
          icon: 'success',
          title: getI18n('table.successTitle'),
          text: getI18n('whatsapp.ruleDeleteSuccess'),
          timer: 2000
        });
        if (window.autoReplyDataTable) {
          window.autoReplyDataTable.ajax.reload();
        } else {
          refreshAutoReplyTable();
        }
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('table.errorTitle'),
          text: data.message || getI18n('whatsapp.ruleDeleteFailed')
        });
      }
    } catch (error) {
      console.error('Failed to delete auto-reply rule:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.ruleDeleteFailed')
      });
    }
  }

  // Show image modal
  function showImageModal(imageUrl, title, jid) {
    const modalEl = document.getElementById('imageViewerModal');
    const imgEl = document.getElementById('imageViewerImg');
    const downloadEl = document.getElementById('imageViewerDownload');
    
    if (modalEl && imgEl && downloadEl) {
      // Keep original URL for display (WhatsApp URLs work for viewing)
      imgEl.src = imageUrl;
      
      // Use our API endpoint for download with the JID
      downloadEl.href = `/api/v1/${RANDOM_ACCESS}/tab-whatsapp/profile-picture/${encodeURIComponent(jid)}`;
      downloadEl.download = `profile-${title.replace(/[^a-zA-Z0-9]/g, '-')}.jpg`;
      
      const modal = new bootstrap.Modal(modalEl);
      modal.show();
    }
  }
  async function showGroupDetails(jid) {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    // Show loading indicator
    Swal.fire({
      title: getI18n('common.loading') || 'Loading...',
      text: getI18n('whatsapp.loadingGroupDetails') || 'Loading group details...',
      allowOutsideClick: false,
      didOpen: () => {
        Swal.showLoading();
      }
    });

    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp/groups/${encodeURIComponent(jid)}`);
      const group = await response.json();

      const detailsContent = document.getElementById('group-details-content');
      if (detailsContent) {
        // Group photo display
        let photoHtml = '';
        if (group.photo_url) {
          photoHtml = `
            <div class="col-12 mb-3">
              <strong data-i18n="whatsapp.groupPhoto">Group Photo:</strong><br>
              <img src="${group.photo_url}" alt="Group Photo" class="img-thumbnail mt-2" style="max-width: 150px; max-height: 150px; cursor: pointer;" onclick="WhatsAppBotManager.showImageModal('${group.photo_url}', '${group.name || 'Group Photo'}', '${group.jid}')">
            </div>
          `;
        }

        // Group settings
        let settingsHtml = '';
        if (group.settings) {
          const settings = group.settings;
          settingsHtml = `
            <div class="col-12 mb-3">
              <strong data-i18n="whatsapp.groupSettings">Group Settings:</strong><br>
              <div class="mt-2">
                <span class="badge ${settings.locked ? 'bg-warning' : 'bg-success'} me-2">
                  ${settings.locked ? 'Locked' : 'Unlocked'}
                </span>
                <span class="badge ${settings.announcement_only ? 'bg-info' : 'bg-secondary'} me-2">
                  ${settings.announcement_only ? 'Announcement Only' : 'All Can Send'}
                </span>
                ${settings.ephemeral ? `<span class="badge bg-primary">Ephemeral (${settings.ephemeral_duration}s)</span>` : ''}
              </div>
            </div>
          `;
        }

        detailsContent.innerHTML = `
          <div class="row">
            ${photoHtml}
            <div class="col-md-6 mb-3">
              <strong data-i18n="whatsapp.groupName">Name:</strong> ${group.name && group.name !== '-' ? group.name : 'Unknown'}
            </div>
            <div class="col-md-6 mb-3">
              <strong data-i18n="whatsapp.jid">JID:</strong> <code>${group.jid && group.jid !== '-' ? group.jid : 'Unknown'}</code>
            </div>
            <div class="col-md-6 mb-3">
              <strong data-i18n="whatsapp.owner">Owner:</strong> <code>${group.owner_jid && group.owner_jid !== '-' ? group.owner_jid : 'Unknown'}</code>
            </div>
            <div class="col-md-6 mb-3">
              <strong data-i18n="whatsapp.participantsCount">Participants:</strong> ${group.participants?.length || 0}
            </div>
            ${settingsHtml}
            <div class="col-md-6 mb-3">
              <strong data-i18n="whatsapp.description">Description:</strong> ${group.description || group.topic || 'No description'}
            </div>
            <div class="col-md-6 mb-3">
              <strong data-i18n="whatsapp.descriptionSetAt">Description Set At:</strong> ${group.topic_set_at ? formatDate(new Date(group.topic_set_at * 1000)) : 'Unknown'}
            </div>
          </div>
        `;
      }

      // Show participants table container
      const tableContainer = document.getElementById('group-participants-table-container');
      if (tableContainer) tableContainer.style.display = 'block';

      // Initialize or refresh participants DataTable
      const participantsTable = $('#group-participants-table');
      if ($.fn.DataTable.isDataTable('#group-participants-table')) {
        participantsTable.DataTable().clear().destroy();
      }

      if (group.participants && group.participants.length > 0) {
        participantsTable.DataTable({
          data: group.participants,
          columns: [
            {
              data: 'profile_picture_url',
              render: function (data, type, row) {
                if (data && data !== '') {
                  return `<img src="${data}" alt="Profile" class="rounded-circle" style="width: 40px; height: 40px; object-fit: cover; cursor: pointer;" onclick="WhatsAppBotManager.showImageModal('${data}', '${row.display_name || 'Profile Picture'}', '${row.jid}')">`;
                }
                return `<div class="bg-secondary text-white rounded-circle d-flex align-items-center justify-content-center" style="width: 40px; height: 40px;">
                  <i class="fas fa-user"></i>
                </div>`;
              },
              orderable: false
            },
            {
              data: 'display_name',
              render: function (data, type, row) {
                return data || row.jid || '-';
              }
            },
            {
              data: 'phone_number',
              render: function (data) {
                return data ? `<code>${data}</code>` : '-';
              }
            },
            {
              data: 'jid',
              render: function (data) {
                return data ? `<code class="small">${data}</code>` : '-';
              }
            },
            {
              data: null,
              render: function (data, type, row) {
                if (row.is_super_admin) {
                  return `<span class="badge bg-danger"><i class="fas fa-crown"></i> Super Admin</span>`;
                } else if (row.is_admin) {
                  return `<span class="badge bg-warning"><i class="fas fa-shield-alt"></i> Admin</span>`;
                }
                return `<span class="badge bg-secondary"><i class="fas fa-user"></i> Member</span>`;
              },
              orderable: false
            }
          ],
          pageLength: 10,
          lengthMenu: [[5, 10, 25, 50], [5, 10, 25, 50]],
          order: [[4, 'desc'], [1, 'asc']], // Sort by role (admin first), then name
          language: {
            processing: '<i class="fa fa-spinner fa-spin"></i> ' + getI18n('common.loading'),
            lengthMenu: '_MENU_ ' + (getI18n('whatsapp.entriesPerPage') || 'entries per page'),
            zeroRecords: getI18n('whatsapp.noParticipants') || 'No participants found',
            info: getI18n('whatsapp.showingParticipants') || 'Showing _START_ to _END_ of _TOTAL_ participants',
            infoEmpty: getI18n('whatsapp.noParticipants') || 'No participants',
            search: getI18n('table.search') || 'Search:',
            searchPlaceholder: getI18n('table.searchPlaceholder') || 'Type to search...',
            paginate: {
              first: '<i class="fas fa-angle-double-left"></i>',
              last: '<i class="fas fa-angle-double-right"></i>',
              next: '<i class="fas fa-angle-right"></i>',
              previous: '<i class="fas fa-angle-left"></i>'
            }
          }
        });
      }

      if (!groupDetailsModal) {
        const modalEl = document.getElementById('groupDetailsModal');
        if (modalEl) groupDetailsModal = new bootstrap.Modal(modalEl);
      }
      if (groupDetailsModal) groupDetailsModal.show();

      // Close loading indicator
      Swal.close();
    } catch (error) {
      console.error('Failed to load group details:', error);
      // Close loading indicator
      Swal.close();
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: 'Failed to load group details'
      });
    }
  }

  // Quick action methods
  async function testConnection() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';
    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp/status`);
      const data = await response.json();

      if (data.connected) {
        Swal.fire({
          icon: 'success',
          title: getI18n('whatsapp.test_connection_success') || 'Connection Test Passed',
          text: getI18n('whatsapp.connection_is_active') || 'Your WhatsApp connection is active and working properly.',
          timer: 2000
        });
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('whatsapp.test_connection_failed') || 'Connection Test Failed',
          text: getI18n('whatsapp.connection_not_active') || 'WhatsApp is not connected.'
        });
      }
    } catch (error) {
      Swal.fire({
        icon: 'error',
        title: getI18n('error') || 'Error',
        text: getI18n('whatsapp.test_connection_error') || 'Failed to test connection'
      });
    }
  }

  async function refreshContactsCount() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp/contacts-count`);
      const data = await response.json();

      if (data.success) {
        const count = data.count || 0;
        Swal.fire({
          icon: 'success',
          title: getI18n('whatsapp.contacts_info') || 'Contacts Information',
          html: `<div class="text-center">
            <h3 class="text-primary mb-3">${count}</h3>
            <p class="text-muted">WhatsApp contacts synced</p>
          </div>`,
          timer: 3000
        });
      } else {
        Swal.fire({
          icon: 'info',
          title: getI18n('whatsapp.contacts_info') || 'Contacts Information',
          text: data.message || getI18n('whatsapp.contacts_sync_info') || 'No contacts found or not synced yet.',
          timer: 3000
        });
      }
    } catch (error) {
      console.error('Failed to fetch contacts count:', error);
      Swal.fire({
        icon: 'info',
        title: getI18n('whatsapp.contacts_info') || 'Contacts Information',
        text: getI18n('whatsapp.contacts_sync_info') || 'This feature will sync and display your WhatsApp contacts count.',
        timer: 3000
      });
    }
  }

  async function checkBatteryStatus() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp/phone-status`);
      const data = await response.json();

      if (data.success) {
        Swal.fire({
          icon: 'info',
          title: getI18n('whatsapp.phone_status') || 'Phone Status',
          html: `
            <div class="text-start">
              <p><strong>Battery:</strong> ${data.battery || 'N/A'}%</p>
              <p><strong>Status:</strong> ${data.status || 'Connected'}</p>
              <p><strong>Device:</strong> ${data.device || 'Unknown'}</p>
            </div>
          `,
          timer: 4000
        });
      } else {
        Swal.fire({
          icon: 'info',
          title: getI18n('whatsapp.phone_status') || 'Phone Status',
          text: getI18n('whatsapp.phone_status_not_available') || 'Battery and phone status information is not available through the API yet.',
          timer: 3000
        });
      }
    } catch (error) {
      console.error('Failed to fetch phone status:', error);
      Swal.fire({
        icon: 'info',
        title: getI18n('whatsapp.phone_status') || 'Phone Status',
        text: getI18n('whatsapp.phone_status_not_available') || 'Battery and phone status information is not available through the API yet.',
        timer: 3000
      });
    }
  }

  // Utility functions
  function truncateText(text, maxLength) {
    if (!text) return '-';
    return text.length > maxLength ? text.substring(0, maxLength) + '...' : text;
  }

  function formatDate(dateString) {
    if (!dateString) return '-';
    const date = new Date(dateString);
    return date.toLocaleString();
  }

  async function loadStatistics() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';
    if (!RANDOM_ACCESS) {
      console.warn('RANDOM_ACCESS not available for loadStatistics');
      return;
    }

    try {
      // Load language count
      const langResp = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/languages/count');
      if (langResp.ok) {
        const langData = await langResp.json();
        document.getElementById('stat-languages').textContent = langData.count || 0;
      }

      // Load languages list
      const langListResp = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/languages');
      if (langListResp.ok) {
        const langListData = await langListResp.json();
        const languages = langListData.data || [];
        const container = document.getElementById('languages-list');
        if (container) {
          container.innerHTML = languages.map(lang =>
            `<div class="col-md-3 mb-2">
              <span class="badge bg-label-primary me-1"><i class="fas fa-check-circle"></i> ${lang.name}</span>
            </div>`
          ).join('');
        }
      }

      // Load messages count
      const messagesResp = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/messages?page=1&limit=1');
      if (messagesResp.ok) {
        const messagesData = await messagesResp.json();
        const totalMessages = messagesData.pagination?.total_items || 0;
        document.getElementById('stat-total-messages').textContent = totalMessages;
      }

      // Load incoming count
      const incomingResp = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/incoming?page=1&limit=1');
      if (incomingResp.ok) {
        const incomingData = await incomingResp.json();
        const totalIncoming = incomingData.pagination?.total_items || 0;
        document.getElementById('stat-incoming-messages').textContent = totalIncoming;
      }

      // Load groups count
      const groupsCountResp = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/groups/count');
      if (groupsCountResp.ok) {
        const groupsCountData = await groupsCountResp.json();
        const totalGroups = groupsCountData.count || 0;
        document.getElementById('stat-groups').textContent = totalGroups;
      }
    } catch (error) {
      console.error('Failed to load statistics:', error);
    }
  }

  function getStatusColor(status) {
    const colors = {
      'sent': 'info',
      'delivered': 'success',
      'read': 'primary',
      'failed': 'danger'
    };
    return colors[status] || 'secondary';
  }

  function getMessageTypeColor(type) {
    const colors = {
      'text': 'primary',
      'image': 'success',
      'video': 'info',
      'document': 'warning',
      'audio': 'secondary',
      'sticker': 'danger',
      'location': 'dark',
      'contact': 'light',
      'reaction': 'primary'
    };
    return colors[type] || 'secondary';
  }

  // Cleanup on tab change
  function cleanup() {
    if (connectionStatusInterval) {
      clearInterval(connectionStatusInterval);
      connectionStatusInterval = null;
    }
  }

  // Sync groups from WhatsApp
  async function syncGroups() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';
    if (!RANDOM_ACCESS) return;

    try {
      const result = await Swal.fire({
        icon: 'question',
        title: getI18n('whatsapp.syncGroupsTitle') || 'Sync Groups',
        text: getI18n('whatsapp.syncGroupsText') || 'This will sync all groups from your WhatsApp account to the database.',
        showCancelButton: true,
        confirmButtonText: getI18n('whatsapp.sync') || 'Sync',
        cancelButtonText: getI18n('table.cancelButton'),
        confirmButtonColor: '#25D366'
      });

      if (!result.isConfirmed) return;

      Swal.fire({
        icon: 'info',
        title: getI18n('whatsapp.syncing') || 'Syncing...',
        text: getI18n('whatsapp.syncingGroups') || 'Please wait while we sync your groups.',
        allowOutsideClick: false,
        didOpen: () => {
          Swal.showLoading();
        }
      });

      const response = await fetch('/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp/groups/sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });

      if (!response.ok) throw new Error('Sync failed');

      const data = await response.json();

      Swal.fire({
        icon: 'success',
        title: getI18n('whatsapp.syncSuccess') || 'Sync Started',
        text: data.message || getI18n('whatsapp.syncSuccessText') || 'Groups are being synced. Please refresh in a few seconds.',
        confirmButtonColor: '#25D366'
      });

      // Refresh statistics and table after a delay
      setTimeout(() => {
        loadStatistics();
        refreshGroupsTable();
      }, 3000);

    } catch (error) {
      console.error('Sync groups error:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('whatsapp.syncFailed') || 'Sync Failed',
        text: getI18n('whatsapp.syncFailedText') || 'Failed to sync groups. Please try again.',
        confirmButtonColor: '#d33'
      });
    }
  }

  // Public API
  return {
    init,
    checkConnectionStatus,
    loadStatistics,
    showConnectModal,
    connectWhatsApp,
    disconnectWhatsApp,
    logoutWhatsApp,
    refreshQRCode,
    toggleMessageFields,
    toggleGroupMode,
    refreshMessagesTable,
    refreshIncomingTable,
    refreshGroupsTable,
    syncGroups,
    refreshAutoReplyTable,
    showAddAutoReplyModal,
    editAutoReply,
    saveAutoReply,
    deleteAutoReply,
    showGroupDetails,
    showImageModal,
    testConnection,
    refreshContactsCount,
    checkBatteryStatus,
    cleanup
  };
})();

// WhatsApp User Management Module
const WhatsAppUserManager = (function () {
  // Private variables
  let userModal, viewUserModal;
  let currentPage = 1;
  let currentFilters = {};
  let RANDOM_ACCESS = '';

  // Initialize the User Management tab
  function init() {
    // Get RANDOM_ACCESS from window if available
    RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    if (!RANDOM_ACCESS) {
      console.error('RANDOM_ACCESS not available yet, will retry on tab load');
      return;
    }

    const userModalEl = document.getElementById('userModal');
    const viewUserModalEl = document.getElementById('viewUserModal');

    if (userModalEl) userModal = new bootstrap.Modal(userModalEl);
    if (viewUserModalEl) viewUserModal = new bootstrap.Modal(viewUserModalEl);

    // Initialize DataTable with server-side processing
    initializeDataTable();
    loadStatistics();

    // Setup filter change handlers
    $('#user-type-filter, #status-filter').on('change', function () {
      if (window.usersDataTable) {
        window.usersDataTable.ajax.reload();
      }
    });

    // Search on Enter key or button click
    const searchInput = document.getElementById('search-input');
    if (searchInput) {
      searchInput.removeEventListener('keypress', handleSearchKeypress);
      searchInput.addEventListener('keypress', handleSearchKeypress);
    }
  }

  function initializeDataTable() {
    const table = $('#users-table');
    if (!table.length) return;

    // Destroy existing DataTable if it exists
    if ($.fn.DataTable.isDataTable('#users-table')) {
      table.DataTable().destroy();
    }

    // Initialize DataTable with server-side processing
    window.usersDataTable = table.DataTable({
      serverSide: true,
      processing: true,
      ajax: {
        url: '/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp-user-management/users/datatable',
        type: 'POST',
        contentType: 'application/json',
        dataType: 'json',
        data: function (d) {
          // Add custom filters to the DataTables request
          const userTypeFilter = $('#user-type-filter').val();
          const statusFilter = $('#status-filter').val();
          const userOfFilter = $('#user-of-filter').val();
          if (userTypeFilter) d.user_type = userTypeFilter;
          if (statusFilter) d.status = statusFilter;
          if (userOfFilter) d.user_of = userOfFilter;
          return JSON.stringify(d);
        },
        error: function (xhr, error, code) {
          console.error('DataTable AJAX error:', xhr.responseText, error);
        }
      },
      columns: [
        {
          data: 'id',
          title: 'ID',
          width: '60px',
          className: 'text-center'
        },
        {
          data: 'full_name',
          title: 'Full Name',
          render: function (data, type, row) {
            return data || '<span class="text-muted">-</span>';
          }
        },
        {
          data: 'phone_number',
          title: 'Phone Number',
          width: '140px',
          render: function (data, type, row) {
            if (!data) return '<span class="text-muted">-</span>';
            return `<span class="badge bg-light text-dark"><i class="fas fa-phone me-1"></i>${data}</span>`;
          }
        },
        {
          data: 'email',
          title: 'Email',
          render: function (data, type, row) {
            if (!data) return '<span class="text-muted">-</span>';
            return `<small>${data}</small>`;
          }
        },
        {
          data: 'user_type',
          title: 'User Type',
          width: '130px',
          render: function (data, type, row) {
            const typeMap = {
              'common': '<span class="badge bg-secondary">Common</span>',
              'super_user': '<span class="badge bg-primary">Super User</span>',
              'client_user': '<span class="badge bg-info">Client User</span>',
              'user_administrator': '<span class="badge bg-danger">Administrator</span>'
            };
            return typeMap[data] || '<span class="text-muted">-</span>';
          }
        },
        {
          data: 'user_of',
          title: 'User Of',
          width: '150px',
          render: function (data, type, row) {
            const userOfMap = {
              'company_employee': '<span class="badge bg-success">Company Employee</span>',
              'client_company_employee': '<span class="badge bg-warning text-dark">Client Company</span>'
            };
            return userOfMap[data] || '<span class="text-muted">-</span>';
          }
        },
        {
          data: 'allowed_chats',
          title: 'Chat Mode',
          width: '100px',
          className: 'text-center',
          render: function (data, type, row) {
            const chatMap = {
              'direct': '<i class="fas fa-user text-info" title="Direct"></i>',
              'group': '<i class="fas fa-users text-warning" title="Group"></i>',
              'both': '<i class="fas fa-user-friends text-success" title="Both"></i>'
            };
            return chatMap[data] || '<span class="text-muted">-</span>';
          }
        },
        {
          data: 'max_daily_quota',
          title: 'Quota',
          width: '80px',
          className: 'text-center',
          render: function (data, type, row) {
            return `<span class="badge bg-primary">${data || 0}</span>`;
          }
        },
        {
          data: 'is_banned',
          title: 'Status',
          width: '90px',
          className: 'text-center',
          render: function (data, type, row) {
            if (data) {
              return '<span class="badge bg-danger"><i class="fas fa-ban me-1"></i>Banned</span>';
            }
            return '<span class="badge bg-success"><i class="fas fa-check-circle me-1"></i>Active</span>';
          }
        },
        {
          data: null,
          title: 'Actions',
          orderable: false,
          searchable: false,
          width: '160px',
          className: 'text-center',
          render: function (data, type, row) {
            return `
              <div class="btn-group btn-group-sm" role="group">
                <button class="btn btn-info" onclick="WhatsAppUserManager.viewUser(${row.id})" title="View Details">
                  <i class="fas fa-eye"></i>
                </button>
                <button class="btn btn-warning" onclick="WhatsAppUserManager.editUser(${row.id})" title="Edit User">
                  <i class="fas fa-edit"></i>
                </button>
                <button class="btn btn-${row.is_banned ? 'success' : 'danger'}" 
                        onclick="WhatsAppUserManager.toggleBanUser(${row.id}, ${!row.is_banned})"
                        title="${row.is_banned ? 'Unban User' : 'Ban User'}">
                  <i class="fas fa-${row.is_banned ? 'user-check' : 'user-slash'}"></i>
                </button>
                <button class="btn btn-danger" onclick="WhatsAppUserManager.deleteUser(${row.id})" title="Delete User">
                  <i class="fas fa-trash-alt"></i>
                </button>
              </div>
            `;
          }
        }
      ],
      order: [[0, 'desc']], // Default sort by ID descending
      pageLength: 20,
      lengthMenu: [[10, 20, 50, 100], [10, 20, 50, 100]],
      language: {
        processing: '<i class="fas fa-spinner fa-spin text-primary"></i> Loading...',
        emptyTable: '<div class="text-center p-3"><i class="fas fa-users fa-2x text-muted mb-2"></i><p class="text-muted mb-0">No users found</p></div>',
        zeroRecords: '<div class="text-center p-3"><i class="fas fa-search fa-2x text-muted mb-2"></i><p class="text-muted mb-0">No matching users found</p></div>',
        search: '_INPUT_',
        searchPlaceholder: 'Search users...',
        lengthMenu: 'Show _MENU_',
        info: 'Showing _START_ to _END_ of _TOTAL_ users',
        infoEmpty: 'No users available',
        infoFiltered: '(filtered from _MAX_ total)',
        paginate: {
          first: '<i class="fas fa-angle-double-left"></i>',
          last: '<i class="fas fa-angle-double-right"></i>',
          next: '<i class="fas fa-angle-right"></i>',
          previous: '<i class="fas fa-angle-left"></i>'
        }
      },
      dom: '<"row mb-3"<"col-sm-12 col-md-6"l><"col-sm-12 col-md-6"f>>' +
        '<"row"<"col-sm-12"tr>>' +
        '<"row mt-3"<"col-sm-12 col-md-5"i><"col-sm-12 col-md-7"p>>',
      responsive: true,
      autoWidth: false,
      drawCallback: function () {
        // Add tooltips after table draw
        $('[title]').tooltip();
      }
    });
  }

  function handleSearchKeypress(e) {
    if (e.key === 'Enter') {
      searchUsers();
    }
  }

  async function loadStatistics() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    try {
      // Build query string with current filters
      const params = new URLSearchParams();
      if (currentFilters.user_type) params.append('user_type', currentFilters.user_type);
      if (currentFilters.user_of) params.append('user_of', currentFilters.user_of);
      if (currentFilters.status) params.append('status', currentFilters.status);

      const queryString = params.toString();
      const url = '/api/v1/' + RANDOM_ACCESS + '/tab-whatsapp-user-management/statistics' +
        (queryString ? '?' + queryString : '');

      const response = await fetch(url);
      const data = await response.json();

      if (data) {
        const totalEl = document.getElementById('stat-total');
        const activeEl = document.getElementById('stat-active');
        const registeredEl = document.getElementById('stat-registered');
        const bannedEl = document.getElementById('stat-banned');

        if (totalEl) totalEl.textContent = data.total || 0;
        if (activeEl) activeEl.textContent = data.active || 0;
        if (registeredEl) registeredEl.textContent = data.registered || 0;
        if (bannedEl) bannedEl.textContent = data.banned || 0;
      }
    } catch (error) {
      console.error('Failed to load statistics:', error);
    }
  }

  function searchUsers() {
    // Store current filters
    currentFilters = {
      user_type: $('#user-type-filter').val(),
      user_of: $('#user-of-filter').val(),
      status: $('#status-filter').val()
    };

    if (window.usersDataTable) {
      // Trigger table reload with filters
      window.usersDataTable.ajax.reload();
    }

    // Reload statistics after filtering
    loadStatistics();
  }

  function refreshUsersTable(page = 1) {
    // For DataTables, just reload the data
    if (window.usersDataTable) {
      window.usersDataTable.ajax.reload(null, false); // false = stay on current page
    }
  }

  function showAddUserModal() {
    const form = document.getElementById('user-form');
    if (form) {
      form.reset();
      form.dataset.userId = '';
    }
    const modalTitle = document.getElementById('userModalLabel');
    if (modalTitle) modalTitle.textContent = 'Add WhatsApp User';

    if (!userModal) {
      const modalEl = document.getElementById('userModal');
      if (modalEl) userModal = new bootstrap.Modal(modalEl);
    }
    if (userModal) userModal.show();
  }

  async function editUser(id) {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    // Show confirmation dialog
    const confirmResult = await Swal.fire({
      icon: 'question',
      title: getI18n('whatsapp.confirmEditUser'),
      text: getI18n('whatsapp.confirmEditUserText'),
      showCancelButton: true,
      confirmButtonText: getI18n('whatsapp.confirmEditButton'),
      cancelButtonText: getI18n('table.cancelButton'),
      confirmButtonColor: '#3085d6'
    });

    if (!confirmResult.isConfirmed) return;

    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp-user-management/users/${id}`);
      const user = await response.json();

      // Populate form
      const fields = [
        'full-name', 'email', 'phone-number', 'allowed-chats',
        'allowed-types', 'max-daily-quota', 'allowed-to-call',
        'use-bot', 'user-type', 'user-of', 'description'
      ];

      fields.forEach(field => {
        const el = document.getElementById(field);
        const key = field.split('-').map((word, i) =>
          i === 0 ? word.charAt(0).toUpperCase() + word.slice(1) :
            word.charAt(0).toUpperCase() + word.slice(1)
        ).join('');

        if (el && user[key] !== undefined) {
          if (el.type === 'checkbox') {
            el.checked = user[key];
          } else {
            el.value = user[key];
          }
        }
      });

      const form = document.getElementById('user-form');
      if (form) form.dataset.userId = id;

      const modalTitle = document.getElementById('userModalLabel');
      if (modalTitle) modalTitle.textContent = 'Edit WhatsApp User';

      if (!userModal) {
        const modalEl = document.getElementById('userModal');
        if (modalEl) userModal = new bootstrap.Modal(modalEl);
      }
      if (userModal) userModal.show();
    } catch (error) {
      console.error('Failed to load user:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: 'Failed to load user'
      });
    }
  }

  async function viewUser(id) {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp-user-management/users/${id}`);
      const user = await response.json();

      const detailsContent = document.getElementById('user-details-content');
      if (detailsContent) {
        detailsContent.innerHTML = `
          <div class="row mb-3">
            <div class="col-4"><strong>Full Name:</strong></div>
            <div class="col-8">${user.full_name || '-'}</div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>Email:</strong></div>
            <div class="col-8">${user.email || '-'}</div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>Phone:</strong></div>
            <div class="col-8">${user.phone_number || '-'}</div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>User Type:</strong></div>
            <div class="col-8">${user.user_type || '-'}</div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>User Of:</strong></div>
            <div class="col-8">${user.user_of || '-'}</div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>Allowed Chats:</strong></div>
            <div class="col-8">${user.allowed_chats || '-'}</div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>Max Daily Quota:</strong></div>
            <div class="col-8">${user.max_daily_quota || 0}</div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>Allowed To Call:</strong></div>
            <div class="col-8">
              <span class="badge bg-${user.allowed_to_call ? 'success' : 'danger'}">
                ${user.allowed_to_call ? 'Yes' : 'No'}
              </span>
            </div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>Use Bot:</strong></div>
            <div class="col-8">
              <span class="badge bg-${user.use_bot ? 'success' : 'danger'}">
                ${user.use_bot ? 'Yes' : 'No'}
              </span>
            </div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>Status:</strong></div>
            <div class="col-8">
              <span class="badge bg-${user.is_banned ? 'danger' : 'success'}">
                ${user.is_banned ? 'Banned' : 'Active'}
              </span>
            </div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>Registered:</strong></div>
            <div class="col-8">
              <span class="badge bg-${user.is_registered ? 'success' : 'secondary'}">
                ${user.is_registered ? 'Yes' : 'No'}
              </span>
            </div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>Description:</strong></div>
            <div class="col-8">${user.description || '-'}</div>
          </div>
          <div class="row mb-3">
            <div class="col-4"><strong>Created:</strong></div>
            <div class="col-8">${formatDate(user.created_at)}</div>
          </div>
        `;
      }

      if (!viewUserModal) {
        const modalEl = document.getElementById('viewUserModal');
        if (modalEl) viewUserModal = new bootstrap.Modal(modalEl);
      }
      if (viewUserModal) viewUserModal.show();
    } catch (error) {
      console.error('Failed to load user:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: 'Failed to load user'
      });
    }
  }

  async function saveUser() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    const form = document.getElementById('user-form');
    const id = form?.dataset.userId;

    const userData = {
      full_name: document.getElementById('user-full-name')?.value,
      email: document.getElementById('user-email')?.value,
      phone_number: document.getElementById('user-phone')?.value,
      allowed_chats: document.getElementById('allowed-chats')?.value,
      allowed_types: document.getElementById('allowed-types')?.value?.split(',').map(t => t.trim()),
      max_daily_quota: parseInt(document.getElementById('max-daily-quota')?.value) || 0,
      allowed_to_call: document.getElementById('allowed-to-call')?.checked || false,
      use_bot: document.getElementById('use-bot')?.checked || false,
      user_type: document.getElementById('user-type')?.value,
      user_of: document.getElementById('user-of')?.value,
      description: document.getElementById('user-description')?.value
    };

    try {
      const url = id ?
        `/api/v1/${RANDOM_ACCESS}/tab-whatsapp-user-management/users/${id}` :
        `/api/v1/${RANDOM_ACCESS}/tab-whatsapp-user-management/users`;
      const method = id ? 'PUT' : 'POST';

      const response = await fetch(url, {
        method: method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(userData)
      });

      const data = await response.json();

      if (data.success) {
        Swal.fire({
          icon: 'success',
          title: getI18n('table.successTitle'),
          text: getI18n('whatsapp.userSaveSuccess'),
          timer: 2000
        });
        if (userModal) userModal.hide();
        refreshUsersTable(currentPage);
        loadStatistics();
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('table.errorTitle'),
          text: data.message || getI18n('whatsapp.userSaveFailed')
        });
      }
    } catch (error) {
      console.error('Failed to save user:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.userSaveFailed')
      });
    }
  }

  async function toggleBanUser(id, isBanned) {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    const action = isBanned ? 'ban' : 'unban';
    const confirmKey = isBanned ? 'whatsapp.confirmBanUser' : 'whatsapp.confirmUnbanUser';
    const successKey = isBanned ? 'whatsapp.userBannedSuccess' : 'whatsapp.userUnbannedSuccess';
    const buttonKey = isBanned ? 'whatsapp.confirmBan' : 'whatsapp.confirmUnban';

    const confirmResult = await Swal.fire({
      icon: 'warning',
      title: getI18n('whatsapp.confirmAction'),
      text: getI18n(confirmKey),
      showCancelButton: true,
      confirmButtonText: getI18n(buttonKey),
      cancelButtonText: getI18n('table.cancelButton'),
      confirmButtonColor: '#d33'
    });

    if (!confirmResult.isConfirmed) return;

    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp-user-management/users/${id}/ban`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ is_banned: isBanned })
      });

      const data = await response.json();

      if (data.success) {
        Swal.fire({
          icon: 'success',
          title: getI18n('table.successTitle'),
          text: getI18n(successKey),
          timer: 2000
        });
        refreshUsersTable(currentPage);
        loadStatistics();
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('table.errorTitle'),
          text: data.message || getI18n('whatsapp.userSaveFailed')
        });
      }
    } catch (error) {
      console.error('Failed to update user:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.userSaveFailed')
      });
    }
  }

  async function deleteUser(id) {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    const confirmResult = await Swal.fire({
      icon: 'warning',
      title: getI18n('whatsapp.confirmDeleteUser'),
      text: getI18n('whatsapp.confirmDeleteText'),
      showCancelButton: true,
      confirmButtonText: getI18n('whatsapp.confirmDeleteButton'),
      cancelButtonText: getI18n('table.cancelButton'),
      confirmButtonColor: '#d33'
    });

    if (!confirmResult.isConfirmed) return;

    try {
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp-user-management/users/${id}`, {
        method: 'DELETE'
      });

      const data = await response.json();

      if (data.success) {
        Swal.fire({
          icon: 'success',
          title: getI18n('table.successTitle'),
          text: getI18n('whatsapp.userDeleteSuccess'),
          timer: 2000
        });
        refreshUsersTable(currentPage);
        loadStatistics();
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('table.errorTitle'),
          text: data.message || getI18n('whatsapp.userDeleteFailed')
        });
      }
    } catch (error) {
      console.error('Failed to delete user:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsapp.userDeleteFailed')
      });
    }
  }

  async function exportUsers() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    const params = new URLSearchParams(currentFilters);
    window.location.href = `/api/v1/${RANDOM_ACCESS}/tab-whatsapp-user-management/users/export?${params}`;
  }

  async function downloadImportTemplate() {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    try {
      // Fetch template data from backend
      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp-user-management/users/import-template`);
      const templateData = await response.json();

      if (!templateData || !templateData.headers) {
        throw new Error('Invalid template data');
      }

      // Build CSV content from backend data
      const headers = templateData.headers;
      const examples = templateData.examples || [];

      let csvRows = [];

      // Add header row
      csvRows.push(headers.join(','));

      // Add example rows
      examples.forEach(example => {
        const row = headers.map(header => {
          let value = example[header] || '';
          // Escape values that contain commas or quotes
          if (typeof value === 'string' && (value.includes(',') || value.includes('"'))) {
            value = `"${value.replace(/"/g, '""')}"`;
          }
          return value;
        });
        csvRows.push(row.join(','));
      });

      const csvContent = csvRows.join('\n');

      // Create and trigger download
      const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      const url = URL.createObjectURL(blob);

      link.setAttribute('href', url);
      link.setAttribute('download', 'whatsapp_users_import_template.csv');
      link.style.visibility = 'hidden';

      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);

      Swal.fire({
        icon: 'success',
        title: getI18n('table.successTitle'),
        text: getI18n('whatsappUserManagement.templateDownloaded'),
        timer: 2000,
        showConfirmButton: false
      });
    } catch (error) {
      console.error('Failed to download template:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsappUserManagement.templateDownloadFailed')
      });
    }
  }

  function showImportModal() {
    Swal.fire({
      title: getI18n('whatsappUserManagement.importUsers'),
      html: `
        <div class="mb-3">
          <label class="form-label">${getI18n('whatsappUserManagement.selectCSVFile')}</label>
          <input type="file" id="import-file-input" class="form-control" accept=".csv">
          <small class="text-muted d-block mt-2">
            ${getI18n('whatsappUserManagement.importHint')}
          </small>
        </div>
      `,
      showCancelButton: true,
      confirmButtonText: getI18n('whatsappUserManagement.import'),
      cancelButtonText: getI18n('table.cancelButton'),
      preConfirm: () => {
        const fileInput = document.getElementById('import-file-input');
        const file = fileInput?.files?.[0];

        if (!file) {
          Swal.showValidationMessage(getI18n('whatsappUserManagement.pleaseSelectFile'));
          return false;
        }

        if (!file.name.endsWith('.csv')) {
          Swal.showValidationMessage(getI18n('whatsappUserManagement.invalidFileType'));
          return false;
        }

        return file;
      }
    }).then((result) => {
      if (result.isConfirmed && result.value) {
        importUsers(result.value);
      }
    });
  }

  async function importUsers(file) {
    if (!RANDOM_ACCESS) RANDOM_ACCESS = window.RANDOM_ACCESS || '';

    // Show loading
    Swal.fire({
      title: getI18n('whatsappUserManagement.importing'),
      allowOutsideClick: false,
      didOpen: () => {
        Swal.showLoading();
      }
    });

    try {
      const formData = new FormData();
      formData.append('file', file);

      const response = await fetch(`/api/v1/${RANDOM_ACCESS}/tab-whatsapp-user-management/users/import`, {
        method: 'POST',
        body: formData
      });

      const data = await response.json();

      if (data.success) {
        const details = getI18n('whatsappUserManagement.importSuccessDetails')
          .replace('{created}', data.created || 0)
          .replace('{updated}', data.updated || 0)
          .replace('{failed}', data.failed || 0);

        Swal.fire({
          icon: 'success',
          title: getI18n('table.successTitle'),
          html: `<p>${getI18n('whatsappUserManagement.importSuccess')}</p><p class="text-muted">${details}</p>`,
          timer: 3000
        });
        refreshUsersTable();
        loadStatistics();
      } else {
        Swal.fire({
          icon: 'error',
          title: getI18n('whatsappUserManagement.importFailed'),
          text: data.message || getI18n('whatsappUserManagement.importError')
        });
      }
    } catch (error) {
      console.error('Failed to import users:', error);
      Swal.fire({
        icon: 'error',
        title: getI18n('table.errorTitle'),
        text: getI18n('whatsappUserManagement.importError')
      });
    }
  }

  function formatDate(dateString) {
    if (!dateString) return '-';
    try {
      // Handle ISO 8601 format and other standard formats
      const date = new Date(dateString);
      if (isNaN(date.getTime())) {
        return dateString; // Return original if can't parse
      }
      return date.toLocaleString('en-US', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      });
    } catch (e) {
      return dateString || '-';
    }
  }

  // Public API
  return {
    init,
    loadStatistics,
    refreshUsersTable,
    searchUsers,
    showAddUserModal,
    editUser,
    viewUser,
    saveUser,
    toggleBanUser,
    deleteUser,
    exportUsers,
    downloadImportTemplate,
    showImportModal,
    importUsers
  };
})();

// Auto-initialize when tab is loaded
// This watches for when the RANDOM_ACCESS becomes available
(function () {
  let initAttempts = 0;
  const maxAttempts = 50; // Try for 5 seconds

  const checkAndInit = setInterval(() => {
    initAttempts++;

    if (window.RANDOM_ACCESS) {
      // Check which tab is currently active and initialize accordingly
      const whatsappTab = document.getElementById('tab-whatsapp');
      const userManagementTab = document.getElementById('tab-whatsapp-user-management');

      if (whatsappTab && whatsappTab.children.length > 0) {
        WhatsAppBotManager.init();
      }

      if (userManagementTab && userManagementTab.children.length > 0) {
        WhatsAppUserManager.init();
      }

      clearInterval(checkAndInit);
    } else if (initAttempts >= maxAttempts) {
      console.warn('RANDOM_ACCESS not available after 5 seconds, modules will initialize on tab load');
      clearInterval(checkAndInit);
    }
  }, 100);
})();

// Expose modules to window for global access
window.WhatsAppBotManager = WhatsAppBotManager;
window.WhatsAppUserManager = WhatsAppUserManager;
