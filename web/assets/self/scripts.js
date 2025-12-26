/**
 * Constants
 * @constant {Object}
 * DateTime may not be available at script-evaluation time if Luxon hasn't
 * been loaded yet. Use a safe reference and only rely on it when present.
 */
const DateTime = (typeof luxon !== 'undefined' && luxon && luxon.DateTime) ? luxon.DateTime : undefined;

/**
 * Load a tab's HTML (if needed) then show the tab and update menu states.
 *
 * This function handles both already-loaded tab panes and lazy-loading
 * components via fetch().
 *
 * Behaviour summary:
 * - If targetTabId resolves to `#debug` it will immediately show the debug
 *   container and hide the loading indicator.
 * - If the tab DOM element already contains children it will reveal the tab.
 * - Otherwise it validates the tab against existing menu links then fetches
 *   component HTML from `webPath + 'components/' + <target without '#'>`.
 * - On fetch success the HTML is injected into the tab container and menu
 *   classes are updated to make the corresponding anchor active.
 * - On failure it shows the error container and removes the loading indicator.
 *
 * @param {string} targetTabId - The target ID string for the tab (e.g. "#my-tab").
 * @returns {void}
 *
 * @example
 * // show the tab with id "#dashboard"
 * loadAndShowTab('#dashboard');
 */
function loadAndShowTab(targetTabId) {
  // loadAndShowTab called
  if (targetTabId.substring(1) == "debug") {
    $("#loading-container").removeClass("d-block").addClass("d-none");
    $("#loading-container").hide();
    $("body").removeClass("loading_background");
    $("#debug-container").removeClass("d-none").addClass("d-block");
    return;
  }
  if (targetTabId.indexOf("#") >= 0) {
    // console.log(targetTabId);
    if ($(targetTabId).children().length > 0) {
      // showing pre-rendered tab
      // Show pre-rendered content and ensure it gets localized.
      $(targetTabId).removeClass("d-none").addClass("d-block");
      // Run localize() safely in case i18next wasn't ready earlier.
      if (typeof localize === 'function') {
        try {
          // initDashboardTab: localize check
          if (typeof i18next !== 'undefined' && i18next && i18next.isInitialized !== true) {
            try { /* i18next not initialized yet — attach initialized handler */ } catch (e) { }
            try { i18next.on && i18next.on('initialized', localize); } catch (e) { /* ignore attach error */ }
            setTimeout(function () { try { localize(); } catch (e) { } }, 500);
          } else {
            try { localize(); } catch (e) { /* ignore */ }
            // If i18next is initialized but resources may not have been
            // loaded against this DOM piece, force reload of the active
            // language resources and re-run localize to ensure translation.
            try {
              if (typeof i18next !== 'undefined' && i18next && i18next.isInitialized === true) {
                // Triggering changeLanguage with the same language forces resource reload
                try { /* forcing changeLanguage to reload resources for current language */ i18next.changeLanguage && i18next.changeLanguage(i18next.language); } catch (e) { /* ignore */ }
              }
            } catch (e) { /* ignore */ }
          }
        } catch (e) { /* ignore */ }
      }
      try {
        if (typeof window.initDashboardTab === 'function' && targetTabId.indexOf('tab-dashboard') >= 0) window.initDashboardTab();
      } catch (e) { /* ignore */ }
    } else {
      var isTabValid =
        $(".menu-link:not(.menu-toggle)").filter(function () {
          return $(this).attr("href") === targetTabId;
        }).length > 0;
      if (isTabValid) {
        fetch(webPath + "components/" + targetTabId.substring(1))
          .then((response) => {
            if (response.status != 200) {
              // window.location.reload();
            }
            return response.text();
          })
          .then((html) => {
            if (html.includes("</html>")) {
              // window.location.reload();
              return;
            }
            $("#loading-container").hide();
            $("body").removeClass("loading_background");

            try { /* fetched HTML for tab (size logged previously during debug) */ } catch (e) { }
            $(targetTabId).html(html);
            // If a global localize() function exists (i18n), run it so newly-inserted
            // component markup with data-i18n attributes gets localized. Be careful
            // not to call localize() before i18next has finished initialization
            // (it returns keys if namespaces/resources are not loaded yet).
            if (typeof localize === 'function') {
              try {
                // calling localize() for newly-fetched content
                // If i18next exists but is not yet initialized, attach an
                // initialized event and use a short timeout as a fallback.
                if (typeof i18next !== 'undefined' && i18next && i18next.isInitialized !== true) {
                  try { i18next.on && i18next.on('initialized', localize); } catch (e) { /* ignore attach error */ }
                  setTimeout(function () { try { localize(); } catch (e) { } }, 500);
                } else {
                  try { localize(); } catch (e) { /* ignore localization errors */ }
                }
              } catch (e) { /* ignore */ }
            }
            // If the loaded component provides a named initializer, call it.
            // This keeps per-tab script logic centralized in scripts.js.
            try {
              if (typeof window.initDashboardTab === 'function' && targetTabId.indexOf('tab-dashboard') >= 0) {
                window.initDashboardTab();
              }
            } catch (e) { /* ignore init errors */ }
            // $(targetTabId).show();
            $(targetTabId).removeClass("d-none").addClass("d-block");
            if (html == "" || html == null) {
              // $("#error-container").show();
              $("#error-container").removeClass("d-none").addClass("d-block");
            }
            var targetAnchor = $(
              'a[href="' + targetTabId + '"].menu-link:not(.menu-toggle)'
            );
            targetAnchor.addClass("active");
            // Remove 'active' class from all menu items within the same menu-sub
            targetAnchor
              .closest(".menu-inner")
              .find(".menu-item")
              .removeClass("active");

            // Add 'active' class to the clicked menu item
            targetAnchor.closest(".menu-item").addClass("active");

            // Set parent menu-item with 'active' class
            targetAnchor
              .closest(".menu-item")
              .parents(".menu-item")
              .addClass("active");
          })
          .catch((error) => {
            // Error fetching content
            $("#loading-container").removeClass("d-block").addClass("d-none");
            $("body").removeClass("loading_background");
            $("#error-container").removeClass("d-none").addClass("d-block");
          });
      } else {
        $("#loading-container").removeClass("d-block").addClass("d-none");
        $("#error-container").removeClass("d-none").addClass("d-block");
        $("body").removeClass("loading_background");
      }
    }
  } else {
    // Handle the case where targetTabId does not contain #
    // Invalid targetTabId
  }
}

