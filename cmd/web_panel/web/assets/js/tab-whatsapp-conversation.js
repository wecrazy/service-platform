// Generate QR code on canvas
function generateQRCode(qrText, canvasId) {
  if (!qrText) return;

  const canvas = document.getElementById(canvasId);
  if (!canvas) {
    console.error("Canvas element not found:", canvasId);
    return;
  }

  canvas.width = 260;
  canvas.height = 260;

  if (typeof QRCode !== "undefined") {
    QRCode.toCanvas(
      canvas,
      qrText,
      {
        width: 260,
        height: 260,
        color: { dark: "#000000", light: "#ffffff" },
        margin: 2,
      },
      function (error) {
        if (error) {
          console.error("QR code generation error:", error);
          const ctx = canvas.getContext("2d");
          ctx.clearRect(0, 0, canvas.width, canvas.height);
          ctx.font = "12px Arial";
          ctx.fillText("Error generating QR code", 10, 130);
        }
      }
    );
  } else {
    const ctx = canvas.getContext("2d");
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.font = "12px Arial";
    ctx.fillText("QRCode library not loaded", 10, 50);
    ctx.fillText("Text: " + qrText.substring(0, 30) + "...", 10, 80);
  }
}

// API Functions
async function checkUserWhatsAppStatus(userId, endpoint) {
  try {
    const response = await fetch(endpoint);
    const data = await response.json();
    return data.logged_in;
  } catch (error) {
    console.error("Error checking WhatsApp status:", error);
    return false;
  }
}

async function connectUserWhatsApp(userId, endpoint) {
  const response = await fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });
  return await response.json();
}

async function getUserWhatsAppQR(userId, endpoint) {
  const response = await fetch(endpoint);
  const data = await response.json();
  return data.qr_code;
}

async function getDetailedUserWhatsAppStatus(userId, endpoint) {
  const response = await fetch(endpoint);
  return await response.json();
}

async function refreshUserWhatsAppQR(userId, endpoint) {
  const response = await fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });
  const data = await response.json();
  return data.qr_code;
}

async function disconnectUserWhatsApp(userId, endpoint) {
  const response = await fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });
  return await response.json();
}

// Update connection info display
function updateConnectionInfo(detailedStatus) {
  const statusInfo = $("#connection-info");

  if (!detailedStatus) {
    statusInfo.hide();
    return;
  }

  const statusConfig = {
    disconnected: {
      icon: '<i class="fas fa-circle text-secondary" style="font-size: 8px;"></i>',
      message: "Disconnected",
      class: "alert-secondary",
    },
    connecting: {
      icon: '<i class="fas fa-circle text-warning" style="font-size: 8px;"></i>',
      message: "Connecting...",
      class: "alert-warning",
    },
    connected: {
      icon: '<i class="fas fa-circle text-primary" style="font-size: 8px;"></i>',
      message: "Connected to WhatsApp",
      class: "alert-primary",
    },
    qr_generated: {
      icon: '<i class="fas fa-circle text-info" style="font-size: 8px;"></i>',
      message: "QR Code Ready",
      class: "alert-info",
    },
    authenticating: {
      icon: '<i class="fas fa-spinner fa-spin text-warning" style="font-size: 8px;"></i>',
      message: "Authenticating...",
      class: "alert-warning",
    },
    authenticated: {
      icon: '<i class="fas fa-circle text-success" style="font-size: 8px;"></i>',
      message: "Authenticated & Ready",
      class: "alert-success",
    },
    error: {
      icon: '<i class="fas fa-circle text-danger" style="font-size: 8px;"></i>',
      message: "Connection Error",
      class: "alert-danger",
    },
  };

  const config =
    statusConfig[detailedStatus.state] || statusConfig["disconnected"];

  $("#status-indicator").html(config.icon);
  $("#status-message").text(config.message);
  statusInfo
    .find(".alert")
    .removeClass(
      "alert-info alert-success alert-warning alert-danger alert-primary alert-secondary"
    )
    .addClass(config.class);

  if (detailedStatus.is_authenticated && detailedStatus.phone_number) {
    $("#phone-number-display").html(
      '<i class="fas fa-phone me-1"></i>Phone: +' + detailedStatus.phone_number
    );
  } else {
    $("#phone-number-display").html("");
  }

  if (detailedStatus.last_seen) {
    $("#connection-time").html(
      '<i class="fas fa-clock me-1"></i>Last seen: ' +
        new Date(detailedStatus.last_seen).toLocaleString()
    );
  } else {
    $("#connection-time").html("");
  }

  statusInfo.toggle(detailedStatus.state !== "disconnected");
}

