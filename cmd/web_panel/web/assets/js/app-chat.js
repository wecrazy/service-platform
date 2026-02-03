/**
 * App Chat (jQuery version)
 */

'use strict';

$(function () {
  const $chatContactsBody = $('.app-chat-contacts .sidebar-body'),
    $chatContactListItems = $('.chat-contact-list-item:not(.chat-contact-list-item-title)'),
    $chatHistoryBody = $('.chat-history-body'),
    $chatSidebarLeftBody = $('.app-chat-sidebar-left .sidebar-body'),
    $chatSidebarRightBody = $('.app-chat-sidebar-right .sidebar-body'),
    $chatUserStatus = $(".form-check-input[name='chat-user-status']"),
    $chatSidebarLeftUserAbout = $('.chat-sidebar-left-user-about'),
    $formSendMessage = $('.form-send-message'),
    $messageInput = $('.message-input'),
    $searchInput = $('.chat-search-input'),
    $speechToText = $('.speech-to-text'), // ! jQuery dependency for speech to text
    userStatusObj = {
      active: 'avatar-online',
      offline: 'avatar-offline',
      away: 'avatar-away',
      busy: 'avatar-busy'
    };

  // Initialize PerfectScrollbar
  if ($chatContactsBody.length) {
    new PerfectScrollbar($chatContactsBody[0], {
      wheelPropagation: false,
      suppressScrollX: true
    });
  }
  if ($chatHistoryBody.length) {
    new PerfectScrollbar($chatHistoryBody[0], {
      wheelPropagation: false,
      suppressScrollX: true
    });
  }
  if ($chatSidebarLeftBody.length) {
    new PerfectScrollbar($chatSidebarLeftBody[0], {
      wheelPropagation: false,
      suppressScrollX: true
    });
  }
  if ($chatSidebarRightBody.length) {
    new PerfectScrollbar($chatSidebarRightBody[0], {
      wheelPropagation: false,
      suppressScrollX: true
    });
  }

  // Scroll to bottom function
  function scrollToBottom() {
    if ($chatHistoryBody.length && $chatHistoryBody[0]) {
      $chatHistoryBody.scrollTop($chatHistoryBody[0].scrollHeight);
    }
  }
  scrollToBottom();

  // User About Maxlength Init
  if ($chatSidebarLeftUserAbout.length) {
    $chatSidebarLeftUserAbout.maxlength({
      alwaysShow: true,
      warningClass: 'label label-success bg-success text-white',
      limitReachedClass: 'label label-danger',
      separator: '/',
      validate: true,
      threshold: 120
    });
  }

  // Update user status
  $chatUserStatus.on('click', function () {
    let value = $(this).val();
    let $chatLeftSidebarUserAvatar = $('.chat-sidebar-left-user .avatar');
    $chatLeftSidebarUserAvatar.attr('class', 'avatar avatar-xl ' + userStatusObj[value]);
    let $chatContactsUserAvatar = $('.app-chat-contacts .avatar');
    $chatContactsUserAvatar.attr('class', 'flex-shrink-0 avatar ' + userStatusObj[value] + ' me-3');
  });

  // Select chat or contact
  $chatContactListItems.on('click', function () {
    $chatContactListItems.removeClass('active');
    $(this).addClass('active');
  });

  // Filter Chats
  if ($searchInput.length) {
    $searchInput.on('keyup', function () {
      let searchValue = $(this).val().toLowerCase(),
        searchChatListItemsCount = 0,
        searchContactListItemsCount = 0,
        $chatListItem0 = $('.chat-list-item-0'),
        $contactListItem0 = $('.contact-list-item-0'),
        $searchChatListItems = $('#chat-list li:not(.chat-contact-list-item-title)'),
        $searchContactListItems = $('#contact-list li:not(.chat-contact-list-item-title)');

      // Search in chats
      searchChatContacts($searchChatListItems, searchValue, $chatListItem0);
      // Search in contacts
      searchChatContacts($searchContactListItems, searchValue, $contactListItem0);
    });
  }

  // Search chat and contacts function
  function searchChatContacts($searchListItems, searchValue, $listItem0) {
    let searchListItemsCount = 0;
    $searchListItems.each(function () {
      let searchListItemText = $(this).text().toLowerCase();
      if (searchValue) {
        if (searchListItemText.indexOf(searchValue) > -1) {
          $(this).addClass('d-flex').removeClass('d-none');
          searchListItemsCount++;
        } else {
          $(this).addClass('d-none').removeClass('d-flex');
        }
      } else {
        $(this).addClass('d-flex').removeClass('d-none');
        searchListItemsCount++;
      }
    });
    // Display no search found if searchListItemsCount == 0
    if (searchListItemsCount == 0) {
      $listItem0.removeClass('d-none');
    } else {
      $listItem0.addClass('d-none');
    }
  }

  // Send Message
  $formSendMessage.on('submit', function (e) {
    e.preventDefault();
    if ($messageInput.val()) {
      let renderMsg = $('<div class="chat-message-text mt-2"><p class="mb-0 text-break">' + $messageInput.val() + '</p></div>');
      $('li:last-child .chat-message-wrapper').append(renderMsg);
      $messageInput.val('');
      scrollToBottom();
    }
  });

  // Remove data-overlay attribute from chatSidebarLeftClose to resolve overlay overlapping issue for two sidebar
  let $chatHistoryHeaderMenu = $(".chat-history-header [data-target='#app-chat-contacts']"),
    $chatSidebarLeftClose = $('.app-chat-sidebar-left .close-sidebar');
  $chatHistoryHeaderMenu.on('click', function () {
    $chatSidebarLeftClose.removeAttr('data-overlay');
  });

  // Speech To Text
  if ($speechToText.length) {
    var SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (SpeechRecognition !== undefined && SpeechRecognition !== null) {
      var recognition = new SpeechRecognition(),
        listening = false;
      $speechToText.on('click', function () {
        const $this = $(this);
        recognition.onspeechstart = function () {
          listening = true;
        };
        if (listening === false) {
          recognition.start();
        }
        recognition.onerror = function (event) {
          listening = false;
        };
        recognition.onresult = function (event) {
          $this.closest('.form-send-message').find('.message-input').val(event.results[0][0].transcript);
        };
        recognition.onspeechend = function (event) {
          listening = false;
          recognition.stop();
        };
      });
    }
  }
});