/**
 * Initialize page tab behaviour for routes under `/page`.
 *
 * - If the current URL contains "/page" the script will pick an initial
 *   target tab from the URL hash (if present) or fall back to the first
 *   `.menu-link:not(.menu-toggle)` entry.
 * - It then loads the selected tab and attaches a click handler to
 *   `.menu-link:not(.menu-toggle)` so subsequent clicks load other tabs,
 *   update active menu classes and hide/show error/loading states.
 *
 * Side effects: DOM updates, possible network fetches via loadAndShowTab,
 * and updates to the browser `location.hash` indirectly.
 *
 * @returns {void}
 */
if (window.location.pathname.indexOf("/page") !== -1) {
  var targetTabId = window.location.hash;

  if (targetTabId == null || targetTabId == "") {
    targetTabId = $(".menu-link:not(.menu-toggle):first").attr("href");
  }
  loadAndShowTab(targetTabId);

  // Handle tab clicks
  /**
   * Click handler for menu anchors which loads a tab and updates menu state.
   *
   * @param {Event} e - Click event from the anchor.
   * @returns {void}
   */
  $(".menu-link:not(.menu-toggle)").click(function (e) {
    $(".tab-content").removeClass("d-block").addClass("d-none");
    $("#error-container").removeClass("d-block").addClass("d-none");

    // Get the target tab ID from the href attribute
    var targetTabId = $(this).attr("href");
    loadAndShowTab(targetTabId);

    // Remove 'active' class from all menu items within the same menu-sub
    $(this).closest(".menu-inner").find(".menu-item").removeClass("active");

    // Add 'active' class to the clicked menu item
    $(this).closest(".menu-item").addClass("active");

    // Set parent menu-item with 'active' class
    $(this).closest(".menu-item").parents(".menu-item").addClass("active");
  });
}

// Global function to load components (used by dashboard links)
/**
 * Load a component by name and switch the UI to that component's tab.
 *
 * This helper constructs a tab id as `'#' + componentName`, hides other
 * `.tab-content`, clears any visible error UI then delegates to
 * `loadAndShowTab()` to perform loading and DOM injection. It also updates
 * menu classes and the window.location.hash.
 *
 * @param {string} componentName - Component name (without the leading '#').
 * @returns {void}
 *
 * @example
 * // load component saved as template 'reports' -> '#reports'
 * window.loadComponent('reports');
 */
