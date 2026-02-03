function showIziToast(status, message) {
    status = status.toLowerCase();

    let divID = "show-izi-toast";
    let type = "";
    let icon = "";
    let progressBarColor = "";

    if (status) {
        switch (status) {
            case 'error':
                type             = 'error';
                icon             = 'fa fa-window-close';
                progressBarColor = '#FF0000';
                break;
            case 'success':
                type             = 'success';
                icon             = 'fa fa-check';
                progressBarColor = '#00FF00';
                break;
            case 'warning':
                type             = 'warning';
                icon             = 'fa fa-exclamation-triangle';
                progressBarColor = '#FFFF00';
                break;
            case 'info':
                type             = 'info';
                icon             = 'fa fa-info-circle';
                progressBarColor = '#0000FF';
                break;
        }
    }

    iziToast.show({
        title: status.toUpperCase(),
        message: message,
        position: 'topRight',
        // displayMode: 0,
        displayMode: 2,
        timeout: 15000,
        transitionIn: 'bounceInLeft',
        transitionOut: 'fadeOutDown',
        type: type,
        icon: icon,
        progressBarColor: progressBarColor,
        resetOnHover: true
    });
}