// Update connection details display
function updateConnectionDetails(statusData) {
  const detailsContainer = document.getElementById("connection-details");
  if (!detailsContainer) return;

  if (!statusData?.is_authenticated) {
    detailsContainer.innerHTML = `
      <div class="text-center text-warning">
        <i class="fas fa-exclamation-triangle" style="font-size: 24px;"></i>
        <h6 class="mt-2">Connection Status Unclear</h6>
        <button class="btn btn-sm btn-outline-primary" onclick="location.reload()">
          <i class="fas fa-refresh me-1"></i>Reload Page
        </button>
      </div>`;
    return;
  }

  const phoneNumber = statusData.phone_number || "Not available";
  const state =
    (statusData.state || "unknown").charAt(0).toUpperCase() +
    (statusData.state || "unknown").slice(1).replace("_", " ");
  const connectedSince = statusData.last_seen
    ? new Date(statusData.last_seen).toLocaleString()
    : "Unknown";

  detailsContainer.innerHTML = `
    <div class="row g-3">
      <div class="col-md-6">
        <div class="d-flex align-items-center">
          <div class="flex-shrink-0">
            <div class="bg-success bg-opacity-10 rounded-circle p-2">
              <i class="fas fa-phone text-success"></i>
            </div>
          </div>
          <div class="flex-grow-1 ms-3">
            <h6 class="mb-0">Phone Number</h6>
            <small class="text-muted">${phoneNumber}</small>
          </div>
        </div>
      </div>
      <div class="col-md-6">
        <div class="d-flex align-items-center">
          <div class="flex-shrink-0">
            <div class="bg-info bg-opacity-10 rounded-circle p-2">
              <i class="fas fa-signal text-info"></i>
            </div>
          </div>
          <div class="flex-grow-1 ms-3">
            <h6 class="mb-0">Connection State</h6>
            <small class="text-muted">${state}</small>
          </div>
        </div>
      </div>
      <div class="col-md-6">
        <div class="d-flex align-items-center">
          <div class="flex-shrink-0">
            <div class="bg-primary bg-opacity-10 rounded-circle p-2">
              <i class="fas fa-clock text-primary"></i>
            </div>
          </div>
          <div class="flex-grow-1 ms-3">
            <h6 class="mb-0">Last Active</h6>
            <small class="text-muted">${connectedSince}</small>
          </div>
        </div>
      </div>
      <div class="col-md-6">
        <div class="d-flex align-items-center">
          <div class="flex-shrink-0">
            <div class="bg-success bg-opacity-10 rounded-circle p-2">
              <i class="fas fa-check-circle text-success"></i>
            </div>
          </div>
          <div class="flex-grow-1 ms-3">
            <h6 class="mb-0">Status</h6>
            <small class="text-success">
              <i class="fas fa-circle me-1" style="font-size: 8px;"></i>
              Connected & Authenticated
            </small>
          </div>
        </div>
      </div>
    </div>
    <div class="mt-3 p-3 bg-light rounded">
      <div class="row text-center">
        <div class="col-4">
          <div class="text-success">
            <i class="fas fa-wifi" style="font-size: 20px;"></i>
            <div class="small mt-1">Server Connected</div>
          </div>
        </div>
        <div class="col-4">
          <div class="text-success">
            <i class="fas fa-mobile-alt" style="font-size: 20px;"></i>
            <div class="small mt-1">Phone Paired</div>
          </div>
        </div>
        <div class="col-4">
          <div class="text-success">
            <i class="fas fa-comments" style="font-size: 20px;"></i>
            <div class="small mt-1">Ready to Send</div>
          </div>
        </div>
      </div>
    </div>`;
}

// Show WhatsApp chat and messages (simplified)
async function showWhatsappChatAndMessages(
  statusCheckInterval,
  detailedStatus,
  userId,
  endpoint
) {
  clearInterval(statusCheckInterval);
  $("#whatsapp-login-container").hide();

  let connectedContainer = $("#whatsapp-connected-container");
  if (connectedContainer.length === 0) {
    connectedContainer = await generateDisplayWhatsapp(userId, endpoint);
    $("#whatsapp-main-container").append(connectedContainer);
  } else {
    connectedContainer.show();
  }

  updateConnectionDetails(detailedStatus);
}

