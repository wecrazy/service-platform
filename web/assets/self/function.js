const now = new Date();
const year = now.getFullYear();
const month = String(now.getMonth() + 1).padStart(2, "0");
const day = String(now.getDate()).padStart(2, "0");
const hours = String(now.getHours()).padStart(2, "0");
const minutes = String(now.getMinutes()).padStart(2, "0");
const seconds = String(now.getSeconds()).padStart(2, "0");

const timezoneOffset = -now.getTimezoneOffset() / 60;
const timezone = `GMT${timezoneOffset >= 0 ? "+" : ""}${timezoneOffset}`;
const timestampnow = `${year}_${month}_${day}__${hours}_${minutes}_${seconds}_${timezone}`;

//RANDOM STRING
function generateRandomString(length) {
  const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";
  let result = "";
  for (let i = 0; i < length; i++) {
    result += characters.charAt(Math.floor(Math.random() * characters.length));
  }
  return result;
}

//ACCEPT User Web Admin
function handleAccept(id) {
  $("#user-admin-approval-id").val(id);

  // Get form values
  var userId = $("#user-admin-approval-id").val();
  var formData = {
    userId: userId,
  };
  // Display a confirmation dialog
  Swal.fire({
    title: "Are you sure?",
    text: "Do you want to accept this user?",
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, accept it!",
    cancelButtonText: "Cancel",
  }).then((result) => {
    if (result.isConfirmed) {
      // User confirmed, proceed with AJAX request
      $.ajax({
        type: "POST",
        url: webPath + "tab-merchant-approval/approval/accept",
        data: formData,
        success: function (data) {
          console.log("Success:", data);

          // Reload DataTable
          $(".datatables-approval").DataTable().ajax.reload();
          // Update Info Value
          getApprovalInfoValue();
          // Show SweetAlert success alert
          Swal.fire({
            title: "Success!",
            text: "Record updated successfully",
            icon: "success",
          });
        },
        error: function (error) {
          console.error("Error:", error);

          // Show SweetAlert error alert
          Swal.fire({
            title: "Error!",
            text: "Failed to update record",
            icon: "error",
          });
        },
      });
    }
  });
}
function formatRupiah(amount) {
  let str = amount.toString();
  let remainder = str.length % 3;
  let rupiah = str.substr(0, remainder);
  let thousands = str.substr(remainder).match(/\d{3}/g);

  if (thousands) {
    let separator = remainder ? "." : "";
    rupiah += separator + thousands.join(" ");
  }
  return rupiah;
}
function formatDate_DD_MM_YYYY(date) {
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const year = date.getFullYear();
  return `${day}/${month}/${year}`;
}
// function webUserApproval() {
//     // Get form values
//     var userId = $("#user-admin-approval-id").val();
//     var role = $("#add-approval-role").val();
//     var corporate = $("#add-approval-corporate").val();
//     var formData = {
//       userId: userId,
//       rolename: role,
//       corporate: corporate
//     };

//     $.ajax({
//       type: 'POST',
//       url: webPath + 'tab-merchant-approval/approval/accept',
//       data: formData,
//       success: function(data) {
//         console.log('Success:', data);

//         // Reload DataTable
//         $('.datatables-approval').DataTable().ajax.reload();

//         // Dismiss modal
//         $('#addApproval').modal('hide');
//         //Update Info Value
//         getApprovalInfoValue()
//         // Show SweetAlert success alert
//         Swal.fire({
//           title: 'Success!',
//           text: 'Record updated successfully',
//           icon: 'success',
//         });
//       },
//       error: function(error) {
//         console.error('Error:', error);

//         // Show SweetAlert error alert
//         Swal.fire({
//           title: 'Error!',
//           text: 'Failed to update record',
//           icon: 'error',
//         });
//       }
//     });
// }

