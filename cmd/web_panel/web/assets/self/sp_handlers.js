// Delete All SP Records Function
function confirmDeleteAllSP(type, button) {
  const count = $(button).data("count");
  const url = $(button).data("url");

  if (!url || count === 0) {
    showIziToast("Error", "No records to delete or invalid URL", "error");
    return;
  }

  Swal.fire({
    title: `Delete All SP ${type}?`,
    html: `Are you sure you want to delete <strong>ALL ${count}</strong> SP records for <strong>${type}</strong>?<br><br>
           <span class="text-danger"><i class="fas fa-exclamation-triangle"></i> This action cannot be undone!</span>`,
    icon: "warning",
    showCancelButton: true,
    confirmButtonColor: "#d33",
    cancelButtonColor: "#3085d6",
    confirmButtonText: `Yes, Delete All ${count} Records!`,
    cancelButtonText: "Cancel",
    showLoaderOnConfirm: true,
    preConfirm: () => {
      return fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
      })
        .then((response) => {
          if (!response.ok) {
            throw new Error(response.statusText);
          }
          return response.json();
        })
        .catch((error) => {
          Swal.showValidationMessage(`Request failed: ${error}`);
        });
    },
    allowOutsideClick: () => !Swal.isLoading(),
  }).then((result) => {
    if (result.isConfirmed) {
      if (result.value && result.value.success) {
        Swal.fire({
          title: "Deleted!",
          html: `All SP ${type} records have been deleted successfully.<br>Deleted: <strong>${
            result.value.deleted_count || count
          }</strong> records`,
          icon: "success",
          confirmButtonColor: "#3085d6",
        }).then(() => {
          // Reload the page or refresh the table
          location.reload();
        });
      } else {
        Swal.fire(
          "Error!",
          result.value?.message || "Failed to delete SP records",
          "error"
        );
      }
    }
  });
}