async function generateDisplayWhatsapp(userId, endpoint) {
  // First, create the basic structure
  const whatsappInterface = $(`
    <div class="app-chat overflow-hidden card" id="whatsapp-connected-container">
      <div class="row g-0">
        <!-- Sidebar Left -->
        <div class="col app-chat-sidebar-left app-sidebar overflow-hidden" id="app-chat-sidebar-left">
          <div class="d-flex justify-content-center align-items-center h-100">
            <div class="spinner-border text-primary" role="status">
              <span class="visually-hidden">Loading sidebar...</span>
            </div>
          </div>
        </div>
        <!-- /Sidebar Left-->

        <!-- Chat & Contacts -->
        <div class="col app-chat-contacts app-sidebar" id="app-chat-contacts">
          <div class="sidebar-header">
            <div class="d-flex align-items-center me-3 me-lg-0">
              <div class="flex-shrink-0 avatar avatar-online me-3" data-bs-toggle="sidebar" data-overlay
                data-target="#app-chat-sidebar-left">
                <img class="user-avatar rounded-circle cursor-pointer" id="current-user-avatar" src="/assets/img/avatars/1.png" alt="Avatar" />
              </div>
              <div class="flex-grow-1 input-group input-group-merge rounded-pill">
                <input type="text" class="form-control chat-search-input" placeholder="Search..." aria-label="Search..."
                  aria-describedby="basic-addon-search31" />
              </div>
              <button class="btn btn-sm btn-success ms-2 me-2" id="refresh-contact-list-btn" title="Refresh contact list">
                <i class="fal fa-sync"></i>
              </button>
            </div>
            <i class="bx bx-x cursor-pointer position-absolute d-block d-lg-none end-0 top-0 mt-2 me-1 fs-4"
              data-overlay data-bs-toggle="sidebar" data-target="#app-chat-contacts"></i>
          </div>
          <hr class="container-m-nx m-0" />
          <div class="sidebar-body">
            <!-- Chats -->
            <ul class="list-unstyled chat-contact-list pt-1" id="chat-list">
              <li class="chat-contact-list-item chat-list-item-0 d-none">
                <h6 class="text-muted mb-0">Chats</h6>
              </li>
              <!-- Chat contacts will be loaded here -->
              <li class="chat-contact-list-item d-flex align-items-center">
                <div class="d-flex justify-content-center align-items-center w-100">
                  <div class="spinner-border spinner-border-sm text-primary me-2" role="status">
                    <span class="visually-hidden">Loading...</span>
                  </div>
                  <span class="text-muted">Loading conversations...</span>
                </div>
              </li>
            </ul>
            <!-- Contacts -->
            <ul class="list-unstyled chat-contact-list mb-0" id="contact-list">
              <li class="chat-contact-list-item chat-contact-list-item-title mt-2">
                <h6 class="text-muted mb-0">Contacts</h6>
              </li>
              <!-- Contacts will be loaded here -->
            </ul>
          </div>
        </div>
        <!-- /Chat contacts -->

        <!-- Chat History -->
        <div class="col app-chat-history bg-body" id="app-chat-history">
          <div class="chat-history-wrapper">
            <div class="chat-history-header border-bottom">
              <div class="d-flex justify-content-between align-items-center">
                <div class="d-flex overflow-hidden align-items-center">
                  <i class="bx bx-menu bx-sm cursor-pointer d-lg-none me-2" data-bs-toggle="sidebar" data-overlay
                    data-target="#app-chat-contacts"></i>
                  <div class="flex-shrink-0 avatar">
                    <img src="/assets/img/avatars/default-avatar.jpeg" alt="Avatar" class="rounded-circle"
                      data-bs-toggle="sidebar" data-overlay data-target="#app-chat-sidebar-right" />
                  </div>
                  <div class="chat-contact-info flex-grow-1 ms-3">
                    <h6 class="m-0">Select a conversation</h6>
                    <small class="user-status text-muted">Choose a contact to start messaging</small>
                  </div>
                </div>
                <div class="d-flex align-items-center">
                  <i class="bx bx-phone-call cursor-pointer d-sm-block d-none me-3"></i>
                  <i class="bx bx-video cursor-pointer d-sm-block d-none me-3"></i>
                  <i class="bx bx-search cursor-pointer d-sm-block d-none me-3"></i>
                  <div class="dropdown">
                    <i class="bx bx-dots-vertical-rounded cursor-pointer" id="chat-header-actions" data-bs-toggle="dropdown"
                      aria-haspopup="true" aria-expanded="false">
                    </i>
                    <div class="dropdown-menu dropdown-menu-end" aria-labelledby="chat-header-actions">
                      <a class="dropdown-item" href="javascript:void(0);">View Contact</a>
                      <a class="dropdown-item" href="javascript:void(0);">Mute Notifications</a>
                      <a class="dropdown-item" href="javascript:void(0);">Block Contact</a>
                      <a class="dropdown-item" href="javascript:void(0);">Clear Chat</a>
                      <a class="dropdown-item" href="javascript:void(0);">Report</a>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <div class="chat-history-body bg-body">
              <div class="d-flex justify-content-center align-items-center h-100">
                <div class="text-center">
                  <i class="fad fa-comment-alt-dots text-muted mb-3 fs-1"></i>
                  <h5 class="text-muted">Start a conversation</h5>
                  <p class="text-muted">Select a contact from the list to begin messaging</p>
                </div>
              </div>
            </div>
            <div class="chat-history-footer shadow-sm">
              <form class="form-send-message d-flex justify-content-between align-items-center">
                <input class="form-control message-input border-0 me-3 shadow-none" placeholder="Type your message here..." />
                <div class="message-actions d-flex align-items-center">
                  <i class="bx bx-microphone bx-sm cursor-pointer me-3"></i>
                  <i class="bx bx-paperclip bx-sm cursor-pointer me-3"></i>
                  <button class="btn btn-primary d-flex send-msg-btn">
                    <i class="fal fa-paper-plane me-md-1 me-0 text-white"></i>
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
        <!-- /Chat History -->

        <!-- Sidebar Right -->
        <div class="col app-chat-sidebar-right app-sidebar overflow-hidden" id="app-chat-sidebar-right">
          <div class="sidebar-header d-flex flex-column justify-content-center align-items-center flex-wrap p-4 mt-2">
            <div class="avatar avatar-xl avatar-online">
              <img src="/assets/img/avatars/default-avatar.jpeg" alt="Avatar" class="rounded-circle" />
            </div>
            <h5 class="mt-3 mb-1">Contact Info</h5>
            <small class="text-muted">Select a contact to view details</small>
            <i class="bx bx-x bx-sm cursor-pointer close-sidebar me-1 fs-4" data-bs-toggle="sidebar" data-overlay
              data-target="#app-chat-sidebar-right"></i>
          </div>
          <div class="sidebar-body px-4 pb-4">
            <div class="my-3">
              <span class="text-muted text-uppercase">Contact Details</span>
              <div class="mt-2">
                <p class="text-muted">No contact selected</p>
              </div>
            </div>
          </div>
        </div>
        <!-- /Sidebar Right -->

        <div class="app-overlay"></div>
      </div>
    </div>
  `);

  // Load the sidebar left after the interface is created
  setTimeout(async () => {
    try {
      await generateDisplayWASidebarLeft(userId, endpoint);
      console.log("Sidebar left loaded successfully");
      
      // Load contact list
      await generateDisplayWAContactList(userId, endpoint);
      console.log("Contact list loaded successfully");
      
      // Initialize perfect scrollbar for chat areas
      initializePerfectScrollbar();
      
      // Set up event handlers
      setupWhatsappEventHandlers(userId, endpoint);
      
      // Initialize responsive behavior
      initializeResponsiveBehavior();
      
    } catch (error) {
      console.error("Failed to load WhatsApp components:", error);
    }
  }, 100);

  return whatsappInterface;
}

