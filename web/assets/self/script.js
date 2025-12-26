// Wait for the document to be ready
$(document).ready(function () {
  // login
  function checkLoginFields() {
    var email = $("#email").val();
    var password = $("#password").val();
    // Remove captcha requirement since we're using math captcha modal
    // var captcha = $("#captcha").val();

    // Enable or disable sign-in button based on conditions
    if (
      email.trim() !== "" &&
      password.trim() !== ""
      // captcha.trim() !== "" // Removed captcha requirement
    ) {
      $("#signInBtn").prop("disabled", false);
    } else {
      $("#signInBtn").prop("disabled", true);
    }
    if (email.trim() !== "") {
      $("#email").removeClass("glowing-border");
    } else {
      $("#email").addClass("glowing-border");
    }
    if (password.trim() !== "") {
      $("#password-container").removeClass("glowing-border");
    } else {
      $("#password-container").addClass("glowing-border");
    }
    // Remove captcha field validation since we're using math captcha modal
    /*
    if (captcha.trim() !== "") {
      $("#captcha").removeClass("glowing-border");
    } else {
      $("#captcha").addClass("glowing-border");
    }
    */
  }
  // Attach keyup event listener to input fields (removed captcha since we use math captcha)
  $("#email, #password").keyup(function () {
    checkLoginFields(this);
  });

  let $captcha = $("#captcha");

  $captcha.on("input", function (event) {
    if ($captcha.val().length > 6) {
      $captcha.val($captcha.val().slice(0, 6)); // Trim to 6 characters
      return;
    }

    if ($captcha.val()) {
      // Check if captcha value exists
      console.log($captcha.val());
      var epochTime = Date.now();
      timeEvents.push(epochTime);
      captchaEvents.push($captcha.val());
    } else {
      timeEvents = [];
      captchaEvents = [];
    }
  });

  // $("#loading-container").hide();
  // Math captcha variables
  let mathNum1, mathNum2, mathResult;

  // Function to generate math captcha
  function generateMathCaptcha() {
    mathNum1 = Math.floor(Math.random() * 10) + 1; // 1-10
    mathNum2 = Math.floor(Math.random() * 10) + 1; // 1-10
    mathResult = mathNum1 + mathNum2;
    return `${mathNum1} + ${mathNum2} = ?`;
  }

  // Function to show math captcha modal
  function showMathCaptcha() {
    const mathQuestion = generateMathCaptcha();

    return new Promise((resolve) => {
      Swal.fire({
        title: 'Security Check',
        html: `<p>Please solve this simple math problem:</p><h3>${mathQuestion}</h3>`,
        input: 'number',
        inputAttributes: {
          autocapitalize: 'off',
          placeholder: 'Enter your answer'
        },
        showCancelButton: true,
        icon: 'warning',
        confirmButtonText: 'Submit',
        cancelButtonText: 'Cancel',
        allowOutsideClick: false,
        inputValidator: (value) => {
          if (!value) {
            return 'Please enter an answer!'
          }
        }
      }).then((result) => {
        if (result.isConfirmed) {
          const userAnswer = parseInt(result.value);
          if (userAnswer === mathResult) {
            resolve(true); // Correct answer
          } else {
            // Wrong answer - show error and regenerate
            Swal.fire({
              icon: 'error',
              title: 'Incorrect Answer',
              html: `The correct answer was <strong>${mathResult}</strong>.<br>Please try again with a new math problem.`,
              confirmButtonText: 'Try Again'
            }).then(() => {
              showMathCaptcha().then(resolve); // Recursively show new captcha
            });
          }
        } else {
          resolve(false); // User cancelled
        }
      });
    });
  }

  // Attach a submit event handler to the form
  $("#formLoginAuthentication").submit(function (e) {
    e.preventDefault();

    // Show math captcha before proceeding with login
    showMathCaptcha().then((captchaResult) => {
      if (captchaResult === true) {
        // Captcha passed, proceed with login
        var formData = {
          "email-username": $("#email").val(),
          password: $("#password").val(),
          "remember-me": $("#remember-me").is(":checked"),
          captcha: $("#captcha").val(),
          "time-event": JSON.stringify(timeEvents),
          "captcha-event": JSON.stringify(captchaEvents),
        };

        // Proceed with the actual login request
        performLogin(formData);
      }
      // If captcha failed or was cancelled, do nothing (stay on login page)
    });
  });

  // Function to perform the actual login
  function performLogin(formData) {
    $.ajax({
      type: "POST",
      url: "/login",
      data: formData,
      success: function (response) {
        console.log(response);
        window.location.reload();
      },
      error: function (error) {
        // refreshCaptcha(); // uncomment this soon !!

        if (error.responseJSON) {
          if (error.responseJSON.msg) {
            let minutes = error.responseJSON.minutes || 0;
            let seconds = error.responseJSON.seconds || 0;
            let email = error.responseJSON.email || "Unknown";

            Swal.fire({
              icon: "error",
              title: "Login Failed",
              html: `Login for <strong>${email}</strong> is banned for ${minutes}:${seconds}`,
              confirmButtonText: "OK",
            });

            startCountdown(minutes, seconds, email);
          } else if (error.responseJSON.error) {
            // Handle Gin's error response
            Swal.fire({
              icon: "error",
              title: "Error",
              text: error.responseJSON.error,
              confirmButtonText: "OK",
            });
          } else {
            // Default error message if no expected keys exist
            Swal.fire({
              icon: "error",
              title: "Unexpected Error",
              text: "An unknown error occurred. Please try again.",
              confirmButtonText: "OK",
            });
          }
        } else {
          // Handle cases where error.responseJSON is undefined
          Swal.fire({
            icon: "error",
            title: "Server Error",
            text: "No response from the server. Please check your connection.",
            confirmButtonText: "OK",
          });
        }
      },
    });
  }

  function startCountdown(minutes, seconds, email) {
    // Update the timer immediately
    updateTimer(minutes, seconds, email);

    // Set interval to update the countdown every second
    let countdown = setInterval(function () {
      if (seconds === 0) {
        if (minutes === 0) {
          clearInterval(countdown); // Stop the countdown if time runs out
          Swal.fire({
            icon: "success",
            title: "Time Up",
            text: `You can now try logging in again for ${email}.`,
            confirmButtonText: "OK",
          });
          $("#delay_timer").text(``);
        } else {
          minutes--; // Decrement minutes and reset seconds to 59
          seconds = 59;
        }
      } else {
        seconds--; // Decrement seconds
      }

      // Update the timer text in the DOM
      updateTimer(minutes, seconds, email);
    }, 1000);
  }

  // Function to update the timer element in the DOM
  function updateTimer(minutes, seconds, email) {
    // Add leading zero to seconds if necessary
    let formattedSeconds = seconds < 10 ? "0" + seconds : seconds;
    $("#delay_timer").text(
      `Login for ${email} is banned for ${minutes}:${formattedSeconds}`
    );
  }

  // For Avatar badge
  var stateNum = Math.floor(Math.random() * 6);
  var states = [
    "success",
    "danger",
    "warning",
    "info",
    "dark",
    "primary",
    "secondary",
  ];
  var state = states[stateNum];

  //in
  var name = $("#user-admin-name").html(),
    initials = name.match(/\b\w/g) || [];

  if (
    $(".mainAvatar > img").attr("src") == "/assets/img/avatars/default.jpg" ||
    $(".mainAvatar > img").attr("src") == ""
  ) {
    initials = (
      (initials.shift() || "") + (initials.pop() || "")
    ).toUpperCase();
    output = `<span class="avatar-initial rounded-circle bg-label-${state}">${initials}</span>`;
    $(".mainAvatar").html(output);
  }
});