window.loadComponent = function (componentName) {
  var targetTabId = '#' + componentName;
  $(".tab-content").removeClass("d-block").addClass("d-none");
  $("#error-container").removeClass("d-block").addClass("d-none");
  loadAndShowTab(targetTabId);

  // Update active menu item
  var targetAnchor = $('a[href="' + targetTabId + '"].menu-link:not(.menu-toggle)');
  $(".menu-inner").find(".menu-item").removeClass("active");
  targetAnchor.closest(".menu-item").addClass("active");
  targetAnchor.closest(".menu-item").parents(".menu-item").addClass("active");

  // Update URL hash
  window.location.hash = targetTabId;
};


// Dashboard specific behavior is encapsulated in initDashboardTab so it
// only runs when the dashboard tab is visible / loaded.
/**
 * Update the dashboard clocks.
 * This updates two DOM elements (if present): #dashboard-clock-utc and
 * #dashboard-clock-jkt using Luxon's DateTime. Intended to be called
 * periodically (every second) while the dashboard is visible.
 *
 * @returns {void}
 */
function updateClocks() {
  // Show UTC time and Asia/Jakarta (server) time
  const nowUtc = DateTime.now().toUTC();
  const nowJkt = DateTime.now().setZone("Asia/Jakarta");
  const utcEl = document.getElementById("dashboard-clock-utc");
  const jktEl = document.getElementById("dashboard-clock-jkt");
  if (utcEl) utcEl.textContent = nowUtc.toFormat("HH:mm:ss");
  if (jktEl) jktEl.textContent = nowJkt.toFormat("HH:mm:ss");
}

/**
 * Choose a time-of-day key based on Jakarta server time and update the
 * #greeting element using i18next translations (dashboard.greetings.*).
 * Falls back to a small default string if i18next isn't available.
 *
 * @returns {void}
 */
function updateGreeting() {
  const nowJkt = DateTime.now().setZone('Asia/Jakarta');
  const hour = nowJkt.hour;
  let greetingKey = 'welcome';
  if (hour >= 0 && hour < 5) greetingKey = 'earlyMorning';
  else if (hour < 11) greetingKey = 'morning';
  else if (hour < 15) greetingKey = 'afternoon';
  else if (hour < 18) greetingKey = 'lateAfternoon';
  else greetingKey = 'evening';

  let greeting = '';
  if (typeof i18next !== 'undefined' && i18next.t) {
    greeting = i18next.t('dashboard.greetings.' + greetingKey);
  } else {
    // Minimal safety fallback
    greeting = '🎉 Welcome,';
  }
  // append user display name placeholder
  let userFullname = "N/A";
  if ($('#user-admin-name').text()) {
    userFullname = $('#user-admin-name').text();
  }

  greeting += ' ' + userFullname;
  const greetingEl = document.getElementById('greeting');
  if (greetingEl) greetingEl.textContent = greeting;
}

// Set weather widget theme dynamically
document.addEventListener("DOMContentLoaded", function () {
  const isDarkStyle = typeof window.isDarkStyle === "boolean" ? window.isDarkStyle : false;
  const weatherTheme = isDarkStyle ? "dark" : "original";

  const weatherWidget = document.querySelector(".weatherwidget-io");
  if (weatherWidget) {
    weatherWidget.setAttribute("data-theme", weatherTheme);
  }
});

// Remove automatic startup here — dashboard will be initialized on demand.

!(function (d, s, id) {
  var js, fjs = d.getElementsByTagName(s)[0];
  if (!d.getElementById(id)) {
    js = d.createElement(s);
    js.id = id;
    js.src = "https://weatherwidget.io/js/widget.min.js";
    fjs.parentNode.insertBefore(js, fjs);
  }
})(document, "script", "weatherwidget-io-js");

/**
 * Initialize dashboard tab behaviours.
 * - Starts the clock updater
 * - Renders the greeting text using i18next
 * - Configures the weather widget theme according to current theme
 * - Adds small UI feedback on quick action clicks
 *
 * This function is intentionally idempotent and is safe to call multiple
 * times (it guards against attaching duplicate event handlers).
 *
 * @name window.initDashboardTab
 * @function
 * @returns {void}
 */