// Load contact list component
async function generateDisplayWAContactList(userId, endpoint) {
  try {
    const response = await fetch(`${endpoint}contact-list`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
    
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    const html = await response.text();
    const chatList = $("#chat-list");
    const contactList = $("#contact-list");
    
    if (chatList.length > 0) {
      chatList.html(html);
    } else {
      console.warn("Chat list container not found");
    }
    
    console.log("Contact list loaded successfully for user:", userId);
    
  } catch (error) {
    console.error("Error loading contact list:", error);
    
    // Fallback: Display error message
    const chatList = $("#chat-list");
    if (chatList.length > 0) {
      chatList.html(`
        <li class="chat-contact-list-item">
          <div class="d-flex justify-content-center align-items-center p-3">
            <div class="text-center">
              <i class="bx bx-error-circle bx-lg text-danger mb-2"></i>
              <p class="text-muted mb-0">Failed to load contacts</p>
              <button class="btn btn-sm btn-primary mt-2" onclick="location.reload()">
                <i class="bx bx-refresh me-1"></i>Retry
              </button>
            </div>
          </div>
        </li>
      `);
    }
  }
}

// Load chat area component
async function generateDisplayWAChatArea(userId, endpoint, contactId = "") {
  try {
    const url = contactId ? 
      `${endpoint}chat-area?contact_id=${encodeURIComponent(contactId)}` : 
      `${endpoint}chat-area`;
      
    const response = await fetch(url, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
    
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    const html = await response.text();
    const chatHistory = $("#app-chat-history");
    
    if (chatHistory.length > 0) {
      chatHistory.html(html);
      
      // Reinitialize perfect scrollbar for chat messages
      initializeChatScrollbar();
      
      // Scroll to bottom of messages
      scrollToBottom();
    } else {
      console.warn("Chat history container not found");
    }
    
    console.log("Chat area loaded successfully for user:", userId, "contact:", contactId);
    
  } catch (error) {
    console.error("Error loading chat area:", error);
  }
}

// Initialize Perfect Scrollbar
function initializePerfectScrollbar() {
  // Initialize scrollbar for sidebar left
  if (typeof PerfectScrollbar !== 'undefined') {
    const sidebarLeft = document.querySelector('#app-chat-sidebar-left .sidebar-body');
    if (sidebarLeft) {
      new PerfectScrollbar(sidebarLeft, {
        wheelPropagation: false,
        suppressScrollX: true
      });
    }
    
    // Initialize scrollbar for contacts
    const contactsBody = document.querySelector('#app-chat-contacts .sidebar-body');
    if (contactsBody) {
      new PerfectScrollbar(contactsBody, {
        wheelPropagation: false,
        suppressScrollX: true
      });
    }
    
    // Initialize scrollbar for sidebar right
    const sidebarRight = document.querySelector('#app-chat-sidebar-right .sidebar-body');
    if (sidebarRight) {
      new PerfectScrollbar(sidebarRight, {
        wheelPropagation: false,
        suppressScrollX: true
      });
    }
  } else {
    console.warn("PerfectScrollbar not available, using native scrolling");
  }
}

// Initialize chat message scrollbar
function initializeChatScrollbar() {
  if (typeof PerfectScrollbar !== 'undefined') {
    const chatBody = document.querySelector('.chat-history-body');
    if (chatBody) {
      new PerfectScrollbar(chatBody, {
        wheelPropagation: false,
        suppressScrollX: true
      });
    }
  }
}

// Scroll to bottom of chat messages
function scrollToBottom() {
  const chatBody = document.querySelector('.chat-history-body');
  if (chatBody) {
    setTimeout(() => {
      chatBody.scrollTop = chatBody.scrollHeight;
    }, 100);
  }
}

// Set up event handlers for WhatsApp interface
function setupWhatsappEventHandlers(userId, endpoint) {
  // Sidebar toggle handlers
  $(document).on('click', '[data-target="#app-chat-sidebar-left"]', function(e) {
    e.preventDefault();
    e.stopPropagation();
    toggleSidebar('#app-chat-sidebar-left');
  });
  
  $(document).on('click', '[data-target="#app-chat-sidebar-right"]', function(e) {
    e.preventDefault();
    e.stopPropagation();
    toggleSidebar('#app-chat-sidebar-right');
  });
  
  $(document).on('click', '[data-target="#app-chat-contacts"]', function(e) {
    e.preventDefault();
    e.stopPropagation();
    toggleSidebar('#app-chat-contacts');
  });

  // Contact list refresh handler
  $(document).on('click', '#refresh-contact-list-btn', async function(e) {
    e.preventDefault();
    const $btn = $(this);
    const originalHtml = $btn.html();
    
    try {
      // Show loading state
      $btn.prop('disabled', true).html('<i class="fad fa-spinner fa-spin"></i>');
      
      // Reload contact list
      await generateDisplayWAContactList(userId, endpoint);
      
      // Show success feedback
      $btn.html('<i class="fal fa-check"></i>');
      setTimeout(() => {
        $btn.prop('disabled', false).html(originalHtml);
      }, 1000);
      
    } catch (error) {
      console.error('Failed to refresh contact list:', error);
      $btn.html('<i class="bx bx-error text-danger"></i>');
      setTimeout(() => {
        $btn.prop('disabled', false).html(originalHtml);
      }, 2000);
    }
  });

  // Contact click handler
  $(document).on('click', '.chat-contact-list-item[data-contact-id]', function(e) {
    e.preventDefault();
    const contactId = $(this).data('contact-id');
    
    if (contactId) {
      // Remove active class from all contacts
      $('.chat-contact-list-item').removeClass('active');
      // Add active class to clicked contact
      $(this).addClass('active');
      
      // Get contact information from the clicked element
      const contactName = $(this).find('.chat-contact-name').text() || 'Unknown Contact';
      const contactStatus = $(this).find('.chat-contact-status').text() || 'No status';
      const contactAvatar = $(this).find('img').attr('src') || '/assets/img/avatars/default-avatar.jpeg';
      const isGroup = $(this).find('.avatar-group').length > 0;
      
      // Update right sidebar with contact info
      updateContactRightSidebar(contactId, contactName, contactStatus, contactAvatar, isGroup);
      
      // Load chat area for selected contact
      generateDisplayWAChatArea(userId, endpoint, contactId);
    }
  });
  
  // Message send handler
  $(document).on('submit', '.form-send-message', function(e) {
    e.preventDefault();
    const form = $(this);
    const messageInput = form.find('.message-input');
    const message = messageInput.val().trim();
    const contactId = form.data('contact-id');
    
    if (message && contactId) {
      sendWhatsappMessage(userId, endpoint, contactId, message);
      messageInput.val(''); // Clear input
    }
  });
  
  // Search handler
  $(document).on('input', '.chat-search-input', function() {
    const searchQuery = $(this).val().toLowerCase();
    searchContacts(searchQuery);
  });
  
  // File upload handler
  $(document).on('change', '#file-upload', function() {
    const file = this.files[0];
    if (file) {
      handleFileUpload(file, userId, endpoint);
    }
  });
}

// Search contacts function
function searchContacts(query) {
  $('.chat-contact-list-item[data-contact-id]').each(function() {
    const contactName = $(this).find('.chat-contact-name').text().toLowerCase();
    const contactPhone = $(this).find('.chat-contact-status').text().toLowerCase();
    
    if (contactName.includes(query) || contactPhone.includes(query)) {
      $(this).show();
    } else {
      $(this).hide();
    }
  });
}

// Send WhatsApp message
async function sendWhatsappMessage(userId, endpoint, contactId, message) {
  try {
    const response = await fetch(`${endpoint}send`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        recipient: contactId,
        message: message,
        is_group: contactId.includes('@g.us')
      })
    });
    
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    const result = await response.json();
    console.log("Message sent successfully:", result);
    
    // Add message to chat (optimistic UI update)
    addMessageToChat(message, true);
    scrollToBottom();
    
    // Show success feedback
    showMessageStatus('Message sent successfully', 'success');
    
  } catch (error) {
    console.error("Error sending message:", error);
    showMessageStatus('Failed to send message', 'error');
  }
}