function loadAndShowTab(targetTabId) {
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
      // $(targetTabId).show();
      $(targetTabId).removeClass("d-none").addClass("d-block");
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

            $(targetTabId).html(html);
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
            console.log("Error fetching content:", error);
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
    console.log("Invalid targetTabId:", targetTabId);
  }
}
if (window.location.pathname.indexOf("/page") !== -1) {
  var targetTabId = window.location.hash;

  if (targetTabId == null || targetTabId == "") {
    targetTabId = $(".menu-link:not(.menu-toggle):first").attr("href");
  }
  loadAndShowTab(targetTabId);

  // Handle tab clicks
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

// For Avatar badge
var stateNum = Math.floor(Math.random() * 6);
var states = [
  "success",
  "danger",
  "warning",
  "info",
  "dark",
  "primary",
  "secondary",
];
var state = states[stateNum];

//in
var name = $("#user-admin-name").html(),
  initials = name.match(/\b\w/g) || [];

if (
  $(".mainAvatar > img").attr("src") == "/assets/img/avatars/default.jpg" ||
  $(".mainAvatar > img").attr("src") == ""
) {
  initials = (
    (initials.shift() || "") + (initials.pop() || "")
  ).toUpperCase();
  output = `<span class="avatar-initial rounded-circle bg-label-${state}">${initials}</span>`;
  $(".mainAvatar").html(output);
}

document.querySelectorAll(".horizontal-scrollbar").forEach((element) => {
  new PerfectScrollbar(element, {
    wheelPropagation: false,
    suppressScrollY: true,
  });
});
function jsonToHtml(jsonData) {
  var container = document.getElementById("mainCourseContent");

  function createHtmlElement(tag, attributes, content) {
    var element = document.createElement(tag);

    // Set attributes
    for (var key in attributes) {
      if (key === "tag" || key === "child") {
        continue;
      }
      if (attributes.hasOwnProperty(key)) {
        element.setAttribute(key, attributes[key]);
      }
    }

    // Set content
    if (content instanceof Array) {
      content.forEach(function (child) {
        var childElement = createHtmlElement(child.tag, child, child.child);
        element.appendChild(childElement);
      });
    } else if (typeof content === "string") {
      const parser = new DOMParser();
      try {
        const doc = parser.parseFromString(content, "text/html");
        if (
          Array.from(doc.body.childNodes).some((node) => node.nodeType === 1)
        ) {
          element.innerHTML += content;
        } else {
          element.appendChild(document.createTextNode(content));
        }
      } catch (error) {
        element.appendChild(document.createTextNode(content));
      }
    }

    return element;
  }

  var path = window.location.pathname;

  // Extract the relevant parts from the path
  var parts = path.split("/").filter((part) => part !== "");

  // Convert the parts into the desired format
  var stringId = parts
    .map((part, index) => {
      if (index % 2 === 0) {
        // Even indices are section names (e.g., "courses", "modules", "contents")
        return part.substring(0, 2);
      } else {
        // Odd indices are section IDs (e.g., "1", "1", "3")
        return `-${part}-`;
      }
    })
    .join("");

  var i = 0;
  jsonData.forEach(function (item) {
    if (document.getElementById("userLevel").value === "true") {
      const viewBtn = document.getElementById("viewBtn");
      viewBtn.classList.remove("d-none");
      viewBtn.classList.add("d-flex");

      var validId = stringId + i;

      // Create a container div to wrap the buttons
      var buttonContainer = document.createElement("div");

      // Create the edit button
      var editButton = document.createElement("button");
      editButton.id = "change-" + validId;
      editButton.className = "btn text-primary edit-btn px-1";
      editButton.innerHTML = 'Edit <i class="bx bx-edit"></i>';
      editButton.addEventListener("click", function () {
        editContents(validId);
      });

      // Create the delete button
      var deleteButton = document.createElement("button");
      deleteButton.id = "delete-" + validId;
      deleteButton.className = "btn text-danger edit-btn  px-1";
      deleteButton.innerHTML = '<i class="bx bx-trash" ></i>';
      deleteButton.addEventListener("click", function () {
        deleteContents(validId);
      });

      // Append both buttons to the container div
      buttonContainer.className = "align-self-end d-flex flex-row";
      buttonContainer.appendChild(editButton);
      buttonContainer.appendChild(deleteButton);

      // Append the container div to your main container
      container.appendChild(buttonContainer);
    }

    var element = createHtmlElement(item.tag, item, item.child);
    element.id = validId;
    container.appendChild(element);
    i++;
  });

  //Content Button
  var newContentButton = document.createElement("a");
  newContentButton.className =
    "btn d-flex flex-column align-items-center justify-content-center text-primary";
  newContentButton.innerHTML =
    '<i class="bx bx-plus-circle bx-lg"></i>Sub Content';
  newContentButton.addEventListener("click", function () {
    i++;
    var contentID = stringId + i;

    if (document.getElementById("userLevel").value === "true") {
      // Create a container div to wrap the buttons
      var buttonContainer = document.createElement("div");

      // Create the edit button
      var editButton = document.createElement("button");
      editButton.id = "change-" + contentID;
      editButton.className = "btn text-primary edit-btn px-1";
      editButton.innerHTML = 'Save <i class="bx bx-save"></i>';
      editButton.addEventListener("click", function () {
        editContents(contentID);
      });

      // Create the delete button
      var deleteButton = document.createElement("button");
      deleteButton.id = "delete-" + contentID;
      deleteButton.className = "btn text-danger edit-btn  px-1";
      deleteButton.innerHTML = '<i class="bx bx-trash" ></i>';
      deleteButton.addEventListener("click", function () {
        var elementToRemove = document.getElementById(contentID); // Replace with your actual element ID

        if (elementToRemove) {
          var siblingsToRemove = [
            elementToRemove.previousElementSibling,
            elementToRemove.previousElementSibling.previousElementSibling,
          ];

          // Remove the element and its two above siblings
          elementToRemove.remove();
          siblingsToRemove.forEach(function (sibling) {
            if (sibling) {
              sibling.remove();
            }
          });
        }
      });

      // Append both buttons to the container div
      buttonContainer.className = "align-self-end d-flex flex-row";
      buttonContainer.appendChild(editButton);
      buttonContainer.appendChild(deleteButton);

      // Append the container div to your main container
      container.appendChild(buttonContainer);

      //create New div
      var div = document.createElement("div");
      div.id = contentID;
      container.appendChild(div);
    }
    const fullToolbar = [
      [
        {
          font: [],
        },
        {
          size: [],
        },
      ],
      ["bold", "italic", "underline", "strike"],
      [
        {
          color: [],
        },
        {
          background: [],
        },
      ],
      [
        {
          script: "super",
        },
        {
          script: "sub",
        },
      ],
      [
        {
          header: "1",
        },
        {
          header: "2",
        },
        "blockquote",
        "code-block",
      ],
      [
        {
          list: "ordered",
        },
        {
          list: "bullet",
        },
        {
          indent: "-1",
        },
        {
          indent: "+1",
        },
      ],
      [{ direction: "rtl" }],
      ["link", "image", "video", "formula"],
      ["clean"],
    ];
    const fullEditor = new Quill("#" + contentID, {
      bounds: "#" + contentID,
      placeholder: "Type Something...",
      modules: {
        formula: true,
        toolbar: fullToolbar,
      },
      theme: "snow",
    });
  });
  var newVideoButton = document.createElement("a");
  newVideoButton.className =
    "btn d-flex flex-column align-items-center justify-content-center text-primary";
  newVideoButton.innerHTML = '<i class="bx bx-video-plus bx-lg"></i>Video';
  newVideoButton.addEventListener("click", function () {
    i++;
    var contentID = stringId + i;

    if (document.getElementById("userLevel").value === "true") {
      // Create a container div to wrap the buttons
      var buttonContainer = document.createElement("div");

      // Create the edit button
      var editButton = document.createElement("button");
      editButton.id = "change-" + contentID;
      editButton.className = "btn text-primary edit-btn px-1";
      editButton.innerHTML = 'Save <i class="bx bx-save"></i>';
      editButton.addEventListener("click", function () {
        editContents(contentID, "video", true);
      });

      // Create the delete button
      var deleteButton = document.createElement("button");
      deleteButton.id = "delete-" + contentID;
      deleteButton.className = "btn text-danger edit-btn  px-1";
      deleteButton.innerHTML = '<i class="bx bx-trash" ></i>';
      deleteButton.addEventListener("click", function () {
        var elementToRemove = document.getElementById(contentID); // Replace with your actual element ID

        if (elementToRemove) {
          var siblingsToRemove = [elementToRemove.previousElementSibling];

          // Remove the element and its two above siblings
          elementToRemove.remove();
          siblingsToRemove.forEach(function (sibling) {
            if (sibling) {
              sibling.remove();
            }
          });
        }
      });

      // Append both buttons to the container div
      buttonContainer.className = "align-self-end d-flex flex-row";
      buttonContainer.appendChild(editButton);
      buttonContainer.appendChild(deleteButton);

      // Append the container div to your main container
      container.appendChild(buttonContainer);

      //create New div _________________________________________________________
      // Create the main container div
      var wrapperDiv = document.createElement("div");
      wrapperDiv.id = contentID;
      wrapperDiv.className = "wrapper-ua";

      // Create the form element
      var formElement = document.createElement("form");
      formElement.id = "form-" + contentID;
      formElement.className = "form-ua text-primary";
      formElement.action = "#";

      // Create the file input element
      var fileInput = document.createElement("input");
      fileInput.className = "file-input";
      fileInput.type = "file";
      fileInput.name = "file";
      fileInput.hidden = true;

      // Create the cloud upload icon
      var cloudUploadIcon = document.createElement("i");
      cloudUploadIcon.className = "bx bx-cloud-upload text-primary";

      // Create the paragraph element
      var paragraphElement = document.createElement("p");
      paragraphElement.className = "m-0";
      paragraphElement.textContent = "Click To Upload Video";

      // Append elements to the form element
      formElement.appendChild(fileInput);
      formElement.appendChild(cloudUploadIcon);
      formElement.appendChild(paragraphElement);

      // Create the progress area section
      var progressArea = document.createElement("div");
      progressArea.className = "sec-ua progress-area";

      // Create the uploaded area section
      var uploadedArea = document.createElement("div");
      uploadedArea.className = "sec-ua uploaded-area";

      var uploadedArea = document.createElement("div");
      uploadedArea.className = "sec-ua uploaded-area";

      // Create the card div
      var cardDiv = document.createElement("div");
      cardDiv.innerHTML = `<div class="card-datatable table-responsive">
                <table class="list-video-edit table border-top">
                  <thead>
                    <tr>
                      <th></th>
                      <th>id</th>
                      <th>Name</th>
                      <th>Action</th>
                    </tr>
                  </thead>
                </table>
              </div>
            `;

      // Append all elements to the main container div
      wrapperDiv.appendChild(formElement);
      wrapperDiv.appendChild(progressArea);
      wrapperDiv.appendChild(uploadedArea);
      // Append the card div to the wrapperDiv
      wrapperDiv.appendChild(cardDiv);

      // Append the main container div to the document body (or any other parent element)
      container.appendChild(wrapperDiv);

      const form_uf = document.querySelector("#form-" + contentID);
      if (form_uf) {
        const fileInput = document.querySelector(".file-input");
        const progressArea = document.querySelector(".progress-area");
        const uploadedArea = document.querySelector(".uploaded-area");
        // form click event
        form_uf.addEventListener("click", () => {
          fileInput.click();
        });

        fileInput.onchange = ({ target }) => {
          let file = target.files[0]; //getting file [0] this means if user has selected multiple files then get first one only
          if (file) {
            let fileName = file.name; //getting file name
            if (fileName.length >= 12) {
              //if file name length is greater than 12 then split it and add ...
              let splitName = fileName.split(".");
              fileName = splitName[0].substring(0, 13) + "... ." + splitName[1];
            }
            // if (!(file.name.endsWith('.zip'))) {
            //   Swal.fire('Invalid File, must be .zip', '', 'error');
            //   return;
            // }
            Swal.fire({
              title: "Upload Verification",
              text: `Are you sure you want to upload "${file.name}"?`,
              icon: "question",
              showCancelButton: true,
              confirmButtonText: "Upload!",
              cancelButtonText: "Cancel",
              confirmButtonClass: "btn btn-primary",
              cancelButtonClass: "btn btn-outline-secondary ml-1",
              buttonsStyling: false,
            }).then((result) => {
              if (result.value) {
                // User clicked "Upload"
                uploadFile(file.name); // Replace with your upload function
              } else if (result.dismiss === Swal.DismissReason.cancel) {
                // User clicked "Cancel" or outside the modal
                Swal.fire(
                  "Cancelled",
                  "The upload process was cancelled",
                  "info"
                );
              }
            });
          }
        };
        // file upload function
        function uploadFile(name) {
          var contentValidatorElement =
            document.getElementById("contentValidator").value;
          let xhr = new XMLHttpRequest(); //creating new xhr object (AJAX)
          xhr.open("POST", "/upload/video/" + contentValidatorElement); //sending post request to the specified URL
          xhr.upload.addEventListener("progress", ({ loaded, total }) => {
            //file uploading progress event
            let fileLoaded = Math.floor((loaded / total) * 100); //getting percentage of loaded file size
            let fileTotal = Math.floor(total / 1000); //gettting total file size in KB from bytes
            let fileSize;
            // if file size is less than 1024 then add only KB else convert this KB into MB
            fileTotal < 1024
              ? (fileSize = fileTotal + " KB")
              : (fileSize = (loaded / (1024 * 1024)).toFixed(2) + " MB");
            let progressHTML = `<li class="row px-2">
                              <i class="fas fa-file-alt"></i>
                              <div class="content">
                                <div class="details">
                                <span class="name">${name} • Uploading</span>
                                <span class="percent">${fileLoaded}%</span>
                                </div>
                                <div class="progress-bar">
                                <div class="progress" style="width: ${fileLoaded}%"></div>
                                </div>
                              </div>
                              </li>`;
            // uploadedArea.innerHTML = ""; //uncomment this line if you don't want to show push history
            uploadedArea.classList.add("onprogress");
            progressArea.innerHTML = progressHTML;
            if (loaded == total) {
              progressArea.innerHTML = "";
              let uploadedHTML = `<li class="row px-2">
                                <div class="content upload">
                                <i class="fas fa-file-alt"></i>
                                <div class="details">
                                  <span class="name">${name} • Uploaded</span>
                                  <span class="size">${fileSize}</span>
                                </div>
                                </div>
                                <i class="fas fa-check"></i>
                              </li>`;
              uploadedArea.classList.remove("onprogress");
              // uploadedArea.innerHTML = uploadedHTML; //uncomment this line if you don't want to show push history
              uploadedArea.insertAdjacentHTML("afterbegin", uploadedHTML); //remove this line if you don't want to show push history
            }
          });
          let data = new FormData(form_uf); //FormData is an object to easily send form data
          xhr.onload = function () {
            if (xhr.status === 200) {
              // Successful response from PHP
              const response = JSON.parse(xhr.responseText);
            } else {
              console.error("Error:", xhr.status);
            }
          };
          xhr.send(data); //sending form data
        }
      }
      var dt_basic_table = $(".list-video-edit");
      if (dt_basic_table.length) {
        dt_basic = dt_basic_table.DataTable({
          ajax: "/table/video",
          columns: [
            { data: "" },
            { data: "id" },
            { data: "filename" },
            { data: "" },
          ],
          columnDefs: [
            {
              // For Responsive
              className: "control",
              orderable: false,
              searchable: false,
              responsivePriority: 2,
              targets: 0,
              render: function (data, type, full, meta) {
                return "";
              },
            },
            {
              targets: 1,
              searchable: false,
              visible: false,
            },
            {
              // Avatar image/badge, Name and post
              targets: 2,
              responsivePriority: 4,
              render: function (data, type, full, meta) {
                var $user_src = full["path"],
                  $id = full["id"],
                  $name = full["filename"],
                  $post = full["updated_by"],
                  $type = full["type"];
                $thumbnail = full["thumbnail"];
                if ($user_src) {
                  if ($type === "video") {
                    // For Avatar image
                    var $output =
                      '<video controls="" id="example-plyr-video-player-' +
                      $id +
                      '" playsinline="" poster="' +
                      $thumbnail +
                      '" width="" class="w-100 round" style="max-width:300px;"><source src="' +
                      $user_src +
                      '" type="video/mp4"></video>';
                  } else {
                    var $output =
                      '<img src="' +
                      assetsPath +
                      "img/avatars/" +
                      $user_src +
                      '" alt="Avatar" class="rounded-circle">';
                  }
                } else {
                  // For Avatar badge
                  var stateNum = Math.floor(Math.random() * 6);
                  var states = [
                    "success",
                    "danger",
                    "warning",
                    "info",
                    "dark",
                    "primary",
                    "secondary",
                  ];
                  var $state = states[stateNum],
                    $name = full["filename"],
                    $initials = $name.match(/\b\w/g) || [];
                  $initials = (
                    ($initials.shift() || "") + ($initials.pop() || "")
                  ).toUpperCase();
                  $output =
                    '<span class="avatar-initial rounded-circle bg-label-' +
                    $state +
                    '">' +
                    $initials +
                    "</span>";
                }
                // Creates full output for row
                var $row_output =
                  '<div class="d-flex justify-content-start align-items-center user-name">' +
                  '<div class="d-flex flex-column">' +
                  '<span class="emp_name text-truncate">' +
                  $name +
                  "</span>" +
                  '<small class="emp_post text-truncate text-muted">' +
                  $post +
                  "</small>" +
                  "</div>" +
                  "</div>";
                return $row_output;
              },
            },
            {
              // Actions
              targets: -1,
              title: "Actions",
              orderable: false,
              searchable: false,
              render: function (data, type, full, meta) {
                var $user_src = full["path"];
                return (
                  '<div class="d-inline-block">' +
                  '<a href="javascript:;" class="btn btn-sm btn-icon dropdown-toggle hide-arrow" data-bs-toggle="dropdown"><i class="bx bx-dots-vertical-rounded"></i></a>' +
                  '<ul class="dropdown-menu dropdown-menu-end m-0">' +
                  '<li><a href="javascript:;" class="dropdown-item">Details</a></li>' +
                  '<li><a href="javascript:;" class="dropdown-item">Archive</a></li>' +
                  '<div class="dropdown-divider"></div>' +
                  '<li><a href="javascript:;" class="dropdown-item text-danger delete-record">Delete</a></li>' +
                  "</ul>" +
                  "</div>" +
                  "<a onclick=\"appendVideoFirst('" +
                  wrapperDiv.id +
                  "', `" +
                  $user_src +
                  '` )" class="btn btn-sm btn-icon item-edit"><i class="bx bxs-video-plus"></i></a>'
                );
              },
            },
          ],
          order: [[1, "desc"]],
          dom: '<"card-header flex-column flex-md-row"<"head-label text-center"><"dt-action-buttons text-end pt-3 pt-md-0"B>><"row"<"col-sm-12 col-md-6"l><"col-sm-12 col-md-6 d-flex justify-content-center justify-content-md-end"f>>t<"row"<"col-sm-12 col-md-6"i><"col-sm-12 col-md-6"p>>',
          displayLength: 7,
          lengthMenu: [7, 10, 25, 50, 75, 100],
          buttons: [],
          responsive: {
            details: {
              display: $.fn.dataTable.Responsive.display.modal({
                header: function (row) {
                  var data = row.data();
                  return "Details of " + data["filename"];
                },
              }),
              type: "column",
              renderer: function (api, rowIdx, columns) {
                var data = $.map(columns, function (col, i) {
                  return col.title !== "" // ? Do not show row in modal popup if title is blank (for check box)
                    ? '<tr data-dt-row="' +
                    col.rowIndex +
                    '" data-dt-column="' +
                    col.columnIndex +
                    '">' +
                    "<td>" +
                    col.title +
                    ":" +
                    "</td> " +
                    "<td>" +
                    col.data +
                    "</td>" +
                    "</tr>"
                    : "";
                }).join("");

                return data
                  ? $('<table class="table"/><tbody />').append(data)
                  : false;
              },
            },
          },
        });
        $("div.card-header.flex-column").addClass("d-none");
        dt_basic_table.find("thead").addClass("d-none");
      }
    }
  });
  var newPdfButton = document.createElement("a");
  newPdfButton.className =
    "btn d-flex flex-column align-items-center justify-content-center text-primary";
  newPdfButton.innerHTML = '<i class="bx bxs-file-plus bx-lg" ></i>Pdf';
  newPdfButton.addEventListener("click", function () {
    i++;
    var contentID = stringId + i;

    if (document.getElementById("userLevel").value === "true") {
      // Create a container div to wrap the buttons
      var buttonContainer = document.createElement("div");

      // Create the edit button
      var editButton = document.createElement("button");
      editButton.id = "change-" + contentID;
      editButton.className = "btn text-primary edit-btn px-1";
      editButton.innerHTML = 'Save <i class="bx bx-save"></i>';
      editButton.addEventListener("click", function () {
        editContents(contentID);
      });

      // Create the delete button
      var deleteButton = document.createElement("button");
      deleteButton.id = "delete-" + contentID;
      deleteButton.className = "btn text-danger edit-btn  px-1";
      deleteButton.innerHTML = '<i class="bx bx-trash" ></i>';
      deleteButton.addEventListener("click", function () {
        var elementToRemove = document.getElementById(contentID); // Replace with your actual element ID

        if (elementToRemove) {
          var siblingsToRemove = [
            elementToRemove.previousElementSibling,
            elementToRemove.previousElementSibling.previousElementSibling,
          ];

          // Remove the element and its two above siblings
          elementToRemove.remove();
          siblingsToRemove.forEach(function (sibling) {
            if (sibling) {
              sibling.remove();
            }
          });
        }
      });

      // Append both buttons to the container div
      buttonContainer.className = "align-self-end d-flex flex-row";
      buttonContainer.appendChild(editButton);
      buttonContainer.appendChild(deleteButton);

      // Append the container div to your main container
      container.appendChild(buttonContainer);

      //create New div
      var div = document.createElement("div");
      div.id = contentID;
      container.appendChild(div);
    }
    const fullToolbar = [
      [
        {
          font: [],
        },
        {
          size: [],
        },
      ],
      ["bold", "italic", "underline", "strike"],
      [
        {
          color: [],
        },
        {
          background: [],
        },
      ],
      [
        {
          script: "super",
        },
        {
          script: "sub",
        },
      ],
      [
        {
          header: "1",
        },
        {
          header: "2",
        },
        "blockquote",
        "code-block",
      ],
      [
        {
          list: "ordered",
        },
        {
          list: "bullet",
        },
        {
          indent: "-1",
        },
        {
          indent: "+1",
        },
      ],
      [{ direction: "rtl" }],
      ["link", "image", "video", "formula"],
      ["clean"],
    ];
    const fullEditor = new Quill("#" + contentID, {
      bounds: "#" + contentID,
      placeholder: "Type Something...",
      modules: {
        formula: true,
        toolbar: fullToolbar,
      },
      theme: "snow",
    });
  });
  var newQuizButton = document.createElement("a");
  newQuizButton.className =
    "btn d-flex flex-column align-items-center justify-content-center text-primary";
  newQuizButton.innerHTML = '<i class="bx bx-message-add bx-lg"></i>Quiz';
  newQuizButton.addEventListener("click", function () {
    i++;
    var contentID = stringId + i;

    if (document.getElementById("userLevel").value === "true") {
      // Create a container div to wrap the buttons
      var buttonContainer = document.createElement("div");

      // Create the edit button
      var editButton = document.createElement("button");
      editButton.id = "change-" + contentID;
      editButton.className = "btn text-primary edit-btn px-1";
      editButton.innerHTML = 'Save <i class="bx bx-save"></i>';
      editButton.addEventListener("click", function () {
        editContents(contentID);
      });

      // Create the delete button
      var deleteButton = document.createElement("button");
      deleteButton.id = "delete-" + contentID;
      deleteButton.className = "btn text-danger edit-btn  px-1";
      deleteButton.innerHTML = '<i class="bx bx-trash" ></i>';
      deleteButton.addEventListener("click", function () {
        var elementToRemove = document.getElementById(contentID); // Replace with your actual element ID

        if (elementToRemove) {
          var siblingsToRemove = [
            elementToRemove.previousElementSibling,
            elementToRemove.previousElementSibling.previousElementSibling,
          ];

          // Remove the element and its two above siblings
          elementToRemove.remove();
          siblingsToRemove.forEach(function (sibling) {
            if (sibling) {
              sibling.remove();
            }
          });
        }
      });

      // Append both buttons to the container div
      buttonContainer.className = "align-self-end d-flex flex-row";
      buttonContainer.appendChild(editButton);
      buttonContainer.appendChild(deleteButton);

      // Append the container div to your main container
      container.appendChild(buttonContainer);

      //create New div
      var div = document.createElement("div");
      div.id = contentID;
      container.appendChild(div);
    }
    const fullToolbar = [
      [
        {
          font: [],
        },
        {
          size: [],
        },
      ],
      ["bold", "italic", "underline", "strike"],
      [
        {
          color: [],
        },
        {
          background: [],
        },
      ],
      [
        {
          script: "super",
        },
        {
          script: "sub",
        },
      ],
      [
        {
          header: "1",
        },
        {
          header: "2",
        },
        "blockquote",
        "code-block",
      ],
      [
        {
          list: "ordered",
        },
        {
          list: "bullet",
        },
        {
          indent: "-1",
        },
        {
          indent: "+1",
        },
      ],
      [{ direction: "rtl" }],
      ["link", "image", "video", "formula"],
      ["clean"],
    ];
    const fullEditor = new Quill("#" + contentID, {
      bounds: "#" + contentID,
      placeholder: "Type Something...",
      modules: {
        formula: true,
        toolbar: fullToolbar,
      },
      theme: "snow",
    });
  });

  var newContainerButton = document.createElement("div");
  newContainerButton.className = "add-new-content-btn";
  newContainerButton.className =
    "d-flex flex-row border my-3 align-items-center justify-content-center text-primary edit-btn";
  newContainerButton.appendChild(newContentButton);
  newContainerButton.appendChild(newVideoButton);
  newContainerButton.appendChild(newPdfButton);
  newContainerButton.appendChild(newQuizButton);

  // Insert the newContentButton after the mainCourseContent element
  container.insertAdjacentElement("afterend", newContainerButton);
}

function appendVideoFirst(idWrapper, videoSrc) {
  var parentElement = $("#" + idWrapper);

  // Check if the first child is a div with class 'plyr'
  if (parentElement.children().first().is("div.plyr")) {
    // If it is, remove the div
    parentElement.children().first().remove();
  }

  // Add the videoHTML as the first child of the parent element
  var videoHTML =
    '<video controls="" id="video-player-' +
    idWrapper +
    '" playsinline="" width="" class="w-100" ><source src="' +
    videoSrc +
    '" type="video/mp4"></video>';
  parentElement.prepend(videoHTML);

  // Initialize Plyr for the newly added video element
  const videoElements = document.querySelectorAll("video");
  videoElements.forEach(function (video, index) {
    console.log(video, index);
    if (!video.id) {
      video.id = "video-player-" + (index + 1);
    }
    const videoPlayer = new Plyr("#" + video.id);
  });
}

var contentValidatorElement = document.getElementById("contentValidator");

if (contentValidatorElement) {
  var contentValue = contentValidatorElement.value;

  fetch("/contents/" + contentValue)
    .then(function (response) {
      if (!response.ok) {
        throw new Error("Network response was not ok");
      }
      return response.text();
    })
    .then(function (data) {
      try {
        var parsedData = JSON.parse(data);
        var jsonObject = JSON.parse(parsedData.data);
        // console.log(jsonObject);
        jsonToHtml(jsonObject);
        makeIframeResponsive("ql-video", 3, 2);

        const videoElements = document.querySelectorAll("video");
        videoElements.forEach(function (video, index) {
          // Check if the video element has an ID
          if (!video.id) {
            // If not, set a new ID
            video.id = "video-player-" + (index + 1);
          }
          const videoPlayer = new Plyr("#" + video.id);
        });
      } catch (error) {
        console.error("Error parsing JSON:", error);
      }
    })
    .catch(function (error) {
      console.log("Error fetching content:", error);
    });
}
function makeIframeResponsive(
  containerClass,
  aspectRatioWidth,
  aspectRatioHeight
) {
  // Get all elements with the specified class
  var iframes = document.getElementsByClassName(containerClass);

  // Function to update the height based on the width
  function updateIframeHeight(iframe) {
    iframe.style.width = "100%";
    var currentWidth = iframe.offsetWidth;
    var calculatedHeight =
      (currentWidth * aspectRatioHeight) / aspectRatioWidth;
    iframe.style.height = calculatedHeight + "px";

    // Prevent right-click on the iframe
    iframe.addEventListener("contextmenu", function (e) {
      e.preventDefault();
    });
    // Prevent long press on the iframe (for touchscreen devices)
    iframe.addEventListener("touchstart", function (e) {
      var now = new Date().getTime();
      var delta = now - (iframe.touchstart || now + 1);
      iframe.touchstart = now;
      if (delta < 500 && delta > 0) {
        e.preventDefault();
      }
    });
  }

  // Function to apply the updateIframeHeight function to all elements
  function updateAllIframesHeight() {
    for (var i = 0; i < iframes.length; i++) {
      updateIframeHeight(iframes[i]);
    }
  }

  // Call the function on initial page load
  updateAllIframesHeight();

  // Listen for window resize events to update the height dynamically
  window.addEventListener("resize", updateAllIframesHeight);
}

function editContents(contentID, type, isNewUpload) {
  const btnChange = document.getElementById("change-" + contentID);
  if (btnChange.innerHTML.includes("Edit")) {
    btnChange.innerHTML = 'Save <i class="bx bx-save"></i>';
    const fullToolbar = [
      [
        {
          font: [],
        },
        {
          size: [],
        },
      ],
      ["bold", "italic", "underline", "strike"],
      [
        {
          color: [],
        },
        {
          background: [],
        },
      ],
      [
        {
          script: "super",
        },
        {
          script: "sub",
        },
      ],
      [
        {
          header: "1",
        },
        {
          header: "2",
        },
        "blockquote",
        "code-block",
      ],
      [
        {
          list: "ordered",
        },
        {
          list: "bullet",
        },
        {
          indent: "-1",
        },
        {
          indent: "+1",
        },
      ],
      [{ direction: "rtl" }],
      ["link", "image", "video", "formula"],
      ["clean"],
    ];
    const fullEditor = new Quill("#" + contentID, {
      bounds: "#" + contentID,
      placeholder: "Type Something...",
      modules: {
        formula: true,
        toolbar: fullToolbar,
      },
      theme: "snow",
    });
  } else {
    const parentElement = document.getElementById(contentID);
    const firstChild = parentElement.children[0];

    // Alternatively, if you want to check if the first child is a form
    if (firstChild.tagName.toLowerCase() === "form") {
      Swal.fire("Error!", "Please Choose Content", "error");
      return;
    }

    if (type === "video" && isNewUpload == true) {
      Swal.fire({
        title: "Do you want to Upload New Content?",
        icon: "warning",
        showCancelButton: true,
        confirmButtonText: "Save",
        cancelButtonText: "Cancel",
        reverseButtons: true,
      }).then((result) => {
        if (result.isConfirmed) {
          if (parentElement.children.length > 0) {
            const postData = {
              level: 0,
              data: firstChild.querySelector("video").outerHTML,
            };

            fetch("/contents/" + contentValue, {
              method: "POST",
              headers: {
                "Content-Type": "application/json", // or 'application/x-www-form-urlencoded' depending on your server expectations
                // Add any other headers if needed
              },
              // Convert the postData object to JSON format if sending JSON data
              body: JSON.stringify(postData),
            })
              .then(function (response) {
                if (!response.ok) {
                  throw new Error("Network response was not ok");
                }
                return response.text();
              })
              .then(function (data) {
                // console.log(data);
                btnChange.innerHTML = 'Edit <i class="bx bx-edit"></i>';
                // Get all child elements except the first one
                var childElementsToRemove = Array.from(
                  parentElement.children
                ).slice(1);

                // Remove each child element
                childElementsToRemove.forEach(function (childElement) {
                  parentElement.removeChild(childElement);
                });

                Swal.fire("Saved!", "Your changes have been saved.", "success");
              })
              .catch(function (error) {
                Swal.fire("Error!", "Your changes Failed to saved.", "error");
                console.log("Error fetching content:", error);
              });
          }
        }
      });
    } else {
      const parts = contentID.split("-");
      const lastNumber = Number(parts[parts.length - 1]);
      const parentElement = document.getElementById(contentID);

      Swal.fire({
        title: "Do you want to save changes?",
        icon: "warning",
        showCancelButton: true,
        confirmButtonText: "Save",
        cancelButtonText: "Cancel",
        reverseButtons: true,
      }).then((result) => {
        if (result.isConfirmed) {
          btnChange.innerHTML = 'Edit <i class="bx bx-edit"></i>';
          if (parentElement.previousElementSibling) {
            const previousSibling = parentElement.previousElementSibling;
            previousSibling.parentNode.removeChild(previousSibling);
          }

          if (parentElement.children.length > 0) {
            // Step 3: Get a reference to the first child
            const firstChild = parentElement.children[0];

            Array.from(firstChild.attributes).forEach((attribute) => {
              firstChild.removeAttribute(attribute.name);
            });
            firstChild.id = contentID;
            parentElement.parentNode.replaceChild(firstChild, parentElement);
            const postData = {
              level: lastNumber,
              data: firstChild.innerHTML,
            };
            console.log(firstChild.innerHTML);

            fetch("/contents/" + contentValue, {
              method: "PATCH",
              headers: {
                "Content-Type": "application/json", // or 'application/x-www-form-urlencoded' depending on your server expectations
                // Add any other headers if needed
              },
              // Convert the postData object to JSON format if sending JSON data
              body: JSON.stringify(postData),
            })
              .then(function (response) {
                if (!response.ok) {
                  throw new Error("Network response was not ok");
                }
                return response.text();
              })
              .then(function (data) {
                console.log(data);
                Swal.fire("Saved!", "Your changes have been saved.", "success");
              })
              .catch(function (error) {
                Swal.fire("Error!", "Your changes Failed to saved.", "error");
                console.log("Error fetching content:", error);
              });
          }
        } else {
          // User clicked "Cancel" or closed the dialog
          // Swal.fire('Cancelled', 'Your changes have not been saved.', 'info');
        }
      });
    }
  }
}
function deleteContents(contentID) {
  Swal.fire({
    title: "ARE YOU SURE TO DELETE?",
    icon: "warning",
    showCancelButton: true,
    confirmButtonText: "I AM SURE",
    cancelButtonText: "Cancel",
    reverseButtons: true,
  }).then((result) => {
    if (result.isConfirmed) {
      const parts = contentID.split("-");
      const lastNumber = Number(parts[parts.length - 1]);
      const postData = {
        level: lastNumber,
      };

      fetch("/contents/" + contentValue, {
        method: "DELETE",
        headers: {
          "Content-Type": "application/json", // or 'application/x-www-form-urlencoded' depending on your server expectations
          // Add any other headers if needed
        },
        // Convert the postData object to JSON format if sending JSON data
        body: JSON.stringify(postData),
      })
        .then(function (response) {
          if (!response.ok) {
            throw new Error("Network response was not ok");
          }
          return response.text();
        })
        .then(function (data) {
          console.log(data);
          Swal.fire("Saved!", "Your content have been DELETED.", "success");
        })
        .catch(function (error) {
          Swal.fire("Error!", "Your content Failed to delete.", "error");
          console.log("Error fetching content:", error);
        });
    }
  });
}
function viewBtn() {
  document.querySelectorAll(".edit-btn").forEach(function (element) {
    element.classList.toggle("d-flex");
    element.classList.toggle("d-none");
  });
}

/**
 * @description Function to show the tooltip
 */
$(function () {
  $('[data-bs-toggle="tooltip"]').each(function () {
    const $el = $(this);
    // Check if tooltip is already initialized
    if (!$el.data("bs.tooltip")) {
      $el.tooltip();
      // $el.tooltip('show');
    }
  });
});

/**
 * @description Function to try ping the bot whatsapp
 */
async function pingBot(userId, endpoint) {
  $.ajax({
    url: endpoint,
    type: "POST",
    data: {
      userId: userId,
    },
    success: function (response) {
      // If response contains a 'message', show it using Swal.fire
      if (response.message) {
        Swal.fire({
          icon: "success",
          title: "Success",
          text: response.message,
        });
      } else {
        console.log("Ping successful:", response);
      }
    },
    error: function (error) {
      const res = error.responseJSON || {};
      const message =
        res.message || `Ping Failed: [${error.status}] ${error.responseText}`;
      const detail = Array.isArray(res.detail)
        ? res.detail.map((d) => `<div class="text-danger">${d}</div>`).join("")
        : "";

      Swal.fire({
        icon: "error",
        title: "Error",
        html: `<div>${message}</div>${detail}`,
      });
    },
  });
}

/**
 *
 * @description Function to try send text message via bot whatsapp
 */

async function sendTextBot(userId, endpoint) {
  const { value: formValues } = await Swal.fire({
    title:
      '<i class="fad fa-comment-alt-dots text-primary me-2"></i> <strong>Send WhatsApp Text Message</strong>',
    html: `
      <div class="form-check text-start mb-3">
        <input class="form-check-input" type="checkbox" id="swal-sendto-group" onchange="toggleInputMode()">
        <label class="form-check-label" for="swal-sendto-group">
          Send to <i class="fad fa-users-class me-2 ms-2"></i>  WhatsApp Group?
        </label>
      </div>
      <div class="mb-3 text-start">
        <label for="swal-input-jid" id="swal-input-label" class="form-label fw-semibold">Phone Number</label>
        <input type="text" id="swal-input-jid" class="form-control" placeholder="+6281234567890">
      </div>
      <div class="mb-2 text-start">
        <label for="swal-input-message" class="form-label fw-semibold">Message</label>
        <textarea id="swal-input-message" class="form-control" rows="4" placeholder="Type your message..."></textarea>
      </div>
      <div class="form-check text-start mt-3">
        <input class="form-check-input" type="checkbox" id="swal-use-footer">
        <label class="form-check-label" for="swal-use-footer">
          Use Footer?
        </label>
      </div>
    `,
    showCancelButton: true,
    confirmButtonText: '<i class="fa fa-paper-plane"></i> Send',
    cancelButtonText: "Cancel",
    focusConfirm: false,
    customClass: {
      confirmButton: "btn btn-success mx-2",
      cancelButton: "btn btn-danger",
    },
    didOpen: () => {
      window.toggleInputMode = () => {
        const isGroup = document.getElementById("swal-sendto-group").checked;
        const label = document.getElementById("swal-input-label");
        const input = document.getElementById("swal-input-jid");
        if (isGroup) {
          label.innerText = "Whatsapp Group JID";
          input.placeholder = "628xxxxxx-123456@g.us";
        } else {
          label.innerText = "Phone Number";
          input.placeholder = "+6281234567890";
        }
      };
    },
    preConfirm: () => {
      const isGroup = document.getElementById("swal-sendto-group").checked;
      const jidOrPhone = document.getElementById("swal-input-jid").value.trim();
      const message = document
        .getElementById("swal-input-message")
        .value.trim();
      const useFooter = document.getElementById("swal-use-footer").checked;
      if (!jidOrPhone || !message) {
        Swal.showValidationMessage(
          "Both phone number / WAG JID and message are required"
        );
        return false;
      }
      return {
        userId,
        isGroup,
        recipient: jidOrPhone,
        message,
        useFooter,
      };
    },
  });

  if (!formValues) return;

  const formData = new FormData();
  formData.append("userId", userId);
  formData.append("isGroup", formValues.isGroup);
  formData.append("recipient", formValues.recipient);
  formData.append("message", formValues.message);
  formData.append("useFooter", formValues.useFooter);

  // Show loading spinner
  Swal.fire({
    title: "Sending...",
    text: "Please wait while the text message is being sent.",
    didOpen: () => {
      Swal.showLoading();
    },
    allowOutsideClick: false,
  });

  $.ajax({
    url: endpoint,
    type: "POST",
    data: formData,
    processData: false,
    contentType: false,
    success: function (response) {
      Swal.fire({
        icon: "success",
        title: "Success",
        text: response.message || "Message sent successfully!",
      });
    },
    error: function (error) {
      const res = error.responseJSON || {};
      const message =
        res.message ||
        `Failed to send message: [${error.status}] ${error.responseText}`;
      const detail = Array.isArray(res.detail)
        ? res.detail.map((d) => `<div class="text-danger">${d}</div>`).join("")
        : "";

      Swal.fire({
        icon: "error",
        title: "Error",
        html: `<div>${message}</div>${detail}`,
      });
    },
  });
}

/**
 *
 * @description Function to try send picture via bot whatsapp
 */
async function sendImageBot(userId, endpoint) {
  const { value: formValues } = await Swal.fire({
    title:
      '<i class="fad fa-image me-2"></i> <strong>Send WhatsApp Image</strong>',
    html: `
      <div class="form-check text-start mb-3">
        <input class="form-check-input" type="checkbox" id="swal-sendto-group" onchange="toggleInputMode()">
        <label class="form-check-label" for="swal-sendto-group">
          Send to <i class="fad fa-users-class me-2 ms-2"></i>  WhatsApp Group?
        </label>
      </div>
      <div class="mb-3 text-start">
        <label for="swal-input-jid" id="swal-input-label" class="form-label fw-semibold">Phone Number</label>
        <input type="text" id="swal-input-jid" class="form-control" placeholder="+6281234567890">
      </div>
      <div class="mb-3 text-start">
        <label for="swal-input-message" class="form-label fw-semibold">Message</label>
        <textarea id="swal-input-message" class="form-control" rows="4" placeholder="Type your message..."></textarea>
      </div>
      <div class="mb-3 text-start">
        <label for="swal-input-image" class="form-label fw-semibold">Image File</label>
        <input type="file" id="swal-input-image" class="form-control" accept=".jpg,.jpeg,.png">
      </div>
      <div class="form-check text-start mt-3">
        <input class="form-check-input" type="checkbox" id="swal-use-footer">
        <label class="form-check-label" for="swal-use-footer">
          Use Footer?
        </label>
      </div>
      <div class="form-check text-start mt-3">
        <input class="form-check-input" type="checkbox" id="swal-view-once">
        <label class="form-check-label" for="swal-view-once">
          View Once?
        </label>
      </div>
    `,
    showCancelButton: true,
    confirmButtonText: '<i class="fa fa-paper-plane"></i> Send',
    cancelButtonText: "Cancel",
    focusConfirm: false,
    customClass: {
      confirmButton: "btn btn-success mx-2",
      cancelButton: "btn btn-danger",
    },
    didOpen: () => {
      window.toggleInputMode = () => {
        const isGroup = document.getElementById("swal-sendto-group").checked;
        const label = document.getElementById("swal-input-label");
        const input = document.getElementById("swal-input-jid");
        if (isGroup) {
          label.innerText = "Whatsapp Group JID";
          input.placeholder = "628xxxxxx-123456@g.us";
        } else {
          label.innerText = "Phone Number";
          input.placeholder = "+6281234567890";
        }
      };
    },
    preConfirm: () => {
      const isGroup = document.getElementById("swal-sendto-group").checked;
      const jidOrPhone = document.getElementById("swal-input-jid").value.trim();
      const message = document
        .getElementById("swal-input-message")
        .value.trim();
      const fileInput = document.getElementById("swal-input-image");
      const file = fileInput.files[0];
      const useFooter = document.getElementById("swal-use-footer").checked;
      const viewOnce = document.getElementById("swal-view-once").checked;

      if (!jidOrPhone || !file) {
        Swal.showValidationMessage(
          "Both phone number /WAG JID and image file are required"
        );
        return false;
      }

      const validTypes = ["image/jpeg", "image/png"];
      if (!validTypes.includes(file.type)) {
        Swal.showValidationMessage("Allowed image types: JPG, JPEG, PNG");
        return false;
      }

      return {
        userId,
        isGroup,
        recipient: jidOrPhone,
        message,
        file,
        useFooter,
        viewOnce,
      };
    },
  });

  if (!formValues) return;

  const formData = new FormData();
  formData.append("userId", userId);
  formData.append("isGroup", formValues.isGroup);
  formData.append("recipient", formValues.recipient);
  formData.append("message", formValues.message);
  formData.append("image", formValues.file);
  formData.append("useFooter", formValues.useFooter);
  formData.append("viewOnce", formValues.viewOnce);

  // Show loading spinner
  Swal.fire({
    title: "Sending...",
    text: "Please wait while the image is being sent.",
    didOpen: () => {
      Swal.showLoading();
    },
    allowOutsideClick: false,
  });

  $.ajax({
    url: endpoint,
    type: "POST",
    data: formData,
    processData: false,
    contentType: false,
    success: function (response) {
      Swal.fire({
        icon: "success",
        title: "Success",
        text: response.message || "Image sent successfully!",
      });
    },
    error: function (error) {
      const res = error.responseJSON || {};
      const message =
        res.message ||
        `Failed to send image: [${error.status}] ${error.responseText}`;
      const detail = Array.isArray(res.detail)
        ? res.detail.map((d) => `<div class="text-danger">${d}</div>`).join("")
        : "";

      Swal.fire({
        icon: "error",
        title: "Error",
        html: `<div>${message}</div>${detail}`,
      });
    },
  });
}

/**
 *
 * @description Function to try send document via bot whatsapp
 */
async function sendDocumentBot(userId, endpoint) {
  const { value: formValues } = await Swal.fire({
    title:
      '<i class="fad fa-file-pdf me-2"></i> <strong>Send WhatsApp Document</strong>',
    html: `
      <div class="form-check text-start mb-3">
        <input class="form-check-input" type="checkbox" id="swal-sendto-group" onchange="toggleInputMode()">
        <label class="form-check-label" for="swal-sendto-group">
          Send to <i class="fad fa-users-class me-2 ms-2"></i> WhatsApp Group?
        </label>
      </div>
      <div class="mb-3 text-start">
        <label for="swal-input-jid" id="swal-input-label" class="form-label fw-semibold">Phone Number</label>
        <input type="text" id="swal-input-jid" class="form-control" placeholder="+6281234567890">
      </div>
      <div class="mb-3 text-start">
        <label for="swal-input-message" class="form-label fw-semibold">Message</label>
        <textarea id="swal-input-message" class="form-control" rows="4" placeholder="Type your message..."></textarea>
      </div>
      <div class="mb-3 text-start">
        <label for="swal-input-document" class="form-label fw-semibold">Document</label>
        <input type="file" id="swal-input-document" class="form-control">
      </div>
      <div class="form-check text-start mt-3">
        <input class="form-check-input" type="checkbox" id="swal-use-footer">
        <label class="form-check-label" for="swal-use-footer">
          Use Footer?
        </label>
      </div>
    `,
    showCancelButton: true,
    confirmButtonText: '<i class="fa fa-paper-plane"></i> Send',
    cancelButtonText: "Cancel",
    focusConfirm: false,
    customClass: {
      confirmButton: "btn btn-success mx-2",
      cancelButton: "btn btn-danger",
    },
    didOpen: () => {
      window.toggleInputMode = () => {
        const isGroup = document.getElementById("swal-sendto-group").checked;
        const label = document.getElementById("swal-input-label");
        const input = document.getElementById("swal-input-jid");
        if (isGroup) {
          label.innerText = "Whatsapp Group JID";
          input.placeholder = "628xxxxxx-123456@g.us";
        } else {
          label.innerText = "Phone Number";
          input.placeholder = "+6281234567890";
        }
      };
    },
    preConfirm: () => {
      const isGroup = document.getElementById("swal-sendto-group").checked;
      const jidOrPhone = document.getElementById("swal-input-jid").value.trim();
      const message = document
        .getElementById("swal-input-message")
        .value.trim();
      const fileInput = document.getElementById("swal-input-document");
      const file = fileInput.files[0];
      const useFooter = document.getElementById("swal-use-footer").checked;

      if (!jidOrPhone || !file) {
        Swal.showValidationMessage(
          "Both phone number / WAG JID and document file are required"
        );
        return false;
      }

      const validTypes = [
        "application/pdf",
        "application/msword",
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
        "text/plain",
      ];

      if (!validTypes.includes(file.type)) {
        Swal.showValidationMessage(
          "Allowed document types: PDF, DOC, DOCX, TXT"
        );
        return false;
      }

      return {
        userId,
        isGroup,
        recipient: jidOrPhone,
        message,
        file,
        useFooter,
      };
    },
  });

  if (!formValues) return;

  const formData = new FormData();
  formData.append("userId", userId);
  formData.append("isGroup", formValues.isGroup);
  formData.append("recipient", formValues.recipient);
  formData.append("message", formValues.message);
  formData.append("document", formValues.file);
  formData.append("useFooter", formValues.useFooter);

  // Show loading spinner
  Swal.fire({
    title: "Sending...",
    text: "Please wait while the document is being sent.",
    didOpen: () => {
      Swal.showLoading();
    },
    allowOutsideClick: false,
  });

  $.ajax({
    url: endpoint,
    type: "POST",
    data: formData,
    processData: false,
    contentType: false,
    success: function (response) {
      Swal.fire({
        icon: "success",
        title: "Success",
        text: response.message || "Document sent successfully!",
      });
    },
    error: function (error) {
      const res = error.responseJSON || {};
      const message =
        res.message ||
        `Failed to send document: [${error.status}] ${error.responseText}`;
      const detail = Array.isArray(res.detail)
        ? res.detail.map((d) => `<div class="text-danger">${d}</div>`).join("")
        : typeof res.detail === "string"
          ? `<div class="text-danger">${res.detail}</div>`
          : "";

      Swal.fire({
        icon: "error",
        title: "Error",
        html: `<div>${message}</div>${detail}`,
      });
    },
  });
}

/**
 *
 * @description Function to try send location via bot whatsapp
 */
async function sendLocationBot(userId, endpoint) {
  const { value: formValues } = await Swal.fire({
    title:
      '<i class="fad fa-map-marker-alt me-2"></i> <strong>Send WhatsApp <br>Location</strong>',
    html: `
    <div class="form-check text-start mb-3">
      <input class="form-check-input" type="checkbox" id="swal-sendto-group" onchange="toggleInputMode()">
      <label class="form-check-label" for="swal-sendto-group">
        Send to <i class="fad fa-users-class me-2 ms-2"></i>  WhatsApp Group?
      </label>
    </div>
    <div class="mb-3 text-start">
      <label for="swal-input-jid" id="swal-input-label" class="form-label fw-semibold">Phone Number</label>
      <input type="text" id="swal-input-jid" class="form-control" placeholder="+6281234567890">
    </div>
    <div class="mb-3 text-start">
      <label class="form-label fw-semibold">Pick Location on Map</label>
      <div id="map" style="height: 300px; border-radius: 8px;"></div>
    </div>
    <div class="row">
      <div class="col">
        <label for="swal-input-lat" class="form-label fw-semibold">Latitude</label>
        <input type="text" id="swal-input-lat" class="form-control" placeholder="Latitude" readonly>
      </div>
      <div class="col">
        <label for="swal-input-lng" class="form-label fw-semibold">Longitude</label>
        <input type="text" id="swal-input-lng" class="form-control" placeholder="Longitude" readonly>
      </div>
    </div>
    <div class="mt-3 text-start">
      <label for="swal-input-location-name" class="form-label fw-semibold">Location Name</label>
      <input type="text" id="swal-input-location-name" class="form-control" placeholder="e.g. Rawamangun's Office">
    </div>
    <div class="mb-3 text-start">
      <label for="swal-input-location-address" class="form-label fw-semibold">Detail Address</label>
      <textarea id="swal-input-location-address" class="form-control" rows="3" placeholder="e.g. Jalan raya No. 99 RT/RW: 01/100"></textarea>
    </div>
    <div class="form-check text-start mt-3">
      <input class="form-check-input" type="checkbox" id="swal-live-location">
      <label class="form-check-label" for="swal-live-location">
        Send as <i class="fad fa-location me-2 ms-2"></i> Live Location?
      </label>
    </div>
  `,
    didOpen: () => {
      const mapID = "map"; // Make sure your HTML has a <div id="map"></div>
      const firstLocation = [-6.175392, 106.827153]; // Monas Jakarta
      const zoomView = 16;

      // Initialize the map
      const map = L.map(mapID, {
        center: firstLocation,
        zoom: zoomView,
        gestureHandling: false,
      });

      // Add OpenStreetMap tile layer
      L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
        attributionControl: false,
      }).addTo(map);

      // Add distance scale
      L.control
        .scale({
          position: "bottomright",
          metric: true,
          imperial: false,
        })
        .addTo(map);

      // Add fullscreen control (from plugin)
      L.control
        .fullscreen({
          pseudoFullscreen: true, // uses CSS to fullscreen the map container only
        })
        .addTo(map);

      // Click to place marker and update lat/lng inputs
      let marker;
      map.on("click", function (e) {
        const { lat, lng } = e.latlng;

        // Assuming you have these input fields in your HTML
        document.getElementById("swal-input-lat").value = lat.toFixed(8);
        document.getElementById("swal-input-lng").value = lng.toFixed(8);

        if (marker) {
          marker.setLatLng(e.latlng);
        } else {
          marker = L.marker(e.latlng).addTo(map);
        }
      });

      // Search location
      L.Control.geocoder({
        defaultMarkGeocode: false,
        placeholder: "Search location...",
      })
        .on("markgeocode", function (e) {
          const latlng = e.geocode.center;
          map.setView(latlng, zoomView);

          // Update inputs
          document.getElementById("swal-input-lat").value =
            latlng.lat.toFixed(8);
          document.getElementById("swal-input-lng").value =
            latlng.lng.toFixed(8);

          // Add or move marker
          if (marker) {
            marker.setLatLng(latlng);
          } else {
            marker = L.marker(latlng).addTo(map);
          }
        })
        .addTo(map);

      document.querySelector(".leaflet-control-attribution")?.remove();

      window.toggleInputMode = () => {
        const isGroup = document.getElementById("swal-sendto-group").checked;
        const label = document.getElementById("swal-input-label");
        const input = document.getElementById("swal-input-jid");
        if (isGroup) {
          label.innerText = "Whatsapp Group JID";
          input.placeholder = "628xxxxxx-123456@g.us";
        } else {
          label.innerText = "Phone Number";
          input.placeholder = "+6281234567890";
        }
      };
    },
    showCancelButton: true,
    confirmButtonText: '<i class="fa fa-paper-plane"></i> Send',
    cancelButtonText: "Cancel",
    focusConfirm: false,
    customClass: {
      confirmButton: "btn btn-success mx-2",
      cancelButton: "btn btn-danger",
    },
    preConfirm: () => {
      const isGroup = document.getElementById("swal-sendto-group").checked;
      const jidOrPhone = document.getElementById("swal-input-jid").value.trim();
      const long = document.getElementById("swal-input-lng").value.trim();
      const lat = document.getElementById("swal-input-lat").value.trim();
      const locName = document
        .getElementById("swal-input-location-name")
        .value.trim();
      const locAddress = document
        .getElementById("swal-input-location-address")
        .value.trim();
      const isLive = document.getElementById("swal-live-location").checked;

      if (!jidOrPhone || !long || !lat) {
        Swal.showValidationMessage(
          "Both phone number / WAG JID, longitude & latitude are required!"
        );
        return false;
      }

      return {
        userId,
        isGroup,
        recipient: jidOrPhone,
        long,
        lat,
        locName,
        locAddress,
        isLive,
      };
    },
  });

  if (!formValues) return;

  const formData = new FormData();
  formData.append("userId", userId);
  formData.append("isGroup", formValues.isGroup);
  formData.append("recipient", formValues.recipient);
  formData.append("long", formValues.long);
  formData.append("lat", formValues.lat);
  formData.append("locName", formValues.locName);
  formData.append("locAddress", formValues.locAddress);
  formData.append("isLive", formValues.isLive);

  // Show loading spinner
  Swal.fire({
    title: "Sending...",
    text: "Please wait while the location is being sent.",
    didOpen: () => {
      Swal.showLoading();
    },
    allowOutsideClick: false,
  });

  $.ajax({
    url: endpoint,
    type: "POST",
    data: formData,
    processData: false,
    contentType: false,
    success: function (response) {
      Swal.fire({
        icon: "success",
        title: "Success",
        text: response.message || "Location sent successfully!",
      });
    },
    error: function (error) {
      const res = error.responseJSON || {};
      const message = res.message || "Failed to send location";
      const detail = Array.isArray(res.detail)
        ? res.detail.map((d) => `<div class="text-danger">${d}</div>`).join("")
        : typeof res.detail === "string"
          ? `<div class="text-danger">${res.detail}</div>`
          : "";

      Swal.fire({
        icon: "error",
        title: "Error",
        html: `<div>${message}</div>${detail}`,
      });
    },
  });
}