//REJECT User Web Admin
function handleDeny(id) {
  // Display a confirmation dialog
  Swal.fire({
    title: "Are you sure?",
    text: "Do you want to deny this user?",
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, deny it!",
    cancelButtonText: "Cancel",
  }).then((result) => {
    if (result.isConfirmed) {
      // User confirmed, proceed with AJAX request
      $.ajax({
        type: "GET",
        url: webPath + "tab-merchant-approval/approval/reject?id=" + id,
        success: function (response) {
          window.location.reload();
          console.log(response);
        },
        error: function (error) {
          console.log(error);
          alert(error.responseJSON["error"]);
          window.location.reload();
        },
      });
    }
  });
}
//Register User
function postRegister() {
  // Add your logic here
  var formData = {
    multiStepsEmail: $("#multiStepsEmail").val(),
    multiStepsUsername: $("#multiStepsUsername").val(),
    multiStepsPass: $("#multiStepsPass").val(),
    multiStepsFullName: $("#multiStepsFullName").val(),
    multiStepsMobile: $("#multiStepsMobile").val(),
    multiStepsCorporate: $("#multiStepsCorporate").val(),
    multiStepsAddress: $("#multiStepsAddress").val(),
  };
  $.ajax({
    type: "POST",
    url: "/register",
    data: formData,
    success: function (response) {
      // if (response.status == "01"){
      window.location.href = "/login";
      // }
      // window.location.reload();
      console.log(response);
    },
    error: function (error) {
      // if(error.status == 404){
      //     alert("User Invalid");
      // }else if (error.status == 400){
      //     alert("Please Fill The Form");
      // }
      console.log(error);
      alert(error.responseJSON["error"]);
      window.location.reload();
    },
  });
}
//Function Vehicle
function getUserInfoDepositValue() {
  $.ajax({
    type: "GET",
    url: webPath + "admin/member/card",
    contentType: "application/json; charset=utf-8",
    dataType: "json",
    // data: JSON.stringify(formData),
    success: function (response) {
      var current_deposit = response.data.current_deposit.toLocaleString(
        "id-ID",
        { style: "currency", currency: "IDR" }
      );
      var shared_deposit = response.data.shared_deposit.toLocaleString(
        "id-ID",
        { style: "currency", currency: "IDR" }
      );
      var used_deposit = response.data.used_deposit.toLocaleString("id-ID", {
        style: "currency",
        currency: "IDR",
      });
      console.log("Data sent successfully:", response);
      $(".current-deposit").html(`${current_deposit}`);
      $(".shared-deposit").html(`${shared_deposit}`);
      $(".used-deposit").html(`${used_deposit}`);
    },
    error: function (error) {
      console.log(error);
      console.error("Error sending data:", error);
    },
  });
}
let vehicleTotalNumber = 0;
let motorcycleTotal = 0;
let carTotal = 0;
let otherTotal = 0;

// function getVehicleInfoValue (){
//     $.ajax({
//         type: 'GET',
//         url: webPath + 'tab-dashboards/vehicles',
//         contentType: 'application/json; charset=utf-8',
//         dataType: 'json',
//         success: function (response) {
//             console.log('Data sent successfully:', response);
//             $('#total-vehicles').html(`Total Vehicles: ${response.data.total}`);
//             $('#total-cars').html(`Total Cars: ${response.data.car}`);
//             $('#total-motorcycles').html(`Total Motorcycles: ${response.data.motorcycle}`);
//             vehicleTotalNumber = response.data.total;
//             console.log(vehicleTotalNumber)
//             motorcycleTotal = response.data.motorcycle;
//             carTotal = response.data.car;
//             otherTotal = 0;
//             $("#vehicleIconValue").addClass("bx bxs-star");
//             $("#cardOpt4Vehicle").addClass("avatar-initial rounded bg-label-primary");
//             $("#vehicleTotalValue").html(vehicleTotalNumber);
//         },
//         error: function (error) {
//           console.log(error)
//           console.error('Error sending data:', error);
//         }
//     });
// }
// function vehicleTotal(vehicleName){
//     // console.log(vehicleName)
//     $("#vehicleIconValue").removeClass();
//     $("#cardOpt4Vehicle").removeClass();
//     switch(vehicleName) {
//         case "Motorcycle":
//             $("#vehicleIconValue").addClass("fa-solid fa-motorcycle");
//             $("#cardOpt4Vehicle").addClass("avatar-initial rounded bg-label-warning");
//             $("#vehicleTotalValue").html(motorcycleTotal)
//           break;
//         case "Car":
//             $("#vehicleIconValue").addClass("bx bxs-car-garage");
//             $("#cardOpt4Vehicle").addClass("avatar-initial rounded bg-label-danger");
//             $("#vehicleTotalValue").html(carTotal)
//           break;
//         case "Bus":
//             $("#vehicleIconValue").addClass("bx bx-bus");
//             $("#cardOpt4Vehicle").addClass("avatar-initial rounded bg-label-info");
//             $("#vehicleTotalValue").html(0)
//           break;
//         case "Truck":
//             $("#vehicleIconValue").addClass("bx bxs-truck");
//             $("#cardOpt4Vehicle").addClass("avatar-initial rounded bg-label-success");
//             $("#vehicleTotalValue").html(0)
//           break;
//         default:
//             $("#vehicleIconValue").addClass("bx bxs-star");
//             $("#cardOpt4Vehicle").addClass("avatar-initial rounded bg-label-primary");
//             $("#vehicleTotalValue").html(vehicleTotalNumber)
//       }

//     $("#vehicleNameValue").html(vehicleName)

// }

//Function Approval
function getApprovalInfoValue() {
  $.ajax({
    type: "GET",
    url: webPath + "tab-merchant-approval/approval/show",
    contentType: "application/json; charset=utf-8",
    dataType: "json",
    success: function (response) {
      console.log("Data approval successfully:", response);
      $("#total-request").html(`${response.data.waiting}`);
      $("#total-approved").html(` ${response.data.approved}`);
      $("#total-denied").html(`${response.data.denied}`);
    },
    error: function (error) {
      console.log(error);
      console.error("Error sending data:", error);
    },
  });
}