// Add message to chat UI
function addMessageToChat(message, isOutgoing = false) {
  const timestamp = new Date().toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
  const messageHTML = `
    <li class="chat-message ${isOutgoing ? 'chat-message-right' : ''}">
      <div class="d-flex overflow-hidden">
        ${!isOutgoing ? `
        <div class="user-avatar flex-shrink-0 me-3">
          <div class="avatar avatar-sm">
            <img src="/assets/img/avatars/default-avatar.jpeg" alt="Contact" class="rounded-circle" />
          </div>
        </div>` : ''}
        <div class="chat-message-wrapper flex-grow-1">
          <div class="chat-message-text">
            <p class="mb-0">${message}</p>
          </div>
          <div class="text-muted mt-1">
            <small>
              ${timestamp}
              ${isOutgoing ? '<i class="bx bx-check text-muted ms-1"></i>' : ''}
            </small>
          </div>
        </div>
      </div>
    </li>
  `;
  
  const chatMessages = $('#chat-messages');
  if (chatMessages.length > 0) {
    chatMessages.append(messageHTML);
  }
}

// Show message status
function showMessageStatus(message, type) {
  const alertClass = type === 'success' ? 'alert-success' : 'alert-danger';
  const icon = type === 'success' ? 'bx-check-circle' : 'bx-error-circle';
  
  const statusHTML = `
    <div class="alert ${alertClass} alert-dismissible fade show position-fixed" 
         style="top: 20px; right: 20px; z-index: 9999; min-width: 300px;" role="alert">
      <i class="bx ${icon} me-2"></i>
      ${message}
      <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
    </div>
  `;
  
  $('body').append(statusHTML);
  
  // Auto-dismiss after 3 seconds
  setTimeout(() => {
    $('.alert').fadeOut();
  }, 3000);
}