/**
 *
 * @description Function to try send polling to group or phone number via bot whatsapp
 */
async function sendPollingBot(userId, endpoint) {
  let optionCounter = 1;

  const { value: formValues } = await Swal.fire({
    title:
      '<i class="fad fa-poll-h me-2"></i> <strong>Send WhatsApp Poll</strong>',
    html: `
      <div class="form-check text-start mb-3">
        <input class="form-check-input" type="checkbox" id="swal-sendto-group" onchange="toggleInputMode()">
        <label class="form-check-label" for="swal-sendto-group">
          Send to <i class="fad fa-users-class me-2 ms-2"></i>  WhatsApp Group?
        </label>
      </div>
      <div class="mb-3 text-start">
        <label for="swal-input-jid" id="swal-input-label" class="form-label fw-semibold">Phone Number</label>
        <input type="text" id="swal-input-jid" class="form-control" placeholder="+6281234567890">
      </div>
      <div class="mb-3 text-start">
        <label for="swal-input-question" class="form-label fw-semibold">Question</label>
        <textarea id="swal-input-question" class="form-control" rows="4" placeholder="e.g. Which one is better use A, B or C ?"></textarea>
      </div>
      <div class="form-check text-start mb-3">
        <input class="form-check-input" type="checkbox" id="swal-only-one">
        <label class="form-check-label" for="swal-only-one">
          Only 1 selection allowed?
        </label>
      </div>
      <div id="poll-option-wrapper">
        <label class="form-label fw-semibold">Poll Options</label>
        <div class="input-group mb-2">
          <input type="text" class="form-control poll-option-input" placeholder="Option 1">
          <button class="btn btn-outline-primary" type="button" onclick="addPollOption()">+</button>
        </div>
      </div>
    `,
    didOpen: () => {
      window.addPollOption = () => {
        optionCounter++;
        const inputGroup = document.createElement("div");
        inputGroup.className = "input-group mb-2";
        inputGroup.innerHTML = `
          <input type="text" class="form-control poll-option-input" placeholder="Option ${optionCounter}">
          <button class="btn btn-outline-danger" type="button" onclick="this.parentElement.remove()">×</button>
        `;
        document.getElementById("poll-option-wrapper").appendChild(inputGroup);
      };

      window.toggleInputMode = () => {
        const isGroup = document.getElementById("swal-sendto-group").checked;
        const label = document.getElementById("swal-input-label");
        const input = document.getElementById("swal-input-jid");
        if (isGroup) {
          label.innerText = "Whatsapp Group JID";
          input.placeholder = "628xxxxxx-123456@g.us";
        } else {
          label.innerText = "Phone Number";
          input.placeholder = "+6281234567890";
        }
      };
    },
    showCancelButton: true,
    confirmButtonText: '<i class="fa fa-paper-plane"></i> Send',
    cancelButtonText: "Cancel",
    focusConfirm: false,
    customClass: {
      confirmButton: "btn btn-success mx-2",
      cancelButton: "btn btn-danger",
    },
    preConfirm: () => {
      const isGroup = document.getElementById("swal-sendto-group").checked;
      const jidOrPhone = document.getElementById("swal-input-jid").value.trim();
      const onlyOne = document.getElementById("swal-only-one").checked;
      const question = document
        .getElementById("swal-input-question")
        .value.trim();

      if (!jidOrPhone || !question) {
        Swal.showValidationMessage(
          "Please enter a Whatsapp Group JID or Phone Number also the polling question"
        );
        return false;
      }

      const options = Array.from(
        document.querySelectorAll(".poll-option-input")
      )
        .map((input) => input.value.trim())
        .filter((val) => val !== "");

      if (options.length < 2) {
        Swal.showValidationMessage("Please enter at least 2 poll options");
        return false;
      }

      return {
        userId,
        isGroup,
        recipient: jidOrPhone,
        question,
        onlyOneSelection: onlyOne,
        options,
      };
    },
  });

  if (!formValues) return;

  const formData = new FormData();
  formData.append("userId", userId);
  formData.append("isGroup", formValues.isGroup);
  formData.append("recipient", formValues.recipient);
  formData.append("question", formValues.question);
  formData.append("onlyOneSelection", formValues.onlyOneSelection);
  formData.append("options", JSON.stringify(formValues.options));

  // Show loading spinner
  Swal.fire({
    title: "Sending...",
    text: "Please wait while the polling is being sent.",
    didOpen: () => {
      Swal.showLoading();
    },
    allowOutsideClick: false,
  });

  $.ajax({
    url: endpoint,
    type: "POST",
    data: formData,
    processData: false,
    contentType: false,
    success: function (response) {
      Swal.fire({
        icon: "success",
        title: "Success",
        text: response.message || "Polling sent successfully!",
      });
    },
    error: function (error) {
      const res = error.responseJSON || {};
      const message =
        res.message ||
        `Failed to send polling: [${error.status}] ${error.responseText}`;
      const detail = Array.isArray(res.detail)
        ? res.detail.map((d) => `<div class="text-danger">${d}</div>`).join("")
        : typeof res.detail === "string"
          ? `<div class="text-danger">${res.detail}</div>`
          : "";

      Swal.fire({
        icon: "error",
        title: "Error",
        html: `<div>${message}</div>${detail}`,
      });
    },
  });
}