//Function Member
function addNewMemberForm() {
  var formData = {
    userFullname: $("#add-member-fullname").val(),
    username: $("#add-member-username").val(),
    userEmail: $("#add-member-email").val(),
    userPassword: $("#add-member-password").val(),
    userPhone: $("#add-member-phone").val(),
    companyName: $("#add-member-company").val(),
  };
  $.ajax({
    type: "POST",
    url: webPath + "admin/member/new",
    contentType: "application/json; charset=utf-8",
    dataType: "json",
    data: JSON.stringify(formData),
    success: function (response) {
      console.log("Data sent successfully:", response);
      // Optionally, handle the response from the server
      // location.reload();
      $(".datatables-memberIRF").DataTable().ajax.reload();
      // Assuming you are using Bootstrap 4
      $("#addMember").modal("hide");
      console.log("12333");
      // getUserwarningbg-label-warningValue ();
    },
    error: function (error) {
      console.log(error);
      console.error("Error sending data:", error);
    },
  });
}
function deleteUserMember(id) {
  Swal.fire({
    title: "Are you sure?",
    text: "You won't be able to revert this!",
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, delete it!",
  }).then((result) => {
    if (result.isConfirmed) {
      // User confirmed, proceed with the delete action
      $.ajax({
        type: "DELETE",
        url: webPath + "admin/member?id=" + id,
        contentType: "application/json; charset=utf-8",
        dataType: "json",
        success: function (response) {
          // Show success message
          Swal.fire("Deleted!", "User has been deleted.", "success");
          // Reload DataTable
          $(".datatables-memberIRF").DataTable().ajax.reload();
          // location.reload();
          getUserInfoValue();
        },
        error: function (error) {
          // Show error message
          Swal.fire(
            "Error",
            "User not deleted. Something went wrong.",
            "error"
          );
          console.error("Error sending data:", error);
        },
      });
    }
  });
}
function memberEdit(id) {
  $("#memberid").val(id);
}
function removeMemberBalance() {
  var memberid = $("#memberid").val();
  var depositValue = $("#nameEx7depositMember").val();

  // Display Sweet Alert confirmation
  Swal.fire({
    title: "Are you sure?",
    text: "You are about to remove this member. This action cannot be undone.",
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#d33",
    cancelButtonColor: "#3085d6",
    confirmButtonText: "Yes, remove it!",
  }).then((result) => {
    if (result.isConfirmed) {
      // User confirmed, proceed with removal
      var formData = {
        memberid: memberid,
        depositValue: depositValue,
      };

      $.ajax({
        type: "POST",
        url: "/web/admin/member/remove",
        data: formData,
        success: function (response) {
          console.log(response);
          window.location.reload();
          // Additional logic or UI changes on success
        },
        error: function (error) {
          console.log(error);
          console.log(error.responseJSON["error"]);
          alert(error.responseJSON["error"]);
          // Additional error handling logic
        },
      });
    }
  });
}

//Function Corporate
function addNewCorporateForm() {
  var formData = {
    corporateName: $("#add-corporate-name").val(),
    corporateAdress: $("#add-corporate-address").val(),
  };
  $.ajax({
    type: "POST",
    url: webPath + "admin/corporate/add",
    contentType: "application/json; charset=utf-8",
    dataType: "json",
    data: JSON.stringify(formData),
    success: function (response) {
      console.log("Data sent successfully:", response);
      // location.reload();
      $(".datatables-corporate").DataTable().ajax.reload();
      $("#addCorporate").modal("hide");
      console.log("12333");
      // getUserwarningbg-label-warningValue ();
    },
    error: function (error) {
      console.log(error);
      console.error("Error sending data:", error);
    },
  });
}
function corporateEdit(id, name, deposit) {
  $("#corpid").val(id);
  $("#corporate_name").html(name);
  $("#corporate_deposit").html("Current Deposit IDR " + deposit);
}
function deleteCorporate(id, name) {
  Swal.fire({
    title: `Are you sure want to DELETE ${name} \nand its all User?`,
    text: "You won't be able to revert this!",
    html: '<input type="password" id="password" class="swal2-input" placeholder="Enter your password">',
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, delete it!",
    customClass: {
      popup: "border border-danger border-5", // Add your custom classes here
      confirmButton: "btn btn-danger", // Add button-specific classes here
      cancelButton: "btn btn-primary", // Add button-specific classes here
    },
  }).then((result) => {
    if (result.isConfirmed) {
      // User confirmed, proceed with the delete action
      var password = $("#password").val(); // Get the entered password

      // You can add additional validation logic for the password if needed

      $.ajax({
        type: "DELETE",
        url: webPath + "admin/corporate?id=" + id + "&password=" + password,
        contentType: "application/json; charset=utf-8",
        dataType: "json",
        success: function (response) {
          // Show success message
          Swal.fire("Deleted!", "User has been deleted.", "success");
          // Reload DataTable
          $(".datatables-corporate").DataTable().ajax.reload();
          // location.reload();
          getUserInfoValue();
        },
        error: function (error) {
          // Show error message
          Swal.fire(
            "Error",
            "User not deleted. Something went wrong.",
            "error"
          );
          console.error("Error sending data:", error);
        },
      });
    }
  });
}

