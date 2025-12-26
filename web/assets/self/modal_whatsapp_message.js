/**
 * Formats a date string into a human-readable format.
 * @param {string} dateString - The date string to format (e.g., ISO string).
 * @returns {string} The formatted date string or "N/A" if invalid.
 */
function formatDateTime(dateString) {
  if (!dateString) return "N/A";
  const date = new Date(dateString);
  const days = [
    "Sunday",
    "Monday",
    "Tuesday",
    "Wednesday",
    "Thursday",
    "Friday",
    "Saturday",
  ];
  const months = [
    "Jan",
    "Feb",
    "Mar",
    "Apr",
    "May",
    "Jun",
    "Jul",
    "Aug",
    "Sept",
    "Oct",
    "Nov",
    "Dec",
  ];

  const dayName = days[date.getDay()];
  const day = date.getDate();
  const monthName = months[date.getMonth()];
  const year = date.getFullYear();
  let hours = date.getHours();
  const minutes = ("0" + date.getMinutes()).slice(-2);
  const seconds = ("0" + date.getSeconds()).slice(-2);
  const ampm = hours >= 12 ? "PM" : "AM";
  hours = hours % 12;
  hours = hours ? hours : 12; // the hour '0' should be '12'
  const strHours = ("0" + hours).slice(-2);

  return `${dayName}, ${day} ${monthName} ${year}, ${strHours}:${minutes}:${seconds} ${ampm}`;
}

/**
 * Displays a modal with formatted WhatsApp messages and replies.
 * @param {Array} messagesArray - Array of message objects containing WhatsApp conversation data.
 */
function showWhatsAppMessageModal(messagesArray) {
  // The 'messagesArray' is already a JavaScript object/array. No need to parse.

  let formattedMessages = "";
  if (!messagesArray || messagesArray.length === 0) {
    formattedMessages =
      "<div class='text-center p-3'>No messages to display.</div>";
  } else {
    // Start of the chat container and styles
    formattedMessages = `
      <style>
        .chat-container {
          overflow-y: auto;
          padding: 10px;
          background-color: #f5f5f5;
          border-radius: 8px;
        }
        .message-pair {
          margin-bottom: 15px;
          display: flex;
          flex-direction: column;
        }
        .message {
          padding: 10px 15px;
          border-radius: 18px;
          margin-bottom: 5px;
          max-width: 80%;
          word-break: break-all;
        }
        .message.sent {
          background-color: #dcf8c6;
          align-self: flex-end;
          text-align: left;
        }
        .message.received {
          background-color: #ffffff;
          align-self: flex-start;
          text-align: left;
          border: 1px solid #eee;
        }
        .message-meta {
          font-size: 0.75rem;
          color: #888;
          margin-top: 5px;
        }
         .message-sender {
          font-weight: bold;
          margin-bottom: 5px;
          color: #075e54;
        }
      </style>
      <div class="chat-container">
    `;

    messagesArray.forEach((msg) => {
      const body = formatWhatsappMessageBodyToHTML(
        msg.whatsapp_message_body || ""
      );

      const sentToPhone = msg.whatsapp_message_sent_to || "N/A";
      const sentTo = msg.destination_name
        ? `${msg.destination_name} (${sentToPhone})`
        : sentToPhone;

      const sentAt = formatDateTime(msg.whatsapp_sent_at);
      const reply = formatWhatsappMessageBodyToHTML(
        msg.whatsapp_reply_text || ""
      );

      const repliedByPhone = msg.whatsapp_replied_by || "N/A";
      const repliedByWithPhone = msg.sender_name
        ? `${msg.sender_name} (${repliedByPhone})`
        : repliedByPhone;
      const repliedBy = msg.sender_name ? `${msg.sender_name}` : repliedByPhone;

      const repliedAt = formatDateTime(msg.whatsapp_replied_at);

      formattedMessages += `
        <div class="message-pair">
          <div class="message sent">
            <div class="message-sender">Bot Whatsapp</div>
            <div class="message-content">${body}</div>
            <div class="message-meta">To: <b>${sentTo}</b> | ${sentAt}</div>
          </div>
      `;

      if (msg.whatsapp_reply_text) {
        formattedMessages += `
          <div class="message received">
            <div class="message-sender">${repliedBy}</div>
            <div class="message-content">${reply}</div>
            <div class="message-meta">From: <b>${repliedByWithPhone}</b> | ${repliedAt}</div>
          </div>
        `;
      }
      formattedMessages += "</div>";
    });

    formattedMessages += "</div>"; // End of chat-container
  }

  // Ensure the modal div exists in your HTML
  if ($("#whatsapp-message-modal").length === 0) {
    $("body").append(
      '<div id="whatsapp-message-modal" style="display: none;"><div class="modal-content"></div></div>'
    );
  }

  // Use iziModal to show the formatted messages.
  $("#whatsapp-message-modal").iziModal({
    title: "WhatsApp Conversation",
    subtitle: "View the sent messages and replies",
    headerColor: "#1E88E5", // Blue header
    width: 700,
    padding: 20,
    fullscreen: true,
    openFullscreen: true,
    overlayClose: false,
    closeOnEscape: false,
    transitionIn: "fadeInUp",
    transitionOut: "fadeOutDown",
    zindex: 9999,
    onFullscreen: function (modal, isFullscreen) {
      const chatContainer = modal.find(".chat-container");
      if (isFullscreen) {
        chatContainer.css("height", "calc(100vh - 150px)"); // Adjust 150px based on header/footer height
      } else {
        chatContainer.css("height", "auto");
      }
    },
  });

  $("#whatsapp-message-modal .modal-content").html(formattedMessages);
  $("#whatsapp-message-modal").iziModal("open");
}

/**
 * Formats WhatsApp message text into HTML, handling URLs, formatting, and escaping.
 * @param {string} text - The raw message text.
 * @returns {string} The formatted HTML string.
 */
function formatWhatsappMessageBodyToHTML(text) {
  if (!text) return "";

  const urlRegex = /(https?:\/\/[^\s]+)/g;
  const urls = [];
  let placeholderIndex = 0;

  // Step 1: Find all URLs and replace them with a unique, safe placeholder.
  // This placeholder must not contain any characters that are used for formatting, like '_', '*', or '~'.
  let processedText = text.replace(urlRegex, (url) => {
    const placeholder = `URLHOLDER${placeholderIndex}URLHOLDER`; // A completely safe placeholder.
    urls.push({ placeholder, url });
    placeholderIndex++;
    return placeholder;
  });

  // Step 2: Apply standard HTML escaping to the entire text to prevent XSS.
  processedText = processedText.replace(/</g, "&lt;").replace(/>/g, "&gt;");

  // Step 3: Apply WhatsApp-style formatting (bold, italic, etc.).
  processedText = processedText.replace(/\*(.*?)\*/g, "<strong>$1</strong>");
  processedText = processedText.replace(/_(.*?)_/g, "<em>$1</em>");
  processedText = processedText.replace(/~(.*?)~/g, "<del>$1</del>");
  processedText = processedText.replace(/```(.*?)```/g, "<code>$1</code>");
  processedText = processedText.replace(/\n/g, "<br>");

  // Step 4: Finally, replace the placeholders with the real anchor tags.
  // This is done last to ensure the anchor tags themselves are not escaped or formatted.
  urls.forEach((urlInfo) => {
    const anchorTag = `<a href="${urlInfo.url}" target="_blank" class="text-primary">${urlInfo.url}</a>`;
    processedText = processedText.replace(urlInfo.placeholder, anchorTag);
  });

  return processedText;
}