function GetLastUpdateData(endpoint, classContainer) {
  $.ajax({
    url: endpoint,
    type: "GET",
    dataType: "json",
    success: function (res) {
      if (res.lastUpdated) {
        // Update the content inside the element
        $(`.${classContainer}`).html(
          `Last Update: <strong>${res.lastUpdated}</strong>`
        );
      } else {
        // Show a SweetAlert if no timestamp was returned
        Swal.fire({
          icon: "info",
          title: "No Data",
          text: res.message || "No last‑update timestamp returned.",
        });
      }
    },
    error: function (xhr, status, error) {
      // Show a SweetAlert on AJAX error
      Swal.fire({
        icon: "error",
        title: "Request Failed",
        text: `Failed to fetch last update: ${xhr.status} ${xhr.statusText}`,
      });
    },
  });
}

async function refreshData(endpoint, classContainer, tableClass, btnClass) {
  const $btn = $("." + btnClass);

  Swal.fire({
    title: "Are you sure?",
    text: `Do you really want to refresh the data ${tableClass} ?`,
    icon: "warning",
    showCancelButton: true,
    confirmButtonText: "Yes, refresh it!",
    cancelButtonText: "Cancel",
    reverseButtons: true,
  }).then((result) => {
    if (result.isConfirmed) {
      $btn
        .prop("disabled", true)
        .removeClass("btn-info")
        .addClass("btn-label-secondary");

      $.ajax({
        url: endpoint,
        type: "GET",
        dataType: "json",
        success: function (response) {
          Swal.fire({
            icon: "success",
            title: "Success!",
            text: response.message || "Data refreshed successfully!",
          });

          // Reload DataTable if it exists
          if ($.fn.DataTable.isDataTable("." + tableClass)) {
            $("." + tableClass)
              .DataTable()
              .ajax.reload(null, false);
          }

          const lastUpdateEndpoint = endpoint.replace("refresh", "last_update");
          GetLastUpdateData(lastUpdateEndpoint, classContainer);
        },
        error: function (xhr, status, error) {
          let errorMsg = "Something went wrong.";
          if (xhr.responseJSON) {
            if (xhr.responseJSON.message) {
              errorMsg = xhr.responseJSON.message;
            } else if (xhr.responseJSON.error) {
              errorMsg = xhr.responseJSON.error;
            }
          }

          Swal.fire({
            icon: "error",
            title: "Error!",
            text: errorMsg,
          });
        },
        complete: function () {
          $btn
            .prop("disabled", false)
            .removeClass("btn-label-secondary")
            .addClass("btn-info");
        },
      });
    }
  });
}