//Function web user list
function updateNewUserForm() {
  var formData = {
    id: $("#user-status-update-id").val(),
    status: $("#user-status-update").val(),
  };

  $.ajax({
    type: "PATCH",
    url: webPath + "tab-roles/admin/status",
    contentType: "application/json; charset=utf-8",
    dataType: "json",
    data: JSON.stringify(formData),
    success: function (response) {
      // Display SweetAlert success message
      Swal.fire({
        icon: "success",
        title: "Success",
        text: "User status updated successfully!",
      });

      // Reload DataTable and other actions
      $(".datatables-usersIRF").DataTable().ajax.reload();
      getUserInfoValue();
    },
    error: function (error) {
      console.log(error);
      console.error("Error sending data:", error);

      // Display SweetAlert error message
      Swal.fire({
        icon: "error",
        title: "Error",
        text: "Failed to update user status. Please try again.",
      });
    },
  });
}
function sanitizeHTML(input) {
  return input
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}
function addNewUserForm() {
  const emailPattern = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/;
  const email = $("#add-user-email").val();
  if (!emailPattern.test(email)) {
    Swal.fire("Error", "Invalid Email", "error");
  }
  const validationError = validatePassword($("#add-user-password").val());
  if (validationError) {
    Swal.fire({
      icon: "error",
      title: "Error",
      text: validationError,
    });
    return;
  }

  var formData = {
    userFullname: sanitizeHTML($("#add-user-fullname").val()),
    // username: sanitizeHTML($("#add-user-username").val()),
    userEmail: sanitizeHTML($("#add-user-email").val()),
    userPassword: sanitizeHTML($("#add-user-password").val()),
    userPhone: sanitizeHTML($("#add-user-phone").val()),
    companyName: sanitizeHTML($("#add-user-company").val()),
    role: sanitizeHTML($("#user-role-select").val()),
    userStatus: sanitizeHTML($("#user-status").val()),
  };

  // Display Swal alert with user details
  Swal.fire({
    title: "Confirm User Details",
    html:
      "<b>Fullname:</b> " +
      formData.userFullname +
      //   '<br><b>Username:</b> ' + formData.username +
      "<br><b>Email:</b> " +
      formData.userEmail +
      "<br><b>Password:</b> " +
      formData.userPassword +
      "<br><b>Phone:</b> " +
      formData.userPhone +
      "<br><b>Company Name:</b> " +
      formData.companyName +
      "<br><b>Role:</b> " +
      formData.role +
      "<br><b>Status:</b> " +
      formData.userStatus,
    icon: "info",
    showCancelButton: true,
    confirmButtonText: "Submit",
    cancelButtonText: "Cancel",
  }).then((result) => {
    if (result.isConfirmed) {
      // If user confirms, submit the form
      $.ajax({
        type: "POST",
        url: webPath + "tab-roles/admins/create",
        contentType: "application/json; charset=utf-8",
        dataType: "json",
        data: JSON.stringify(formData),
        success: function (response) {
          console.log("Data sent successfully:", response);
          $(".datatables-users-roles").DataTable().ajax.reload();
          $(".datatables-usersIRF").DataTable().ajax.reload();
          getUserInfoValue();
          location.reload();
        },
        error: function (error) {
          console.error("Error sending data:", error);
          Swal.fire("Error", error.responseJSON.error, "error");
        },
      });
    }
  });
}

function getUserInfoValue() {
  $.ajax({
    type: "GET",
    url: webPath + "/tab-dashboard/total-admin",
    contentType: "application/json; charset=utf-8",
    dataType: "json",
    // data: JSON.stringify(formData),
    success: function (response) {
      console.log("Data sent successfully:", response);
      $("#total-users").html(`${response.data.total}`);
      $("#total-active-users").html(`${response.data.active}`);
      $("#total-inactive-users").html(`${response.data.inactive}`);
      $("#total-pending-users").html(`${response.data.pending}`);
    },
    error: function (error) {
      console.log(error);
      console.error("Error sending data:", error);
    },
  });
}
function deleteUserAdmin(id) {
  Swal.fire({
    title: "Are you sure?",
    text: "You won't be able to revert this!",
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, delete it!",
  }).then((result) => {
    if (result.isConfirmed) {
      // User confirmed, proceed with the delete action
      $.ajax({
        type: "DELETE",
        url: webPath + "admin?id=" + id,
        contentType: "application/json; charset=utf-8",
        dataType: "json",
        success: function (response) {
          // Show success message
          Swal.fire("Deleted!", "User has been deleted.", "success");
          // Reload DataTable
          $(".datatables-usersIRF").DataTable().ajax.reload();
          // location.reload();
          getUserInfoValue();
        },
        error: function (error) {
          // Show error message
          Swal.fire(
            "Error",
            "User not deleted. Something went wrong.",
            "error"
          );
          console.error("Error sending data:", error);
        },
      });
    }
  });
}