// Handle file upload
function handleFileUpload(file, userId, endpoint) {
  console.log("File upload not implemented yet:", file.name);
  showMessageStatus('File upload feature coming soon', 'info');
}

async function generateDisplayWASidebarLeft(userId, endpoint) {
  try {
    const response = await fetch(`${endpoint}sidebar-left`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
    
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    const html = await response.text();
    const sidebarLeft = $("#app-chat-sidebar-left");
    
    if (sidebarLeft.length === 0) {
      console.error("Sidebar left container not found");
      return;
    }
    
    // Insert the HTML content
    sidebarLeft.html(html);
    
    // Extract user avatar from sidebar and update contact list header
    setTimeout(() => {
      const sidebarAvatar = sidebarLeft.find('.avatar img').attr('src');
      if (sidebarAvatar) {
        $('#current-user-avatar').attr('src', sidebarAvatar);
      }
    }, 100);
    
    console.log("Sidebar left loaded successfully for user:", userId);
    
  } catch (error) {
    console.error("Error loading sidebar left:", error);
    
    // Fallback: Display a basic sidebar with error message
    const sidebarLeft = $("#app-chat-sidebar-left");
    if (sidebarLeft.length > 0) {
      sidebarLeft.html(`
        <div class="chat-sidebar-left-user sidebar-header d-flex flex-column justify-content-center align-items-center flex-wrap p-4 mt-2">
          <div class="avatar avatar-xl avatar-offline">
            <img src="/assets/img/avatars/default-avatar.jpeg" alt="Avatar" class="rounded-circle" />
          </div>
          <h5 class="mt-3 mb-1">Loading...</h5>
          <small class="text-danger">Failed to load user data</small>
          <i class="bx bx-x bx-sm cursor-pointer close-sidebar me-1 fs-4" data-bs-toggle="sidebar" data-overlay
            data-target="#app-chat-sidebar-left"></i>
        </div>
        <div class="sidebar-body px-4 pb-4">
          <div class="alert alert-danger">
            <i class="bx bx-error-circle me-2"></i>
            Unable to load sidebar. Please try refreshing.
          </div>
          <button class="btn btn-primary w-100" onclick="location.reload()">
            <i class="bx bx-refresh me-1"></i>Refresh Page
          </button>
        </div>
      `);
    }
  }
}

// Update right sidebar with contact information
function updateContactRightSidebar(contactId, contactName, contactStatus, contactAvatar, isGroup) {
  const rightSidebar = $("#app-chat-sidebar-right");
  
  if (rightSidebar.length === 0) {
    console.warn("Right sidebar not found");
    return;
  }
  
  // Extract phone number from JID if it's not a group
  let phoneNumber = '';
  if (!isGroup && contactId) {
    const phonePart = contactId.split('@')[0];
    if (phonePart && phonePart !== contactId) {
      // Remove device suffix if present (e.g., :21)
      const cleanPhone = phonePart.split(':')[0];
      phoneNumber = '+' + cleanPhone;
    }
  }
  
  // Generate contact info HTML
  const contactInfoHtml = `
    <div class="sidebar-header d-flex flex-column justify-content-center align-items-center flex-wrap p-4 mt-2">
      <div class="avatar avatar-xl">
        <img src="${contactAvatar}" alt="Avatar" class="rounded-circle" />
      </div>
      <h5 class="mt-3 mb-1">${contactName}</h5>
      <small class="text-muted">${isGroup ? 'Group Chat' : (phoneNumber || 'Contact')}</small>
      <i class="bx bx-x bx-sm cursor-pointer close-sidebar me-1 fs-4" data-bs-toggle="sidebar" data-overlay
        data-target="#app-chat-sidebar-right"></i>
    </div>
    <div class="sidebar-body px-4 pb-4">
      <div class="my-3">
        <span class="text-muted text-uppercase">Contact Details</span>
        <div class="mt-2">
          <div class="d-flex align-items-center mb-2">
            <i class="bx bx-user me-2 text-muted"></i>
            <span>${contactName}</span>
          </div>
          ${!isGroup && phoneNumber ? `
          <div class="d-flex align-items-center mb-2">
            <i class="bx bx-phone me-2 text-muted"></i>
            <span>${phoneNumber}</span>
          </div>
          ` : ''}
          ${isGroup ? `
          <div class="d-flex align-items-center mb-2">
            <i class="bx bx-group me-2 text-muted"></i>
            <span>Group Conversation</span>
          </div>
          ` : ''}
          <div class="d-flex align-items-center mb-2">
            <i class="bx bx-message me-2 text-muted"></i>
            <span class="text-truncate">${contactStatus}</span>
          </div>
        </div>
      </div>
      
      <div class="my-3">
        <span class="text-muted text-uppercase">Actions</span>
        <div class="d-grid gap-2 mt-2">
          ${!isGroup ? `
          <button class="btn btn-outline-primary btn-sm">
            <i class="bx bx-phone me-1"></i>Voice Call
          </button>
          <button class="btn btn-outline-primary btn-sm">
            <i class="bx bx-video me-1"></i>Video Call
          </button>
          ` : ''}
          <button class="btn btn-outline-secondary btn-sm">
            <i class="bx bx-search me-1"></i>Search Messages
          </button>
          <button class="btn btn-outline-secondary btn-sm">
            <i class="bx bx-volume-mute me-1"></i>Mute Notifications
          </button>
          ${isGroup ? `
          <button class="btn btn-outline-info btn-sm">
            <i class="bx bx-group me-1"></i>Group Info
          </button>
          ` : `
          <button class="btn btn-outline-info btn-sm">
            <i class="bx bx-user me-1"></i>Contact Info
          </button>
          `}
          <button class="btn btn-outline-danger btn-sm">
            <i class="bx bx-block me-1"></i>Block Contact
          </button>
        </div>
      </div>
      
      <div class="my-3">
        <span class="text-muted text-uppercase">Media & Files</span>
        <div class="mt-2">
          <div class="row g-2">
            <div class="col-4">
              <div class="card bg-light text-center p-2">
                <i class="bx bx-image bx-sm text-primary"></i>
                <small class="text-muted d-block">Media</small>
              </div>
            </div>
            <div class="col-4">
              <div class="card bg-light text-center p-2">
                <i class="bx bx-file bx-sm text-info"></i>
                <small class="text-muted d-block">Files</small>
              </div>
            </div>
            <div class="col-4">
              <div class="card bg-light text-center p-2">
                <i class="bx bx-link bx-sm text-warning"></i>
                <small class="text-muted d-block">Links</small>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `;
  
  rightSidebar.html(contactInfoHtml);
  console.log("Right sidebar updated for contact:", contactName);
}

// Show image modal for viewing large images
function showImageModal(imageUrl, filename) {
  const modal = $(`
    <div class="modal fade" id="imageModal" tabindex="-1" aria-hidden="true">
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">${filename || 'Image'}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
          </div>
          <div class="modal-body text-center">
            <img src="${imageUrl}" class="img-fluid" alt="${filename || 'Image'}" style="max-height: 70vh;" />
          </div>
          <div class="modal-footer">
            <a href="${imageUrl}" download="${filename}" class="btn btn-primary">
              <i class="bx bx-download me-1"></i>Download
            </a>
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Close</button>
          </div>
        </div>
      </div>
    </div>
  `);
  
  // Remove existing modal if any
  $('#imageModal').remove();
  
  // Add and show modal
  $('body').append(modal);
  $('#imageModal').modal('show');
  
  // Clean up after modal is hidden
  $('#imageModal').on('hidden.bs.modal', function () {
    $(this).remove();
  });
}

// Format file size for display
function formatFileSize(bytes) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Toggle sidebar visibility
function toggleSidebar(sidebarSelector) {
  const sidebar = $(sidebarSelector);
  const overlay = $('.app-overlay');
  
  if (!sidebar.length) {
    console.warn("Sidebar not found:", sidebarSelector);
    return;
  }
  
  const isVisible = sidebar.is(':visible') && sidebar.hasClass('show');
  
  if (isVisible) {
    // Hide sidebar
    sidebar.removeClass('show');
    overlay.removeClass('show');
    $('body').removeClass('app-sidebar-open');
    
    // On mobile, also hide via display
    if (window.innerWidth <= 991) {
      sidebar.hide();
    }
  } else {
    // Hide all other sidebars first
    $('.app-sidebar').removeClass('show');
    if (window.innerWidth <= 991) {
      $('.app-sidebar').hide();
    }
    
    // Show target sidebar
    sidebar.show().addClass('show');
    overlay.addClass('show');
    $('body').addClass('app-sidebar-open');
  }
  
  console.log("Toggled sidebar:", sidebarSelector, "Now visible:", !isVisible);
}

// Close all sidebars
function closeSidebars() {
  $('.app-sidebar').removeClass('show');
  $('.app-overlay').removeClass('show');
  $('body').removeClass('app-sidebar-open');
  
  // On mobile, hide sidebars except contacts which should stay visible
  if (window.innerWidth <= 991) {
    $('#app-chat-sidebar-left, #app-chat-sidebar-right').hide();
  }
}

// Handle overlay click to close sidebars
$(document).on('click', '.app-overlay', function() {
  closeSidebars();
});

// Handle window resize to manage sidebar visibility
$(window).on('resize', function() {
  if (window.innerWidth > 991) {
    // On desktop, show left sidebar by default, hide overlay
    $('#app-chat-sidebar-left').show();
    $('.app-overlay').removeClass('show');
    $('body').removeClass('app-sidebar-open');
  } else {
    // On mobile, hide sidebars except if they have show class
    $('#app-chat-sidebar-left, #app-chat-sidebar-right').each(function() {
      if (!$(this).hasClass('show')) {
        $(this).hide();
      }
    });
  }
});

// Initialize responsive behavior
function initializeResponsiveBehavior() {
  // Set initial state based on screen size
  if (window.innerWidth > 991) {
    // Desktop: show left sidebar by default
    $('#app-chat-sidebar-left').show();
    $('#app-chat-sidebar-right').hide();
  } else {
    // Mobile: hide both sidebars initially
    $('#app-chat-sidebar-left, #app-chat-sidebar-right').hide();
  }
  
  // Remove any lingering overlay
  $('.app-overlay').removeClass('show');
  $('body').removeClass('app-sidebar-open');
  
  console.log("Responsive behavior initialized");
}