/**
 *
 * @description Function to show the whatsapp group owner avatart
 */
function showAvatarModal(avatarUrl) {
  // Remove existing modal if any
  const oldModal = document.getElementById("dynamicAvatarModal");
  if (oldModal) oldModal.remove();

  // Create modal container
  const modalDiv = document.createElement("div");
  modalDiv.className = "modal fade show";
  modalDiv.id = "dynamicAvatarModal";
  modalDiv.style.display = "block";
  modalDiv.tabIndex = -1;
  modalDiv.setAttribute("aria-hidden", "false");

  // Modal dialog and content
  modalDiv.innerHTML = `
    <div class="modal-dialog modal-dialog-centered modal-sm">
      <div class="modal-content bg-transparent border-0">
        <div class="modal-body p-0">
          <img src="${avatarUrl}" alt="Full Avatar" style="width: 100%; border-radius: 8px;">
        </div>
      </div>
    </div>
  `;

  // Add modal to body
  document.body.appendChild(modalDiv);

  // Close modal on backdrop click or ESC
  function closeModal() {
    modalDiv.classList.remove("show");
    modalDiv.style.display = "none";
    modalDiv.remove();
    document.removeEventListener("keydown", escListener);
  }
  modalDiv.addEventListener("click", (e) => {
    if (e.target === modalDiv) closeModal();
  });
  function escListener(e) {
    if (e.key === "Escape") closeModal();
  }
  document.addEventListener("keydown", escListener);
}