//Function Roles
function getRoleList() {
  var role_list = $("#role_list");
  if (role_list.length) {
    $.ajax({
      type: "GET",
      url: webPath + "tab-roles/roles/gui",
      success: function (response) {
        if (response.data) {
          role_list.html(response.data);
          var roleNames = $(".role-name")
            .map(function () {
              //   console.log($(this).html())
              return $(this).html();
            })
            .get();

          // Clear existing options
          $("#roleEx3user").empty();
          $("#roleEx3user").append(
            $("<option>", {
              value: "",
              text: "Select Role",
            })
          );
          var user_role_select = $("#user-role-select");
          if (user_role_select.length) {
            // Clear existing options
            user_role_select.empty();
            roleNames.forEach(function (roleName) {
              user_role_select.append(
                `<option value="${roleName}">${roleName}</option>`
              );
            });
            // Trigger select2 to update
            user_role_select.trigger("change");
          }
          roleNames.forEach(function (roleName) {
            $("#roleEx3user").append(
              $("<option>", {
                value: roleName,
                text: roleName,
              })
            );
          });
        }
      },
      error: function (error) {
        if (error.status == 404) {
          alert("User Invalid");
        } else if (error.status == 400) {
          alert("Please Fill The Form");
        } else if (error.status == 401) {
          alert("Unauthorized");
        }
      },
    });
  }
}
function getRoleListApproval() {
  $.ajax({
    type: "GET",
    url: webPath + "tab-roles/roles/list",
    success: function (response) {
      if (response.data) {
        response.data.forEach(function (roleName) {
          $("#add-approval-role").append(
            $("<option>", {
              value: roleName,
              text: roleName,
            })
          );
        });
      }
    },
    error: function (error) {
      if (error.status == 404) {
        alert("User Invalid");
      } else if (error.status == 400) {
        alert("Please Fill The Form");
      } else if (error.status == 401) {
        alert("Unauthorized");
      }
    },
  });
}
function editRole(uuid, name, duplicate = false) {
  if (duplicate) {
    $(".role-title").html("Duplicate Role");
  } else {
    $(".role-title").html(uuid == 0 ? "Add Role" : "Edit Role");
  }
  $("#modalRoleName").val(name);
  var list_permission_loading = `<tr>
                        <td class="col-2 placeholder-glow" style="padding-left:0;padding-right:0;"> <span class="placeholder col-12 rounded"></span> </td>
                        <td class="col-3 placeholder-glow" style="padding-left:0;padding-right:0;"> </td>
                        <td class="col-12 row placeholder-glow" style="padding-left:0;padding-right:0;"> 
            <div class="col-1 placeholder-glow" style="padding-left:17px;padding-right:0px;"> <input class="form-check-input" type="checkbox" disabled checked /> </div>
            <div class="col-2 placeholder-glow" style="padding-left:0;padding-right:0;"> <span class="placeholder col-12 rounded"></span> </div>
            <div class="col-1 placeholder-glow" style="padding-left:17px;padding-right:0px;"> <input class="form-check-input" type="checkbox" disabled checked /> </div>
            <div class="col-2 placeholder-glow" style="padding-left:0;padding-right:0;"> <span class="placeholder col-12 rounded"></span> </div>
            <div class="col-1 placeholder-glow" style="padding-left:17px;padding-right:0px;"> <input class="form-check-input" type="checkbox" disabled checked /> </div>
            <div class="col-2 placeholder-glow" style="padding-left:0;padding-right:0;"> <span class="placeholder col-12 rounded"></span> </div>
            <div class="col-1 placeholder-glow" style="padding-left:17px;padding-right:0px;"> <input class="form-check-input" type="checkbox" disabled checked /> </div>
            <div class="col-2 placeholder-glow" style="padding-left:0;padding-right:0;"> <span class="placeholder col-12 rounded"></span> </div>
          </td>
                    </tr>`;
  for (let i = 0; i < 3; i++) {
    list_permission_loading += list_permission_loading;
  }
  $("#submit-role-user").prop("disabled", true);
  $("#list_permission").html(list_permission_loading);
  $.ajax({
    type: "GET",
    url: webPath + "tab-roles/roles/modal?data=" + uuid,
    success: function (response) {
      if (response.data) {
        $("#submit-role-user").prop("disabled", false);
        $("#list_permission").html(response.data);
        if (duplicate) {
          $('input[name="role_id"]').val("0");
        }
      }
    },
    error: function (error) {
      if (error.status == 404) {
        alert("User Invalid");
      } else if (error.status == 400) {
        alert("Please Fill The Form");
      } else if (error.status == 401) {
        alert("Unauthorized");
      }
    },
  });
}
function deleteRole(roleID, roleName) {
  Swal.fire({
    title: `Are you sure to delete ${roleName} Role?`,
    text: "Once deleted, you will not be able to recover this role!",
    icon: "warning",
    showCancelButton: true,
    confirmButtonText: "Yes, delete it!",
    customClass: {
      popup: "border border-danger border-5", // Add your custom classes here
      confirmButton: "btn btn-danger", // Add button-specific classes here
      cancelButton: "btn btn-primary", // Add button-specific classes here
    },
    buttonsStyling: false, // Disable default styling of buttons
  }).then((result) => {
    if (result.isConfirmed) {
      // Perform the deletion or send data to the server using jQuery
      $.ajax({
        url:
          webPath + "tab-roles/roles?data=" + roleID + "&rolename=" + roleName, // Replace with your server endpoint
        method: "DELETE",
        data: { roleID: roleID, roleName: roleName },
        success: function (response) {
          // Handle the success response
          Swal.fire("Role deleted successfully!", "", "success");
          getRoleList();
        },
        error: function (error) {
          // Handle the error response
          Swal.fire("Error deleting role!", error.responseJSON.error, "error");
        },
      });
    }
  });
}
function editUserRole(id, rolename, full_name) {
  $("#roleEx3user").val(rolename);
  $("#change-role-username").html(full_name);
  $("#userIdChangedName").val(id);
}
function updateUserRole() {
  var rolename = $("#roleEx3user").val();
  var username = $("#change-role-username").html();
  var userId = $("#userIdChangedName").val();
  Swal.fire({
    title: `Are you sure ?`,
    text: `Do you really want to change '${username}' to '${rolename}'?`,
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, change it!",
  }).then((result) => {
    if (result.isConfirmed) {
      // If the user clicks "Yes, submit it!", proceed with the form submission
      var formData = new FormData();
      formData.append("id", userId);
      formData.append("username", username);
      formData.append("rolename", rolename);

      $.ajax({
        type: "PATCH",
        url: webPath + "tab-roles/admin/roles",
        data: formData,
        processData: false, // Prevent jQuery from processing the data
        contentType: false, // Prevent jQuery from setting content type
        success: function (response) {
          // Handle success response
          getRoleList();
          $(".datatables-users-roles").DataTable().ajax.reload();
          $("#editUserRoleModal").modal("hide");
          Swal.fire("Success", "Form submitted successfully!", "success");
        },
        error: function (error) {
          // Handle error response
          console.error(error);
          Swal.fire(
            "Error",
            "An error occurred while submitting the form.",
            "error"
          );
        },
      });
    }
  });
}