// Expose a tab initializer so we can attach behaviors only when the
// dashboard component is loaded. This keeps the global namespace cleaner
// and avoids running dashboard code on other pages.
window.initDashboardTab = (function () {
  // internal state for interval so we can clear if needed
  let dashboardIntervalId = null;

  function setupClocks() {
    // Clear any existing interval so we don't create duplicates when
    // initDashboardTab is called multiple times.
    if (dashboardIntervalId) clearInterval(dashboardIntervalId);

    // Try to use DateTime from luxon if available; otherwise fall back
    // to a simple Date-based formatter so clocks still show.
    function safeUpdate() {
      try {
        // If luxon DateTime exists (we declared const { DateTime } = luxon at top)
        if (typeof DateTime !== 'undefined' && DateTime && DateTime.now) {
          updateClocks();
        } else {
          // fallback to native Date
          const utcEl = document.getElementById('dashboard-clock-utc');
          const jktEl = document.getElementById('dashboard-clock-jkt');
          const now = new Date();
          if (utcEl) utcEl.textContent = now.toISOString().substr(11, 8);
          if (jktEl) {
            // Jakarta offset is +7 hours from UTC (no DST)
            const jkt = new Date(now.getTime() + 7 * 60 * 60 * 1000);
            jktEl.textContent = jkt.toTimeString().substr(0, 8);
          }
        }
      } catch (e) {
        // ignore errors and stop interval to avoid noisy logs
        if (dashboardIntervalId) clearInterval(dashboardIntervalId);
      }
    }

    safeUpdate();
    dashboardIntervalId = setInterval(safeUpdate, 1000);
  }

  /**
   * Update a formatted localized date/time in the hero.
   * Uses Luxon when available and i18next for locale selection.
   * @returns {void}
   */
  function updateHeroDatetime() {
    try {
      const el = document.getElementById('dashboard-datetime');
      if (!el) return;
      const zone = 'Asia/Jakarta';
      const locale = (typeof i18next !== 'undefined' && i18next.language) ? i18next.language : 'en';

      let formatted = '';
      if (typeof DateTime !== 'undefined' && DateTime && DateTime.now) {
        // Format like: Wednesday, 26 November 2025 09:50:00 AM
        const dt = DateTime.now().setZone(zone).setLocale(locale);
        // Use a deterministic format to match example (weekday, dd month yyyy hh:mm:ss AM/PM)
        formatted = dt.toFormat("EEEE, dd LLLL yyyy hh:mm:ss a");
      } else {
        const now = new Date();
        // Fallback naive UTC->JKT offset
        const jkt = new Date(now.getTime() + 7 * 60 * 60 * 1000);
        // Simple english format fallback
        formatted = jkt.toLocaleString(locale, {
          weekday: 'long',
          day: '2-digit',
          month: 'long',
          year: 'numeric',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: true
        });
      }

      el.textContent = formatted;
    } catch (e) {
      // ignore formatting errors
    }
  }

  function setupGreeting() {
    updateGreeting();
  }

  function setupHeroDatetime() {
    // Run once and start an interval to keep the displayed datetime updated
    updateHeroDatetime();
    // If there is already a dashboardIntervalId that updates clocks, we can rely on that
    // to also update the hero datetime. Otherwise start a local interval.
    if (!dashboardIntervalId) {
      dashboardIntervalId = setInterval(updateHeroDatetime, 1000);
    } else {
      // Attach an additional updater that runs alongside the main clock update
      // but avoid creating too many intervals — attach a short-lived updater.
      const dtUpdater = setInterval(updateHeroDatetime, 1000);
      // we won't clear dtUpdater here; it will be cleaned when page unloads.
    }
  }

  function setupWeatherWidget() {
    const weatherWidget = document.querySelector('.weatherwidget-io');
    if (!weatherWidget) return;

    // Choose theme based on light / dark.
    // Detect dark mode reliably from available APIs
    const isDark = (typeof window.isDarkStyle === 'boolean' && window.isDarkStyle) ||
      (typeof window.Helpers !== 'undefined' && typeof window.Helpers.isDarkStyle === 'function' && window.Helpers.isDarkStyle()) ||
      (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
    const theme = isDark ? 'random_grey' : 'retro-sky';
    weatherWidget.setAttribute('data-theme', theme);

    // Make sure the widget script is loaded
    (function (d, s, id) {
      var js, fjs = d.getElementsByTagName(s)[0];
      if (!d.getElementById(id)) {
        js = d.createElement(s);
        js.id = id;
        js.src = 'https://weatherwidget.io/js/widget.min.js';
        fjs.parentNode.insertBefore(js, fjs);
      } else {
        // If script is already present we might need to re-run widget init
        try { window.__weatherwidget_init && window.__weatherwidget_init(); } catch (e) { /* ignore */ }
      }
    })(document, 'script', 'weatherwidget-io-js');
  }

  function setupQuickActionFeedback() {
    const quickActionLinks = document.querySelectorAll('a[onclick*="loadComponent"]');
    quickActionLinks.forEach(link => {
      // avoid attaching multiple times
      if (link.dataset.dashboardInit === '1') return;
      link.dataset.dashboardInit = '1';
      link.addEventListener('click', function (e) {
        const card = this.querySelector('.card');
        if (card) {
          card.style.opacity = '0.7';
          setTimeout(() => { card.style.opacity = '1'; }, 300);
        }
      });
    });
  }

  return function initDashboardTab() {
    try { /* initDashboardTab called */ } catch (e) { }
    // Localize any data-i18n strings inside the dashboard (ensures quick actions
    // and other labels are translated). If i18next hasn't finished loading yet
    // wait for its initialized event and then call localize.
    if (typeof localize === 'function') {
      try {
        if (typeof i18next !== 'undefined' && i18next && i18next.isInitialized !== true) {
          // attach once
          // i18next not initialized — attaching handler and scheduling fallback localize()
          try { i18next.on && i18next.on('initialized', localize); } catch (e) { /* ignore attach error */ }
          // also call localize later as a fallback
          setTimeout(function () { try { localize(); } catch (e) { } }, 500);
        } else {
          try { /* calling localize() now */ localize(); } catch (e) { /* ignore errors */ }
        }
      } catch (e) { /* ignore */ }
    }
    setupClocks();
    setupGreeting();
    setupHeroDatetime();
    setupWeatherWidget();
    // Ensure localization is applied again after widgets (like weatherwidget) have
    // had a chance to initialize or mutate the DOM. This helps avoid cases where
    // a third-party script overwrites translated values shortly after localize()
    // runs.
    setTimeout(function () { try { if (typeof localize === 'function') localize(); } catch (e) { } }, 400);
    setupQuickActionFeedback();
  };
})();

/**
 * Extracts plain text from HTML input or processes plain text by replacing double quotes with single quotes.
 * @param {string} inputString - The input string to process, can be HTML or plain text.
 * @returns {string} The processed text: plain text extracted from HTML or plain text with quotes replaced.
 */
function extractTxt_HTML(inputString) {
  // Ensure inputString is a string
  if (typeof inputString !== "string") {
    inputString = String(inputString || ""); // Convert to string, default to empty if null/undefined
  }

  // Check if the input contains HTML tags
  const isHTML = /<\/?[a-z][\s\S]*>/i.test(inputString);

  if (!isHTML) {
    // Replace double quotes with single quotes in plain text
    return inputString.replace(/"/g, "'");
  }

  // Create a temporary DOM element to parse the string
  const parser = new DOMParser();

  // Parse the string as an HTML document
  const doc = parser.parseFromString(inputString, "text/html");

  // Check for any parsing errors
  const errorNode = doc.querySelector("parsererror");
  if (errorNode) {
    return ""; // Return an empty string if the input is invalid HTML
  }

  // Extract the plain text
  const plainText = doc.body.textContent || "";

  // Replace double quotes with single quotes in the extracted text
  return plainText.replace(/"/g, "'");
}

/**
 * Enables inline editing for table cells with classes 'editable', 'selectable-suggestion', 'selectable-choice', and deletion for 'deleteable' elements.
 * Handles double-click to edit, confirmation dialogs, and server updates via PATCH/DELETE requests.
 * Uses SweetAlert for confirmations and i18next for internationalization.
 * @returns {void}
 */
function enableEditableCells() {
  document.querySelectorAll(".editable").forEach(function (element) {
    let originalValue;
    element.addEventListener("dblclick", function () {
      // Store the original value before editing
      originalValue = this.textContent;

      // Add a new class
      this.classList.add("editing", "border", "border-primary");

      // Set the element to be editable
      this.setAttribute("contenteditable", "true");
      this.focus();
    });

    element.addEventListener("blur", function () {
      saveOrRevert.call(this);
    });

    element.addEventListener("keydown", function (event) {
      if (event.key === "Enter") {
        event.preventDefault(); // Prevents a new line from being added
        this.blur(); // Trigger blur event to save the changes
      } else if (event.key === "Escape") {
        event.preventDefault();
        this.textContent = originalValue; // Revert to original value
        this.blur(); // Trigger blur event to remove editing state
      }
    });

    function saveOrRevert() {
      if (this.textContent != originalValue) {
        // Show SweetAlert confirmation dialog
        var html = "";
        const pass = this.getAttribute("pass");
        if (pass == "true") {
          html = `
                    <input type="text" id="username" class="swal2-input" placeholder="${i18next.t('table.usernamePlaceholder')}">
                    <input type="password" id="password" class="swal2-input" placeholder="${i18next.t('table.passwordPlaceholder')}">
                    `;
        }
        Swal.fire({
          title: i18next.t('table.confirmSaveTitle'),
          html: html,
          showCancelButton: true,
          confirmButtonText: i18next.t('table.saveButton'),
          cancelButtonText: i18next.t('table.cancelButton'),
        }).then((result) => {
          if (result.isConfirmed) {
            var req_data = {};
            // User confirmed, make a PATCH request
            const updatedValue = this.textContent;
            const field = this.getAttribute("field");
            const point = this.getAttribute("point");
            const patch = this.getAttribute("patch");
            var usernameElement = document.getElementById("username");
            if (usernameElement) {
              req_data.username = usernameElement.value;
            }
            var passwordElement = document.getElementById("password");
            if (passwordElement) {
              req_data.password = passwordElement.value;
            }
            req_data.id = point;
            req_data.field = field;
            req_data.value = updatedValue;
            // console.log(field, patch, point);
            fetch(patch, {
              method: "PATCH",
              headers: {
                "Content-Type": "application/json",
              },
              body: JSON.stringify(req_data),
            })
              .then((response) => response.json())
              .then((data) => {
                Swal.fire({
                  icon: "success",
                  title: i18next.t('table.successTitle'),
                  text: data.msg,
                  timer: 3000, // Timer set to 3 seconds
                  timerProgressBar: true, // Display timer progress bar
                });
              })
              .catch((error) => {
                console.error("Error:", error);
                Swal.fire({
                  icon: "error",
                  title: i18next.t('table.errorTitle'),
                  text: error.message || i18next.t('table.unexpectedError'),
                });
              });
          } else {
            // User canceled, revert to the original value
            this.textContent = originalValue;
          }
          // Remove the editable attribute and the additional class
          this.removeAttribute("contenteditable");
          this.classList.remove("editing", "border", "border-primary");
        });
      } else {
        this.removeAttribute("contenteditable");
        this.classList.remove("editing", "border", "border-primary");
      }
    }
  });
  document
    .querySelectorAll(".selectable-suggestion")
    .forEach(function (element) {
      let originalValue;
      let inputElement;
      let validValues = [];

      element.addEventListener("dblclick", function () {
        // Store the original value from the 'data-origin' attribute
        originalValue =
          this.getAttribute("data-origin") || this.textContent.trim();

        // Create the input field with the original value
        inputElement = document.createElement("input");
        inputElement.className = "form-control typeahead-default-suggestions";
        inputElement.type = "text";
        inputElement.value = originalValue;
        inputElement.setAttribute("autocomplete", "off");

        // Replace the element's content with the input field
        this.textContent = "";
        this.appendChild(inputElement);

        // Initialize typeahead on the new input element
        $(".typeahead-default-suggestions").typeahead(
          {
            hint: !isRtl,
            highlight: true,
            minLength: 0,
          },
          {
            name: "selects",
            source: renderDefaults,
          }
        );

        // Focus on the input field
        inputElement.focus();

        // Fetch valid values from the server based on 'select-option' attribute
        const selectOptionUrl = this.getAttribute("select-option");
        if (selectOptionUrl) {
          fetch(selectOptionUrl)
            .then((response) => response.json())
            .then((data) => {
              validValues = data;
              inputElement.value = ""; // Empty the input field
              inputElement.dispatchEvent(new Event("input")); // Manually trigger the input event
            })
            .catch((error) =>
              console.error("Error fetching valid values:", error)
            );
        }
        var prefetchExample = new Bloodhound({
          datumTokenizer: Bloodhound.tokenizers.whitespace,
          queryTokenizer: Bloodhound.tokenizers.whitespace,
          prefetch: selectOptionUrl,
        });

        function renderDefaults(q, sync) {
          if (q === "") {
            sync(prefetchExample.get(...validValues));
          } else {
            prefetchExample.search(q, sync);
          }
        }
      });

      element.addEventListener(
        "blur",
        function () {
          if (inputElement) {
            saveOrRevert.call(this);
          }
        },
        true
      );

      element.addEventListener("keydown", function (event) {
        if (inputElement) {
          if (event.key === "Enter") {
            event.preventDefault();
            inputElement.blur(); // Trigger blur event to save the changes
          } else if (event.key === "Escape") {
            event.preventDefault();
            revertChanges.call(this);
          }
        }
      });

      function saveOrRevert() {
        const updatedValue = inputElement.value.trim();
        if (updatedValue !== originalValue && validateValue(updatedValue)) {
          Swal.fire({
            title: i18next.t('table.confirmSaveTitle'),
            showCancelButton: true,
            confirmButtonText: i18next.t('table.saveButton'),
            cancelButtonText: i18next.t('table.cancelButton'),
          }).then((result) => {
            if (result.isConfirmed) {
              const field = this.getAttribute("field");
              const point = this.getAttribute("point");
              const patch = this.getAttribute("patch");
              fetch(patch, {
                method: "PATCH",
                headers: {
                  "Content-Type": "application/json",
                },
                body: JSON.stringify({
                  id: point,
                  field: field,
                  value: updatedValue,
                }),
              })
                .then((response) => response.json())
                .then((data) => {
                  Swal.fire({
                    icon: "success",
                    title: i18next.t('table.successTitle'),
                    text: i18next.t('table.updateSuccess'),
                    timer: 3000,
                    timerProgressBar: true,
                  });
                  this.setAttribute("data-origin", updatedValue); // Update the data-origin attribute
                  this.textContent = updatedValue;
                })
                .catch((error) => {
                  console.error("Error:", error);
                  Swal.fire({
                    icon: "error",
                    title: i18next.t('table.errorTitle'),
                    text: error,
                  });
                  this.textContent = originalValue;
                });
            } else {
              this.textContent = originalValue;
            }
            cleanUp.call(this);
          });
        } else {
          this.textContent = originalValue;
          cleanUp.call(this);
        }
      }

      function revertChanges() {
        const updatedValue = inputElement.value.trim();
        if (!validateValue(updatedValue)) {
          this.textContent = originalValue; // Revert to the original value if invalid
        } else {
          this.textContent = updatedValue; // Keep the entered value if valid
        }
        cleanUp.call(this);
      }

      function validateValue(value) {
        // Ensure that the value exists in the fetched dataset
        return validValues.includes(value);
      }

      function cleanUp() {
        if (inputElement) {
          inputElement.remove();
        }
        this.classList.remove("editing", "border", "border-primary");
      }
    });

  document.querySelectorAll(".selectable-choice").forEach(function (element) {
    let originalValue;
    element.addEventListener("dblclick", function () {
      // Store the original value before editing
      originalValue = this.textContent;

      const choicesStr = element.getAttribute("choices");
      // Create a dropdown menu with choices fetched from data
      const choicesArray = choicesStr.split(",");

      const dropdownHtml = `
                <select class="form-select">
                    ${choicesArray
          .map(
            (choice) =>
              `<option value="${choice}" ${element.getAttribute("origin") == choice
                ? "selected"
                : ""
              }>${choice}</option>`
          )
          .join("")}
                </select>
            `;

      // Replace the content with the dropdown menu
      this.innerHTML = dropdownHtml;

      // Add an event listener to the dropdown to handle saving
      const dropdown = this.querySelector("select");
      dropdown.focus(); // Focus on the dropdown
      dropdown.addEventListener("change", saveOrRevert);
    });
    element.addEventListener("blur", saveOrRevert);
    function saveOrRevert(event) {
      const updatedValue = event.target.value;
      if (updatedValue !== element.getAttribute("origin")) {
        // Show SweetAlert confirmation dialog
        Swal.fire({
          title: i18next.t('table.confirmSaveTitle'),
          showCancelButton: true,
          confirmButtonText: i18next.t('table.saveButton'),
          cancelButtonText: i18next.t('table.cancelButton'),
        }).then((result) => {
          if (result.isConfirmed) {
            // User confirmed, make a PATCH request
            const field = element.getAttribute("field");
            const point = element.getAttribute("point");
            const patch = element.getAttribute("patch");
            const tab = element.getAttribute("tab");
            fetch(patch, {
              method: "PATCH",
              headers: {
                "Content-Type": "application/json",
              },
              body: JSON.stringify({
                id: point,
                field: field,
                value: updatedValue,
              }),
            })
              .then((response) => response.json())
              .then((data) => {
                Swal.fire({
                  icon: "success",
                  title: i18next.t('table.successTitle'),
                  text: i18next.t('table.updateSuccess'),
                  timer: 3000, // Timer set to 3 seconds
                  timerProgressBar: true, // Display timer progress bar
                });
                $(`#${tab}`).DataTable().ajax.reload();
              })
              .catch((error) => {
                console.error("Error:", error);
                Swal.fire({
                  icon: "error",
                  title: i18next.t('table.errorTitle'),
                  text: error,
                });
                $(`#${tab}`).DataTable().ajax.reload();
              });
          } else {
            // User canceled, revert to the original value
            element.textContent = element.getAttribute("origin");
          }
        });
      } else {
        element.textContent = element.getAttribute("origin");
      }
    }
  });

  document.querySelectorAll(".deleteable").forEach(function (element) {
    element.addEventListener("click", function () {
      const parentColumn = $(this).closest("tr");
      // Add the glowing red background class
      parentColumn.addClass("glowing-red-background");
      var html = "";
      const pass = this.getAttribute("pass");
      if (pass == "true") {
        html = `
                    <input type="text" id="username" class="swal2-input" placeholder="Username">
                    <input type="password" id="password" class="swal2-input" placeholder="Password">
                    `;
      }
      Swal.fire({
        title: i18next.t('table.confirmDeleteTitle'),
        html: html,
        text: i18next.t('table.confirmDeleteText'),
        position: "top",
        showCancelButton: true,
        confirmButtonText: `${i18next.t('table.deleteButton')} (5)`,
        customClass: {
          popup: "border border-danger border-5",
          confirmButton: "btn btn-danger",
          cancelButton: "btn btn-primary",
        },
        buttonsStyling: false,
        didOpen: () => {
          const confirmButton = Swal.getConfirmButton();
          confirmButton.disabled = true;
          let timerInterval;
          let timer = 5;

          timerInterval = setInterval(() => {
            timer--;
            confirmButton.textContent = `${i18next.t('table.deleteButton')} (${timer})`;

            if (timer <= 0) {
              clearInterval(timerInterval);
              confirmButton.textContent = i18next.t('table.deleteButton');
              confirmButton.disabled = false;
            }
          }, 1000);
        },
      }).then((result) => {
        if (result.isConfirmed) {
          const tab = this.getAttribute("tab");
          const deleteable = this.getAttribute("delete");
          // console.log(field, patch, point);
          fetch(deleteable, {
            method: "DELETE",
            headers: {
              "Content-Type": "application/json",
            },
            // body: JSON.stringify({ id: point, field: field, value: updatedValue })
          })
            .then((response) => response.json())
            .then((data) => {
              if (data.error) {
                Swal.fire({
                  icon: "error",
                  title: i18next.t('table.errorTitle'),
                  text: data.error,
                });
                $(`#${tab}`).DataTable().ajax.reload();
              } else {
                Swal.fire({
                  icon: "success",
                  title: i18next.t('table.successTitle'),
                  text: i18next.t('table.deleteSuccess'),
                  timer: 3000, // Timer set to 3 seconds
                  timerProgressBar: true, // Display timer progress bar
                });
                $(`#${tab}`).DataTable().ajax.reload();
              }
            })
            .catch((error) => {
              console.error("Error:", error);
              Swal.fire({
                icon: "error",
                title: i18next.t('table.errorTitle'),
                text: error,
              });
            });
        }
        parentColumn.removeClass("glowing-red-background");
      });
    });
  });
}