/**
 * @description Function to show photos of data in merchant fast link & KresekBag
 */
function openPopupPhotosMerchantFastLink(id, keterangan) {
  window.open(
    "/photos/merchant_fastlink/" + id,
    "popupWindow",
    "width=400,height=700,top=20,right=20,left=" +
    (window.screen.width - 420) +
    ",scrollbars=yes,resizable=yes"
  );
}

function openPopupPhotosMerchantKresekBag(id) {
  let endpoint = $("[data-endpoint-photos-merchant-kresekbag]").data(
    "endpoint-photos-merchant-kresekbag"
  );

  window.open(
    endpoint + "/" + id,
    "popupWindow",
    "width=400,height=700,top=20,right=20,left=" +
    (window.screen.width - 420) +
    ",scrollbars=yes,resizable=yes"
  );
}

function resetQuotaWhatsappPrompt(id) {
  let endpoint = $("#data-wa-user-management").data(
    "reset-quota-endpoint"
  );
  Swal.fire({
    title: "Are you sure?",
    text: "Do you want to reset the quota?",
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, reset it!",
  }).then((result) => {
    if (result.isConfirmed) {
      $.ajax({
        url: endpoint,
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({ id: id }),
        success: function (response) {
          if (response.success) {
            Swal.fire(
              "Reset!",
              "Quota was reset successfully.",
              "success"
            ).then(() => {
              $('.dt_whatsapp_user_management').DataTable().ajax.reload(null, false);
            });
          } else {
            Swal.fire(
              "Failed!",
              response.message || "Unknown error occurred.",
              "error"
            );
          }
        },
        error: function (xhr, status, error) {
          console.error("Error:", error);
          Swal.fire("Error!", "An unexpected error occurred.", "error");
        },
      });
    }
  });
}

function unbanUser(id) {
  let endpoint = $("#data-wa-user-management").data(
    "unban-user-endpoint"
  );
  Swal.fire({
    title: "Are you sure?",
    text: "Do you want to unban this user?",
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, unban user"
  }).then((result) => {
    if (result.isConfirmed) {
      $.ajax({
        url: endpoint,
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({ id: id }),
        success: function (response) {
          if (response.success) {
            Swal.fire(
              "Unlocked!",
              "User activated again.",
              "success"
            ).then(() => {
              $('.dt_whatsapp_user_management').DataTable().ajax.reload(null, false);
            });
          } else {
            Swal.fire(
              "Failed!",
              response.message || "Unknown error occurred.",
              "error"
            );
          }
        },
        error: function (xhr, status, error) {
          console.error("Error:", error);
          Swal.fire("Error!", "An unexpected error occurred.", "error");
        },
      });
    }
  });
}