//Function Transaction
function openImageModal(imagePath) {
  document.getElementById("imageModalImage").src = "../" + imagePath;
  $("#imageModal").modal("show");
}
function handleViewimages(srcDispenser, srcVehicle, srcPlate) {
  $("#imageDispenser").attr("src", srcDispenser);
  $("#imageVehicles").attr("src", srcVehicle);
  $("#imagePlateNumber").attr("src", srcPlate);
}

//Function Operator
function addNewOperatorForm() {
  var formData = {
    username: $("#add-operator-username").val(),
    userEmail: $("#add-operator-email").val(),
    userPassword: $("#add-operator-password").val(),
    userStatus: $("#add-operator-status").val(),
  };
  $.ajax({
    type: "POST",
    url: webPath + "admin/operator",
    contentType: "application/json; charset=utf-8",
    dataType: "json",
    data: JSON.stringify(formData),
    success: function (response) {
      console.log("Data sent successfully:", response);
      // location.reload();
      $(".datatables-operators").DataTable().ajax.reload();
      $("#addOperator").modal("hide");
      console.log("12333");
      // getUserwarningbg-label-warningValue ();
    },
    error: function (error) {
      console.log(error);
      console.error("Error sending data:", error);
    },
  });
}
function deleteUserOperator(id) {
  Swal.fire({
    title: "Are you sure?",
    text: "You won't be able to revert this!",
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, delete it!",
  }).then((result) => {
    if (result.isConfirmed) {
      // User confirmed, proceed with the delete action
      $.ajax({
        type: "DELETE",
        url: webPath + "admin/operator?id=" + id,
        contentType: "application/json; charset=utf-8",
        dataType: "json",
        success: function (response) {
          Swal.fire("Deleted!", "User has been deleted.", "success");
          $(".datatables-operators").DataTable().ajax.reload();
          // location.reload();
          getUserInfoValue();
        },
        error: function (error) {
          // Show error message
          Swal.fire(
            "Error",
            "User not deleted. Something went wrong.",
            "error"
          );
          console.error("Error sending data:", error);
        },
      });
    }
  });
}

//Function Merchant
function addNewMerchantForm() {
  var formData = {
    merchant_name: $("#add-merchant-name").val(),
    merchant_addr1: $("#add-merchant-address1").val(),
    merchant_addr2: $("#add-merchant-address2").val(),
    merchant_addr3: $("#add-merchant-address3").val(),
  };
  $.ajax({
    type: "POST",
    url: webPath + "admin/merchant",
    contentType: "application/json; charset=utf-8",
    dataType: "json",
    data: JSON.stringify(formData),
    success: function (response) {
      console.log("Data sent successfully:", response);
      // location.reload();
      $(".datatables-merchants").DataTable().ajax.reload();
      $("#addMerchant").modal("hide");
      // getUserwarningbg-label-warningValue ();
    },
    error: function (error) {
      console.log(error);
      console.error("Error sending data:", error);
    },
  });
}
function deleteUserMerchant(id) {
  Swal.fire({
    title: "Are you sure?",
    text: "You won't be able to revert this!",
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, delete it!",
  }).then((result) => {
    if (result.isConfirmed) {
      // User confirmed, proceed with the delete action
      $.ajax({
        type: "DELETE",
        url: webPath + "admin/merchant?id=" + id,
        contentType: "application/json; charset=utf-8",
        dataType: "json",
        success: function (response) {
          Swal.fire("Deleted!", "User has been deleted.", "success");
          $(".datatables-merchants").DataTable().ajax.reload();
          // location.reload();
          getUserInfoValue();
        },
        error: function (error) {
          // Show error message
          Swal.fire(
            "Error",
            "User not deleted. Something went wrong.",
            "error"
          );
          console.error("Error sending data:", error);
        },
      });
    }
  });
}

