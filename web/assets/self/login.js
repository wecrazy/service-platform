/**
 * Login Page JS
 */

$(document).ready(function() {

    // Inject dynamic styles via JavaScript to control sliding and status visuals
    (function injectLoginStyles(){
        var css = `
        .glowing-border { border: 2px solid transparent; border-radius: 3px; transition: border-color .5s ease; }
        @keyframes glow { 0% { border-color: transparent; box-shadow: none } 50% { border-color: paleturquoise; box-shadow: 0 0 10px paleturquoise } 100% { border-color: transparent; box-shadow: none } }
        .glowing-border:not(:hover):not(:focus) { animation: glow 2.5s infinite alternate; }
        .glow-border { border: 2px solid transparent; border-radius: 3px; border-color: skyblue; box-shadow: 0 0 10px skyblue }
        #submitContainer { position: relative }
        .login-slide-btn { width: 100% !important; position: relative; left: auto; transform: none; transition: opacity .18s linear, width .18s linear }
        .captcha-status { position: absolute; right: 12px; top: 50%; transform: translateY(-50%); font-size: 1.05rem; line-height: 1; pointer-events: none; display: inline-block; min-width: 1.5em; text-align: right }
        .captcha-status.text-success { color: #198754 }
        .captcha-status.text-danger { color: #dc3545 }
        `;
        var style = document.createElement('style');
        style.id = 'login-js-styles';
        style.appendChild(document.createTextNode(css));
        document.head.appendChild(style);
    })();

    // Define t function for i18n
    window.t = function(key) {
        return (typeof i18next !== 'undefined' && i18next.t) ? i18next.t(key) : key;
    };

    // initialize button disabled state
    $('#signInBtn').attr('data-disabled', 'true');
    // remove Bootstrap "disabled" class to avoid pointer-events blocking
    $('#signInBtn').removeClass('disabled');

    // setup container and button references
    var $submitContainer = $('#submitContainer');
    var $btn = $('#signInBtn');

    // apply inline base styles: make button static (no sliding)
    function applyButtonInlineStyles() {
        $submitContainer.css({ position: 'relative' });
        $btn.css({ position: 'relative', top: '0', left: 'auto', width: '100%', boxSizing: 'border-box', transition: 'opacity .18s linear, width .18s linear' });
        // start visually disabled
        $btn.css({ opacity: .65, 'pointer-events': 'none' });
    }
    applyButtonInlineStyles();

    // reposition according to current logical state — static behavior: only change width/opacity/pointer-events
    function repositionAccordingState() {
        var disabled = $btn.attr('data-disabled');
        if (disabled === 'true') {
            $btn.css({ width: '100%', opacity: .65, 'pointer-events': 'none', cursor: 'default' });
        } else {
            $btn.css({ width: '100%', opacity: 1, 'pointer-events': 'auto', cursor: 'pointer' });
        }
    }

    // initial positioning
    repositionAccordingState();

    // Observe attribute/style changes on the button and enforce our desired state
    try {
        var mo = new MutationObserver(function(mutations){
            // if someone else changed attributes/styles, reapply our state
            mutations.forEach(function(m){ /* noop */ });
            repositionAccordingState();
        });
        mo.observe($btn[0], { attributes: true, attributeFilter: ['style', 'class', 'data-disabled'] });
    } catch (err) { /* ignore if MutationObserver not available */ }

    // Debug badge to help trace state during testing
    if (window.ACTIVE_DEBUG) {
        (function addDebugBadge(){
            var dbg = document.createElement('div');
            dbg.id = 'login-debug-badge';
            dbg.style.position = 'fixed';
            dbg.style.right = '12px';
            dbg.style.bottom = '12px';
            dbg.style.background = 'rgba(0,0,0,0.6)';
            dbg.style.color = 'white';
            dbg.style.padding = '6px 8px';
            dbg.style.fontSize = '12px';
            dbg.style.borderRadius = '6px';
            dbg.style.zIndex = 9999;
            dbg.style.whiteSpace = 'pre';
            dbg.style.pointerEvents = 'none';
            dbg.innerText = 'login.debug';
            document.body.appendChild(dbg);
            function updateBadge(){
                try {
                    var dd = $btn.attr('data-disabled');
                    var cv = ($('#client_captcha_valid').val() || '0');
                    var cs = $('#captcha-status').text().trim();
                    dbg.innerText = 'disabled=' + dd + ' | captcha_valid=' + cv + '\n' + cs;
                } catch(e) {}
            }
            setInterval(updateBadge, 300);
        })();
    }

    // Form validation and button enable/disable
    function validateForm() {
        const email = $('#email').val().trim();
        const password = $('#password').val().trim();
        const captchaInput = $('#captcha-input').val().trim();

        // Enable button only if all fields filled and CAPTCHA validated
        const captchaValid = ($('#client_captcha_valid').val() || '0') === '1';
        const allFilled = email && password && captchaInput && captchaValid;
        // Use data-disabled attribute instead of relying on Bootstrap's .disabled class,
        // so pointer-events are not blocked and we can control click behavior in JS.
        $('#signInBtn').attr('data-disabled', allFilled ? 'false' : 'true');
        $('#signInBtn').attr('aria-disabled', allFilled ? 'false' : 'true');
        if (allFilled) {
            $('#signInBtn').addClass('enabled');
        } else {
            $('#signInBtn').removeClass('enabled');
        }
        // ensure visual position matches new state
        repositionAccordingState();
    }

    // Helper to set captcha status text/icon
    function setCaptchaStatus(state, message) {
        // state: 'success' | 'error' | 'checking' | 'clear'
        const $status = $('#captcha-status');
        $status.removeClass('text-muted text-danger text-success');
        if (state === 'success') {
            $status.addClass('text-success').html('✅ ' + (message || window.t('login.captchaCorrect'))).show();
            $('#captcha-input').removeClass('is-invalid');
        } else if (state === 'error') {
            $status.addClass('text-danger').html('❌ ' + (message || window.t('login.captchaIncorrect'))).show();
            $('#captcha-input').addClass('is-invalid');
        } else if (state === 'checking') {
            $status.addClass('text-muted').html('… ' + (message || window.t('common.loading'))).show();
            $('#captcha-input').removeClass('is-invalid');
        } else {
            $status.hide().html('');
            $('#captcha-input').removeClass('is-invalid');
        }
    }

    // Build absolute URL from GLOBAL_URL and a relative path
    function urlFor(relPath) {
        var base = (typeof window.GLOBAL_URL !== 'undefined' && window.GLOBAL_URL) ? window.GLOBAL_URL : '/';
        if (base === '') base = '/';
        if (!base.endsWith('/')) base = base + '/';
        relPath = String(relPath).replace(/^\/+/, '');
        return base + relPath;
    }

    // Bind events
    $('#email, #password, #captcha-input').on('input', function() {
        setCaptchaStatus('clear');
        validateForm();

        // Real-time CAPTCHA validation (client-side): compare SHA-256 hash
        const captchaInput = $('#captcha-input').val().trim();
        if (captchaInput.length === 6) {
            setCaptchaStatus('checking');
            // compute hash and compare to hidden captcha_hash
            (function(userVal){
                // normalize (digits only expected)
                var v = String(userVal).trim();
                if (!v) { setCaptchaStatus('error'); return; }
                // compute hash
                if (window.crypto && window.crypto.subtle) {
                    const enc = new TextEncoder();
                    window.crypto.subtle.digest('SHA-256', enc.encode(v)).then(function(hash){
                        var bytes = new Uint8Array(hash);
                        var binary = '';
                        for (var i=0;i<bytes.byteLength;i++) binary += String.fromCharCode(bytes[i]);
                        var b64 = btoa(binary);
                        var expected = $('#captcha_hash').val();
                        if (b64 === expected) {
                            setCaptchaStatus('success');
                            $('#client_captcha_valid').val('1');
                            // re-evaluate form to enable button only when captcha is valid
                            validateForm();
                        } else {
                            setCaptchaStatus('error');
                            $('#client_captcha_valid').val('0');
                            validateForm();
                        }
                    }).catch(function(e){ setCaptchaStatus('error'); });
                } else {
                    // fallback compare plaintext (unsafe)
                    var expectedPlain = $('#captcha_hash').val();
                    if (expectedPlain && expectedPlain === btoa(unescape(encodeURIComponent(v)))) {
                        setCaptchaStatus('success'); $('#client_captcha_valid').val('1'); validateForm();
                    } else { setCaptchaStatus('error'); $('#client_captcha_valid').val('0'); }
                }
            })(captchaInput);
        }
    });
    // Form submission
    $('#formLoginAuthentication').on('submit', function(e) {
        e.preventDefault();
        // prevent submission when sign-in is logically disabled
        if ($('#signInBtn').attr('data-disabled') === 'true') {
            return;
        }

        const email = $('#email').val().trim();
        const password = $('#password').val().trim();
        const captchaInput = $('#captcha-input').val().trim();

        if (!email || !password || !captchaInput) {
            Swal.fire({
                icon: 'warning',
                title: t('login.pleaseFillAllFields'),
                confirmButtonText: t('common.ok')
            });
            return;
        }

        // Show loading and mark disabled
        $('#signInBtn').attr('data-disabled', 'true').attr('aria-disabled', 'true').html('<span class="spinner-border spinner-border-sm ms-2" role="status" aria-hidden="true"></span> ' + t('common.loading'));

        // Send login request
        $.ajax({
            url: urlFor('login'),
            method: 'POST',
            data: {
                    'email-username': email,
                    password: password,
                    captcha: captchaInput,
                    client_captcha_valid: $('#client_captcha_valid').val() || '0'
                },
            success: function(response, textStatus, xhr) {
                // If server returned JSON/object (errors), handle it; otherwise assume redirect/HTML => success
                if (typeof response === 'object') {
                    if (response.success) {
                        Swal.fire({
                            icon: 'success',
                            title: t('login.loginSuccessful'),
                            confirmButtonText: t('common.ok')
                        }).then(() => {
                            window.location.href = urlFor('page');
                        });
                    } else {
                        if (response && response.error && String(response.error).toLowerCase().includes('captcha')) {
                            setCaptchaStatus('error', response.error);
                        } else {
                            setCaptchaStatus('error');
                        }
                        Swal.fire({
                            icon: 'error',
                            title: t('login.loginFailed'),
                            text: response.message || t('login.invalidCredentials'),
                            confirmButtonText: t('common.ok')
                        });
                        refreshCaptcha(); // Refresh CAPTCHA
                    }
                } else {
                    // Received HTML (redirect followed) - consider this a success and navigate to the page
                    window.location.href = urlFor('page');
                }
            },
            error: function(xhr) {
                const errorMsg = xhr.responseJSON ? (xhr.responseJSON.error || xhr.responseJSON.message) : t('common.networkError');
                if (errorMsg && String(errorMsg).toLowerCase().includes('invalid') && String(errorMsg).toLowerCase().includes('captcha')) {
                    setCaptchaStatus('error', t('login.captchaIncorrect'));
                } else {
                    setCaptchaStatus('error', errorMsg || null);
                }
                Swal.fire({
                    icon: 'error',
                    title: t('login.loginFailed'),
                    text: errorMsg,
                    confirmButtonText: t('common.ok')
                });
                refreshCaptcha(); // Refresh CAPTCHA
            },
            complete: function() {
                // restore button label, but let validateForm decide enabled state
                $('#signInBtn').html(t('login.signIn'));
                validateForm(); // Re-validate and set data-disabled appropriately
            }
        });
    });

    // Password toggle
    $('.form-password-toggle .input-group-text').on('click', function() {
        const input = $(this).siblings('input');
        const type = input.attr('type') === 'password' ? 'text' : 'password';
        input.attr('type', type);
        $(this).find('i').toggleClass('fa-eye-slash fa-eye');
    });

    // initial validation to set correct enabled/disabled state
    validateForm();

});