// Using PDF.js

(function () {
  // Create modal HTML if not present
  if (!document.getElementById("modal-pdf-global")) {
    var modalDiv = document.createElement("div");
    modalDiv.id = "modal-pdf-global";
    modalDiv.className = "iziModal";
    modalDiv.setAttribute("data-izimodal-title", "PDF Viewer");
    modalDiv.style.display = "none";
    modalDiv.innerHTML = `
      <div id="toolbar-pdf-global" class="d-flex align-items-center gap-2 p-2 border-bottom bg-white shadow-sm">
        <button id="prev-pdf-global" class="btn btn-sm btn-outline-primary">&#8592; Prev</button>
        <button id="next-pdf-global" class="btn btn-sm btn-outline-primary">Next &#8594;</button>
        <span class="ms-2">Page: <span id="page_num-pdf-global">1</span> / <span id="page_count-pdf-global">?</span></span>
        <div class="ms-auto d-flex gap-2">
          <button id="download-pdf-global" class="btn btn-sm btn-outline-info">Download</button>
          <button id="zoom_in-pdf-global" class="btn btn-sm btn-outline-success">+</button>
          <button id="zoom_out-pdf-global" class="btn btn-sm btn-outline-danger">-</button>
          <button id="zoom_reset-pdf-global" class="btn btn-sm btn-outline-secondary">Reset</button>
        </div>
      </div>
      <div class="d-flex justify-content-center align-items-start bg-secondary-subtle p-3" style="height:500px;overflow:auto;">
        <canvas id="canvas-pdf-global" class="shadow rounded bg-white" style="display:none;"></canvas>
      </div>
    `;
    document.body.appendChild(modalDiv);
  }

  document.addEventListener("DOMContentLoaded", function () {
    // Support both inline onclick and event delegation
    document.body.addEventListener("click", function (e) {
      if (e.target.closest(".view-pdf-btn")) {
        var btn = e.target.closest(".view-pdf-btn");
        var pdfUrl = btn.getAttribute("data-pdf-url");
        if (pdfUrl) openPDFModelForPDFJS(pdfUrl);
      }
    });
  });

  window.openPDFModelForPDFJS = function (pdfUrl) {
    var modal = document.getElementById("modal-pdf-global");
    if (
      modal &&
      typeof $(modal).iziModal === "function" &&
      !$(modal).hasClass("iziModal-initialized")
    ) {
      $(modal).iziModal({
        width: 900,
        fullscreen: true,
        overlayClose: false,
        closeOnEscape: false,
        zindex: 9999,
      });
      $(modal).addClass("iziModal-initialized");
    }
    if (modal && typeof $(modal).iziModal === "function") {
      $(modal).iziModal("open");
    }
    if (window.loadPDFUsingPDFJsViewerOfPDFJS) {
      window.loadPDFUsingPDFJsViewerOfPDFJS(pdfUrl);
    }
  };

  window.loadPDFUsingPDFJsViewerOfPDFJS = function (pdfUrl) {
    var pdfDoc = null,
      pageNum = 1,
      scale = 1.2;
    var canvas = document.getElementById("canvas-pdf-global");
    var ctx = canvas.getContext("2d");
    canvas.style.display = "none";
    async function renderPage(num) {
      canvas.style.display = "none";
      var page = await pdfDoc.getPage(num);
      var viewport = page.getViewport({ scale: scale });
      canvas.height = viewport.height;
      canvas.width = viewport.width;
      await page.render({ canvasContext: ctx, viewport: viewport }).promise;
      document.getElementById("page_num-pdf-global").textContent = num;
      canvas.style.display = "";
    }

    // Fix canvas cropping in fullscreen
    function resizeCanvasForFullscreen() {
      var modal = document.getElementById("modal-pdf-global");
      if (document.fullscreenElement === modal) {
        // Set canvas width/height to modal size
        var modalRect = modal.getBoundingClientRect();
        canvas.width = modalRect.width - 40; // padding
        canvas.height = modalRect.height - 100; // toolbar + padding
        renderPage(pageNum);
      }
    }
    document.addEventListener("fullscreenchange", resizeCanvasForFullscreen);

    async function loadPDFUsingPDFJs() {
      try {
        pdfDoc = await pdfjsLib.getDocument(pdfUrl).promise;
        document.getElementById("page_count-pdf-global").textContent =
          pdfDoc.numPages;
        renderPage(pageNum);
      } catch (error) {
        console.error("Error loading PDF:", error);
        document.getElementById("canvas-pdf-global").style.display = "none";
        var errorDiv = document.createElement("div");
        errorDiv.className = "alert alert-danger text-center";
        errorDiv.innerHTML = "Failed to load PDF. The file may not exist or is inaccessible.";
        var container = document.querySelector("#modal-pdf-global .d-flex.justify-content-center");
        container.innerHTML = "";
        container.appendChild(errorDiv);
      }
    }
    document.getElementById("prev-pdf-global").onclick = function () {
      if (pageNum > 1) {
        pageNum--;
        renderPage(pageNum);
      }
    };
    document.getElementById("next-pdf-global").onclick = function () {
      if (pageNum < pdfDoc.numPages) {
        pageNum++;
        renderPage(pageNum);
      }
    };
    document.getElementById("zoom_in-pdf-global").onclick = function () {
      scale += 0.2;
      renderPage(pageNum);
    };
    document.getElementById("zoom_out-pdf-global").onclick = function () {
      if (scale > 0.4) {
        scale -= 0.2;
        renderPage(pageNum);
      }
    };
    document.getElementById("zoom_reset-pdf-global").onclick = function () {
      scale = 1.2;
      renderPage(pageNum);
    };
    document.getElementById("download-pdf-global").onclick = function () {
      var link = document.createElement('a');
      link.href = pdfUrl;
      link.download = pdfUrl.split('/').pop() || 'document.pdf';
      link.click();
    };
    loadPDFUsingPDFJs();
  };
})();