//Function Product
function addNewProductForm() {
  var formData = {
    product_name: $("#add-product-name").val(),
    product_type: $("#add-product-type").val(),
    product_price: $("#add-product-price").val(),
  };
  $.ajax({
    type: "POST",
    url: webPath + "admin/product",
    contentType: "application/json; charset=utf-8",
    dataType: "json",
    data: JSON.stringify(formData),
    success: function (response) {
      console.log("Data sent successfully:", response);
      // location.reload();
      $(".datatables-products").DataTable().ajax.reload();
      $("#addProduct").modal("hide");
      console.log("12333");
      // getUserwarningbg-label-warningValue ();
    },
    error: function (error) {
      console.log(error);
      console.error("Error sending data:", error);
    },
  });
}
function deleteUserProduct(id) {
  Swal.fire({
    title: "Are you sure?",
    text: "You won't be able to revert this!",
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#3085d6",
    cancelButtonColor: "#d33",
    confirmButtonText: "Yes, delete it!",
  }).then((result) => {
    if (result.isConfirmed) {
      // User confirmed, proceed with the delete action
      $.ajax({
        type: "DELETE",
        url: webPath + "admin/product?id=" + id,
        contentType: "application/json; charset=utf-8",
        dataType: "json",
        success: function (response) {
          Swal.fire("Deleted!", "Product has been deleted.", "success");
          $(".datatables-products").DataTable().ajax.reload();
          // location.reload();
          getUserInfoValue();
        },
        error: function (error) {
          // Show error message
          Swal.fire(
            "Error",
            "Product not deleted. Something went wrong.",
            "error"
          );
          console.error("Error sending data:", error);
        },
      });
    }
  });
}

function generateSkeletonLoadingHTML(rows, cols) {
  let skeletonHTML = '<tbody class="skeleton-loading">';
  for (let i = 0; i < rows; i++) {
    skeletonHTML += "<tr>";
    for (let j = 0; j < cols; j++) {
      skeletonHTML += `
                <td><p class="placeholder-glow"><span class="placeholder col-12 rounded"></span></p></td>
            `;
    }
    skeletonHTML += "</tr>";
  }
  skeletonHTML += "</tbody>";
  return skeletonHTML;
}

function filterRole(role) {
  var userRoleSelect = document.getElementById("UserRole");
  if (userRoleSelect.length) {
    userRoleSelect.value = role; // This will select the "Operator" option
    // Manually trigger the change event
    var event = new Event("change");
    userRoleSelect.dispatchEvent(event);
  }
}

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
                    <input type="text" id="username" class="swal2-input" placeholder="Username">
                    <input type="password" id="password" class="swal2-input" placeholder="Password">
                    `;
        }
        Swal.fire({
          title: "Do you want to save the changes?",
          html: html,
          showCancelButton: true,
          confirmButtonText: "Save",
          cancelButtonText: "Cancel",
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
                  title: "Success",
                  text: data.msg,
                  timer: 3000, // Timer set to 3 seconds
                  timerProgressBar: true, // Display timer progress bar
                });
              })
              .catch((error) => {
                console.error("Error:", error);
                Swal.fire({
                  icon: "error",
                  title: "Error",
                  text: error.message || "An unexpected error occurred",
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
            title: "Do you want to save the changes?",
            showCancelButton: true,
            confirmButtonText: "Save",
            cancelButtonText: "Cancel",
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
                    title: "Success",
                    text: "The data was updated successfully!",
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
                    title: "Error",
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
                          `<option value="${choice}" ${
                            element.getAttribute("origin") == choice
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
          title: "Do you want to save the changes?",
          showCancelButton: true,
          confirmButtonText: "Save",
          cancelButtonText: "Cancel",
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
                  title: "Success",
                  text: "The data was updated successfully!",
                  timer: 3000, // Timer set to 3 seconds
                  timerProgressBar: true, // Display timer progress bar
                });
                $(`#${tab}`).DataTable().ajax.reload();
              })
              .catch((error) => {
                console.error("Error:", error);
                Swal.fire({
                  icon: "error",
                  title: "Error",
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
        title: `Are you sure to delete THIS?`,
        html: html,
        text: "Once deleted, you will not be able to recover this data!",
        position: "top",
        showCancelButton: true,
        confirmButtonText: "Yes, delete it! (5)",
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
            confirmButton.textContent = `Yes, delete it! (${timer})`;

            if (timer <= 0) {
              clearInterval(timerInterval);
              confirmButton.textContent = `Yes, delete it!`;
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
                  title: "Error",
                  text: data.error,
                });
                $(`#${tab}`).DataTable().ajax.reload();
              } else {
                Swal.fire({
                  icon: "success",
                  title: "Success",
                  text: "The data was DELETED successfully!",
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
                title: "Error",
                text: error,
              });
            });
        }
        parentColumn.removeClass("glowing-red-background");
      });
    });
  });
}
// Function to add a new form at the bottom of the table
function addNewForm(tableId, endpoint, ...columnFields) {
  // const formHtml = `
  //   <form id="${tableId}_create" class="ms-4">
  //     <div class="row">
  //       ${columnFields.map(field => `
  //         <div class="col">
  //           <input type="text" class="form-control mb-2" id="new_${field}" placeholder="${field}" required>
  //         </div>
  //       `).join('')}
  //       <div class="col">
  //         <button type="submit" class="btn btn-sm btn-primary"><i class='bx bx-save'></i></button>
  //         <button type="button" id="cancelNewRow" class="btn btn-sm btn-secondary"><i class='bx bx-x'></i></button>
  //       </div>
  //     </div>
  //   </form>
  // `;
  const formHtml = `
        <form id="${tableId}_create" class="ms-4">
            <div class="row">
                ${columnFields
                  .map((field) => {
                    if (Array.isArray(field)) {
                      const [fieldName, choices] = field;
                      return `
                            <div class="col">
                                <select class="form-control mb-2" id="new_${fieldName}" required>
                                    ${choices
                                      .map(
                                        (choice) =>
                                          `<option value="${choice}">${choice}</option>`
                                      )
                                      .join("")}
                                </select>
                            </div>
                        `;
                    } else {
                      return `
                            <div class="col">
                                <input type="text" class="form-control mb-2" id="new_${field}" placeholder="${field}" required>
                            </div>
                        `;
                    }
                  })
                  .join("")}
                <div class="col">
                    <button type="submit" class="btn btn-sm btn-primary"><i class='bx bx-save'></i></button>
                    <button type="button" id="cancelNewRow" class="btn btn-sm btn-secondary"><i class='bx bx-x'></i></button>
                </div>
            </div>
        </form>
    `;

  if ($(`#${tableId}_create`).length === 0) {
    // Append the form to the bottom of the table
    $(`#${tableId}`).before(formHtml);

    // Handle form submission
    document
      .getElementById(`${tableId}_create`)
      .addEventListener("submit", function (event) {
        event.preventDefault();

        const formData = {};
        columnFields.forEach((field) => {
          if (Array.isArray(field)) {
            const [fieldName, choices] = field;
            field = fieldName;
          }
          formData[field] = document.getElementById(`new_${field}`).value;
        });

        Swal.fire({
          title: "Do you want to Create New Data?",
          showCancelButton: true,
          confirmButtonText: "Save",
          cancelButtonText: "Cancel",
        }).then((result) => {
          if (result.isConfirmed) {
            fetch(endpoint, {
              method: "POST",
              headers: {
                "Content-Type": "application/json",
              },
              body: JSON.stringify(formData),
            })
              .then((response) => {
                response.json();
              })
              .then((data) => {
                // console.log('Success:');

                // Reload the table data to reflect the new row
                $(`#${tableId}`).DataTable().ajax.reload();

                // Remove the form after submission
                document.getElementById(`${tableId}_create`).remove();
                Swal.fire({
                  icon: "success",
                  title: "Success",
                  text: "The data was created successfully!",
                  timer: 3000, // Timer set to 3 seconds
                  timerProgressBar: true, // Display timer progress bar
                });
              })
              .catch((error) => {
                console.error("Error:", error);
                Swal.fire({
                  icon: "error",
                  title: "Error",
                  text: error,
                });
              });
          }
        });
      });

    // Handle form cancellation
    document
      .getElementById("cancelNewRow")
      .addEventListener("click", function () {
        document.getElementById(`${tableId}_create`).remove();
      });
  } else {
    document.getElementById(`${tableId}_create`).remove();
  }
}

function validatePassword(password) {
  if (password.length < 12) {
    return "Password must be at least 12 characters long";
  }
  if (!/[A-Z]/.test(password)) {
    return "Password must contain at least one uppercase letter";
  }
  if (!/[a-z]/.test(password)) {
    return "Password must contain at least one lowercase letter";
  }
  if (!/[0-9]/.test(password)) {
    return "Password must contain at least one number";
  }
  if (!/[~!@#$%^&*()_+\-={}|:"<>?]/.test(password)) {
    return 'Password must contain at least one special character (~!@#$%^&*()_+`{}|:"<>?)';
  }
  return null;
}
function isNumberAndConvert(str) {
  // Check if the string contains only digits
  if (/^\d+$/.test(str)) {
    // Convert the string to a number
    return Number(str);
  }
  return null; // Return null or handle the case where the string is not a number
}
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

function updateEditableModalValue(element) {
  // Loop through all attributes of the button
  Array.from(element.attributes).forEach((attr) => {
    // Check if the attribute name starts with 'dt_' or 'ed_'
    if (attr.name.startsWith("dt_") || attr.name.startsWith("ed_")) {
      // console.log(`Attribute: ${attr.name}, Value: ${attr.value}`);
      $(`#${attr.name}`).val(`${attr.value}`);
    }
  });
}
